package dto

// AddCartItemRequest is the body of POST /cart/items.
// Adding a product that's already in the cart increases its quantity.
type AddCartItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	// A pointer, so "not sent" (add 1) is different from an explicit 0 (rejected).
	Quantity *int `json:"quantity" binding:"omitempty,min=1,max=99"`
}

// UpdateCartItemRequest is the body of PUT /cart/items/:product_id.
// It sets the quantity to exactly this number.
type UpdateCartItemRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1,max=99"`
}

// Cart item statuses. The cart is checked against the live catalog every
// time it's read, because products change while they sit in a cart.
const (
	CartItemOK             = "ok"
	CartItemUnavailable    = "unavailable"      // deleted or hidden since it was added
	CartItemOutOfStock     = "out_of_stock"     // stock is 0
	CartItemNotEnoughStock = "not_enough_stock" // some stock, but less than the quantity
)

// CartItemResponse is one line in the cart.
type CartItemResponse struct {
	ProductID      uint   `json:"product_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	UnitPriceCents int64  `json:"unit_price_cents"` // today's price, not the price when added
	Quantity       int    `json:"quantity"`
	LineTotalCents int64  `json:"line_total_cents"`
	Available      int    `json:"available"` // how many can be bought right now
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
}

// CartResponse is the whole cart, as returned by every cart endpoint.
type CartResponse struct {
	Items []CartItemResponse `json:"items"`
	// ItemCount is the total number of units, e.g. 2 shirts + 1 mug = 3.
	ItemCount int `json:"item_count"`
	// SubtotalCents only counts lines with status "ok".
	SubtotalCents int64 `json:"subtotal_cents"`
	// CanCheckout is false when any line has a problem the customer must fix first.
	CanCheckout bool `json:"can_checkout"`
}
