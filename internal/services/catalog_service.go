package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Errors the catalog service can return.
var (
	ErrCategoryNotFound  = errors.New("category not found")
	ErrCategoryNameTaken = errors.New("category name already exists")
	ErrCategoryInUse     = errors.New("category still has products")
	ErrProductNotFound   = errors.New("product not found")
	ErrSKUTaken          = errors.New("sku already exists")
)

// productSorts maps the ?sort= values clients may send to ORDER BY clauses.
// User input never goes into SQL directly: it's only used to look up one of
// these fixed strings. Every option ends with id so that products with the
// same price or date always come back in the same order, which keeps
// pagination from skipping or repeating items.
var productSorts = map[string]string{
	"newest":     "created_at DESC, id DESC",
	"price_asc":  "price_cents ASC, id ASC",
	"price_desc": "price_cents DESC, id DESC",
	"name":       "lower(name) ASC, id ASC",
}

// DefaultProductSort is used when the client doesn't choose one.
const DefaultProductSort = "newest"

// ProductFilter describes which products to list and which page of them.
type ProductFilter struct {
	Page, PageSize int
	CategoryID     uint
	Search         string
	MinPrice       *int64
	MaxPrice       *int64
	InStock        bool
	Sort           string
	// IncludeInactive shows hidden products and products in hidden
	// categories. Only admin routes set it.
	IncludeInactive bool
}

// CatalogService manages categories and products.
type CatalogService struct {
	db *gorm.DB
}

// NewCatalogService returns a CatalogService.
func NewCatalogService(db *gorm.DB) *CatalogService {
	return &CatalogService{db: db}
}

// ---- Categories ----

// ListCategories returns categories sorted by name. Shops have tens of
// categories, not millions, so this list isn't paginated.
func (s *CatalogService) ListCategories(ctx context.Context, includeInactive bool) ([]models.Category, error) {
	q := s.db.WithContext(ctx).Order("lower(name)")
	if !includeInactive {
		q = q.Where("is_active")
	}
	var cats []models.Category
	if err := q.Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return cats, nil
}

// CreateCategory adds a category.
func (s *CatalogService) CreateCategory(ctx context.Context, req *dto.CreateCategoryRequest) (*models.Category, error) {
	cat := models.Category{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		IsActive:    boolOr(req.IsActive, true),
	}
	if err := s.db.WithContext(ctx).Create(&cat).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrCategoryNameTaken
		}
		return nil, fmt.Errorf("create category: %w", err)
	}
	return &cat, nil
}

// UpdateCategory changes only the fields that were sent.
func (s *CatalogService) UpdateCategory(ctx context.Context, id uint, req *dto.UpdateCategoryRequest) (*models.Category, error) {
	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	db := s.db.WithContext(ctx)
	if len(updates) > 0 {
		res := db.Model(&models.Category{}).Where("id = ?", id).Updates(updates)
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return nil, ErrCategoryNameTaken
		}
		if res.Error != nil {
			return nil, fmt.Errorf("update category: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return nil, ErrCategoryNotFound
		}
	}

	var cat models.Category
	if err := db.First(&cat, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("find category: %w", err)
	}
	return &cat, nil
}

// DeleteCategory soft-deletes a category, but only if no products use it.
func (s *CatalogService) DeleteCategory(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// FOR UPDATE locks the category row until this transaction ends.
		// CreateProduct locks the same row (FOR SHARE) before adding a product
		// to it, so "count the products, then delete" can't race with someone
		// adding a product in between.
		cat, err := lockCategory(tx, id, "UPDATE")
		if err != nil {
			return err
		}

		var count int64
		if err := tx.Model(&models.Product{}).Where("category_id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("count products: %w", err)
		}
		if count > 0 {
			return ErrCategoryInUse
		}

		if err := tx.Delete(cat).Error; err != nil {
			return fmt.Errorf("delete category: %w", err)
		}
		return nil
	})
}

// ---- Products ----

