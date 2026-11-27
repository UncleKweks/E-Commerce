package services

import (
	"testing"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/models"
	"gorm.io/gorm"
)

func product(price int64, stock int) *models.Product {
	return &models.Product{
		Name: "Thing", PriceCents: price, Stock: stock, IsActive: true,
		Category: &models.Category{IsActive: true},
	}
}

func TestBuildCartStatuses(t *testing.T) {
	deleted := product(500, 10)
	deleted.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	hidden := product(500, 10)
	hidden.IsActive = false
	hiddenCategory := product(500, 10)
	hiddenCategory.Category.IsActive = false

	cases := []struct {
		name    string
		product *models.Product
		qty     int
		want    string
	}{
		{"fine", product(500, 10), 2, dto.CartItemOK},
		{"exactly the stock", product(500, 3), 3, dto.CartItemOK},
		{"more than the stock", product(500, 3), 4, dto.CartItemNotEnoughStock},
		{"out of stock", product(500, 0), 1, dto.CartItemOutOfStock},
		{"deleted", deleted, 1, dto.CartItemUnavailable},
		{"hidden", hidden, 1, dto.CartItemUnavailable},
		{"hidden category", hiddenCategory, 1, dto.CartItemUnavailable},
		{"product row gone", nil, 1, dto.CartItemUnavailable},
	}
	for _, tc := range cases {
		cart := buildCart([]models.CartItem{{Quantity: tc.qty, Product: tc.product}})
		if got := cart.Items[0].Status; got != tc.want {
			t.Errorf("%s: status = %q, want %q", tc.name, got, tc.want)
		}
		if wantCheckout := tc.want == dto.CartItemOK; cart.CanCheckout != wantCheckout {
			t.Errorf("%s: CanCheckout = %v, want %v", tc.name, cart.CanCheckout, wantCheckout)
		}
	}
}

func TestBuildCartTotals(t *testing.T) {
	cart := buildCart([]models.CartItem{
		{Quantity: 2, Product: product(1999, 10)}, // 39.98
		{Quantity: 1, Product: product(500, 10)},  //  5.00
		{Quantity: 5, Product: product(100, 2)},   // not enough stock: left out of the subtotal
	})
	if cart.SubtotalCents != 4498 {
		t.Errorf("SubtotalCents = %d, want 4498", cart.SubtotalCents)
	}
	if cart.ItemCount != 8 {
		t.Errorf("ItemCount = %d, want 8", cart.ItemCount)
	}
	if cart.CanCheckout {
		t.Error("CanCheckout should be false while a line has a problem")
	}
}

func TestEmptyCart(t *testing.T) {
	cart := buildCart(nil)
	if cart.Items == nil {
		t.Error("Items should be an empty list, not nil, so the JSON says [] instead of null")
	}
	if cart.CanCheckout {
		t.Error("an empty cart can't be checked out")
	}
}

func TestPurchaseLimit(t *testing.T) {
	if got := purchaseLimit(product(1, 500)); got != MaxQuantityPerItem {
		t.Errorf("plenty of stock: limit = %d, want %d", got, MaxQuantityPerItem)
	}
	if got := purchaseLimit(product(1, 3)); got != 3 {
		t.Errorf("low stock: limit = %d, want 3", got)
	}
}
