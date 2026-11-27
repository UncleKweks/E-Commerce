package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/models"
	"gorm.io/gorm"
)

// MaxQuantityPerItem caps how many of one product fit in a cart, even when
// there's more stock. It stops one customer emptying the warehouse by
// mistake (or on purpose) and keeps price x quantity far from overflowing.
const MaxQuantityPerItem = 99

// Errors the cart service can return.
var (
	ErrProductUnavailable = errors.New("product is not available")
	ErrCartItemNotFound   = errors.New("product is not in the cart")
)

// NotEnoughStockError means the requested quantity is more than can be
// bought. Unlike the errors above it carries data, so the handler can tell
// the customer exactly how many they can have. Check for it with errors.As.
type NotEnoughStockError struct {
	Max    int // the most this cart line may hold
	InCart int // how many are already in the cart
}

func (e *NotEnoughStockError) Error() string {
	return fmt.Sprintf("not enough stock: max %d, %d already in cart", e.Max, e.InCart)
}

// CartService manages each user's shopping cart.
//
// Notice that no method takes a cart ID. The cart is always found from the
// user ID, which comes from the verified access token, so there is no way to
// point a request at someone else's cart.
type CartService struct {
	db *gorm.DB
}

// NewCartService returns a CartService.
func NewCartService(db *gorm.DB) *CartService {
	return &CartService{db: db}
}

// GetCart returns the user's cart, checked against today's prices and stock.
// It never writes anything: a GET must be safe to repeat, and a user who has
// never added anything simply has an empty cart, with no row needed.
func (s *CartService) GetCart(ctx context.Context, userID uint) (*dto.CartResponse, error) {
	return s.loadCart(s.db.WithContext(ctx), userID)
}