// ListProducts returns one page of products matching the filter, plus the
// total number of matches (for "page 2 of 5").
func (s *CatalogService) ListProducts(ctx context.Context, f *ProductFilter) ([]models.Product, int64, error) {
	order, ok := productSorts[f.Sort]
	if !ok {
		order = productSorts[DefaultProductSort]
	}

	q := s.db.WithContext(ctx).Model(&models.Product{})
	if !f.IncludeInactive {
		q = visibleProducts(q)
	}
	if f.CategoryID != 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}
	if f.Search != "" {
		// ? placeholders send the value separately from the SQL, so it can
		// never be run as SQL. escapeLike stops % and _ acting as wildcards.
		q = q.Where("name ILIKE ?", "%"+escapeLike(f.Search)+"%")
	}
	if f.MinPrice != nil {
		q = q.Where("price_cents >= ?", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		q = q.Where("price_cents <= ?", *f.MaxPrice)
	}
	if f.InStock {
		q = q.Where("stock > 0")
	}
	// A fresh session lets us run two queries (count, then fetch) from the
	// same filters without the second one inheriting leftovers from the first.
	q = q.Session(&gorm.Session{})

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}
	products := []models.Product{}
	if total == 0 {
		return products, 0, nil
	}

	err := q.Preload("Category").
		Order(order).
		Limit(f.PageSize).
		Offset((f.Page - 1) * f.PageSize).
		Find(&products).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	return products, total, nil
}

// GetProduct returns one product. Unless includeInactive is set, hidden
// products are reported as not found, exactly like products that don't exist.
func (s *CatalogService) GetProduct(ctx context.Context, id uint, includeInactive bool) (*models.Product, error) {
	q := s.db.WithContext(ctx).Preload("Category")
	if !includeInactive {
		q = visibleProducts(q)
	}
	var p models.Product
	if err := q.First(&p, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("find product: %w", err)
	}
	return &p, nil
}

// CreateProduct adds a product to an existing category.
func (s *CatalogService) CreateProduct(ctx context.Context, req *dto.CreateProductRequest) (*models.Product, error) {
	p := models.Product{
		CategoryID:  req.CategoryID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		PriceCents:  *req.PriceCents,
		Stock:       req.Stock,
		SKU:         normalizeSKU(req.SKU),
		IsActive:    boolOr(req.IsActive, true),
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		cat, err := lockCategory(tx, req.CategoryID, "SHARE")
		if err != nil {
			return err
		}
		if err := tx.Create(&p).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrSKUTaken
			}
			return fmt.Errorf("create product: %w", err)
		}
		p.Category = cat
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateProduct changes only the fields that were sent.
func (s *CatalogService) UpdateProduct(ctx context.Context, id uint, req *dto.UpdateProductRequest) (*models.Product, error) {
	// A map, not a struct: GORM skips zero values in a struct, so
	// Updates(Product{Stock: 0}) would silently NOT set stock to 0.
	updates := map[string]any{}
	if req.CategoryID != nil {
		updates["category_id"] = *req.CategoryID
	}
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.PriceCents != nil {
		updates["price_cents"] = *req.PriceCents
	}
	if req.Stock != nil {
		updates["stock"] = *req.Stock
	}
	if req.SKU != nil {
		updates["sku"] = normalizeSKU(*req.SKU)
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.CategoryID != nil {
			if _, err := lockCategory(tx, *req.CategoryID, "SHARE"); err != nil {
				return err
			}
		}
		if len(updates) == 0 {
			return nil
		}
		res := tx.Model(&models.Product{}).Where("id = ?", id).Updates(updates)
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return ErrSKUTaken
		}
		if res.Error != nil {
			return fmt.Errorf("update product: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrProductNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetProduct(ctx, id, true)
}

// DeleteProduct soft-deletes a product. Past orders keep their own copy of
// its name and price, so they're unaffected.
func (s *CatalogService) DeleteProduct(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.Product{}, id)
	if res.Error != nil {
		return fmt.Errorf("delete product: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrProductNotFound
	}
	return nil
}

// ---- helpers ----

// visibleProducts limits a query to what customers may see: active products
// in active categories. Soft-deleted rows are already excluded by GORM.
func visibleProducts(q *gorm.DB) *gorm.DB {
	return q.Where("is_active").
		Where("category_id IN (SELECT id FROM categories WHERE is_active AND deleted_at IS NULL)")
}

// lockCategory loads a category and locks its row for the rest of the
// transaction. strength is "UPDATE" (exclusive: we're about to change it)
// or "SHARE" (we only need it to stay put while we use it).
func lockCategory(tx *gorm.DB, id uint, strength string) (*models.Category, error) {
	var cat models.Category
	err := tx.Clauses(clause.Locking{Strength: strength}).First(&cat, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock category: %w", err)
	}
	return &cat, nil
}

// escapeLike makes %, _ and \ match themselves in an ILIKE pattern, so a
// search for "100%" finds "100% cotton" instead of matching everything.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// normalizeSKU stores SKUs in one form, so "abc-1" and "ABC-1" can't both exist.
func normalizeSKU(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func boolOr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