// AddItem puts quantity units of a product in the cart, on top of any
// already there.
func (s *CartService) AddItem(ctx context.Context, userID, productID uint, quantity int) (*dto.CartResponse, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		product, err := findPurchasable(tx, productID)
		if err != nil {
			return err
		}
		cartID, err := ensureCart(tx, userID)
		if err != nil {
			return err
		}

		// UPSERT: insert the line, or if this product is already in the cart,
		// add to its quantity. One statement, so a double-click can't create
		// two lines, and the addition happens inside the database, so two
		// requests at once can't overwrite each other's change.
		now := time.Now()
		var newQty int
		err = tx.Raw(`
			INSERT INTO cart_items (cart_id, product_id, quantity, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (cart_id, product_id)
			DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity,
			              updated_at = EXCLUDED.updated_at
			RETURNING quantity`,
			cartID, productID, quantity, now, now).Scan(&newQty).Error
		if err != nil {
			return fmt.Errorf("add cart item: %w", err)
		}

		// Checked after the upsert, inside the transaction: if it's too many,
		// returning an error rolls the upsert back as if it never happened.
		if limit := purchaseLimit(product); newQty > limit {
			return &NotEnoughStockError{Max: limit, InCart: newQty - quantity}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetCart(ctx, userID)
}

// SetQuantity changes a cart line to exactly quantity units.
func (s *CartService) SetQuantity(ctx context.Context, userID, productID uint, quantity int) (*dto.CartResponse, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.CartItem
		err := inUsersCart(tx, userID).Where("product_id = ?", productID).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCartItemNotFound
		}
		if err != nil {
			return fmt.Errorf("find cart item: %w", err)
		}

		product, err := findPurchasable(tx, productID)
		if err != nil {
			return err // only removing is allowed once a product is unavailable
		}
		if limit := purchaseLimit(product); quantity > limit {
			return &NotEnoughStockError{Max: limit, InCart: item.Quantity}
		}

		err = tx.Model(&item).Updates(map[string]any{"quantity": quantity, "updated_at": time.Now()}).Error
		if err != nil {
			return fmt.Errorf("update cart item: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetCart(ctx, userID)
}

// RemoveItem takes a product out of the cart. Removing something that isn't
// there isn't an error: the cart ends up the same either way.
func (s *CartService) RemoveItem(ctx context.Context, userID, productID uint) (*dto.CartResponse, error) {
	db := s.db.WithContext(ctx)
	if err := inUsersCart(db, userID).Where("product_id = ?", productID).Delete(&models.CartItem{}).Error; err != nil {
		return nil, fmt.Errorf("remove cart item: %w", err)
	}
	return s.loadCart(db, userID)
}

// Clear empties the cart.
func (s *CartService) Clear(ctx context.Context, userID uint) (*dto.CartResponse, error) {
	db := s.db.WithContext(ctx)
	if err := inUsersCart(db, userID).Delete(&models.CartItem{}).Error; err != nil {
		return nil, fmt.Errorf("clear cart: %w", err)
	}
	return s.loadCart(db, userID)
}

// ---- helpers ----

func (s *CartService) loadCart(db *gorm.DB, userID uint) (*dto.CartResponse, error) {
	var items []models.CartItem
	err := inUsersCart(db, userID).
		// Unscoped: include products that were soft-deleted after being added,
		// so the cart can say "no longer available" instead of the line
		// silently vanishing.
		Preload("Product", func(q *gorm.DB) *gorm.DB { return q.Unscoped() }).
		Preload("Product.Category", func(q *gorm.DB) *gorm.DB { return q.Unscoped() }).
		Order("created_at, id").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("load cart: %w", err)
	}
	return buildCart(items), nil
}

// buildCart turns cart rows into the response, checking each line against
// the product's current state. It's a pure function (no database), which
// makes it easy to unit test.
func buildCart(items []models.CartItem) *dto.CartResponse {
	cart := &dto.CartResponse{Items: make([]dto.CartItemResponse, 0, len(items)), CanCheckout: len(items) > 0}

	for i := range items {
		item := &items[i]
		line := dto.CartItemResponse{ProductID: item.ProductID, Quantity: item.Quantity, Status: dto.CartItemOK}
		p := item.Product

		switch {
		case p == nil || !isPurchasable(p):
			line.Status = dto.CartItemUnavailable
			line.Message = "This product is no longer available. Please remove it."
		case p.Stock <= 0:
			line.Status = dto.CartItemOutOfStock
			line.Message = "Out of stock. Please remove it or check back later."
		case item.Quantity > p.Stock:
			line.Status = dto.CartItemNotEnoughStock
			line.Message = fmt.Sprintf("Only %d left. Please lower the quantity.", p.Stock)
		}

		if p != nil {
			line.Name = p.Name
			line.SKU = p.SKU
			line.UnitPriceCents = p.PriceCents
			line.LineTotalCents = p.PriceCents * int64(item.Quantity)
			if line.Status != dto.CartItemUnavailable {
				line.Available = purchaseLimit(p)
			}
		}

		cart.ItemCount += item.Quantity
		if line.Status == dto.CartItemOK {
			cart.SubtotalCents += line.LineTotalCents
		} else {
			cart.CanCheckout = false
		}
		cart.Items = append(cart.Items, line)
	}
	return cart
}

// isPurchasable mirrors visibleProducts for a product already loaded in memory.
func isPurchasable(p *models.Product) bool {
	return !p.DeletedAt.Valid && p.IsActive &&
		p.Category != nil && !p.Category.DeletedAt.Valid && p.Category.IsActive
}

// purchaseLimit is the most of this product one cart line may hold.
func purchaseLimit(p *models.Product) int {
	return max(0, min(p.Stock, MaxQuantityPerItem))
}

// findPurchasable loads a product only if customers can currently buy it.
func findPurchasable(tx *gorm.DB, productID uint) (*models.Product, error) {
	var p models.Product
	err := visibleProducts(tx.Model(&models.Product{})).First(&p, productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProductUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("find product: %w", err)
	}
	return &p, nil
}

// ensureCart returns the user's cart ID, creating the cart on first use.
// It's an upsert too: if two "add to cart" requests arrive at once for a
// user with no cart, both try to insert, user_id is UNIQUE, and ON CONFLICT
// turns the loser's insert into a harmless update instead of an error.
func ensureCart(tx *gorm.DB, userID uint) (uint, error) {
	now := time.Now()
	var id uint
	err := tx.Raw(`
		INSERT INTO carts (user_id, created_at, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = EXCLUDED.updated_at
		RETURNING id`, userID, now, now).Scan(&id).Error
	if err != nil {
		return 0, fmt.Errorf("ensure cart: %w", err)
	}
	return id, nil
}

// inUsersCart limits a cart_items query to rows in this user's cart. Every
// cart_items query goes through it, which is what makes it impossible to
// read or change another user's cart.
func inUsersCart(db *gorm.DB, userID uint) *gorm.DB {
	return db.Model(&models.CartItem{}).Where("cart_id = (SELECT id FROM carts WHERE user_id = ?)", userID)
}
