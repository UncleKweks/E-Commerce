package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/middleware"
	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/UncleKweks/E-Commerce/internal/services"
	"github.com/gin-gonic/gin"
)

// CartHandler serves /cart. Every route sits behind AuthRequired, and the
// user ID always comes from the token, never from the URL or the body.
type CartHandler struct {
	carts *services.CartService
}

// NewCartHandler returns a CartHandler.
func NewCartHandler(carts *services.CartService) *CartHandler {
	return &CartHandler{carts: carts}
}

// Get handles GET /cart.
func (h *CartHandler) Get(c *gin.Context) {
	cart, err := h.carts.GetCart(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		internalError(c, err)
		return
	}
	response.OK(c, cart)
}

// AddItem handles POST /cart/items.
func (h *CartHandler) AddItem(c *gin.Context) {
	var req dto.AddCartItemRequest
	if !bindJSON(c, &req) {
		return
	}
	quantity := 1
	if req.Quantity != nil {
		quantity = *req.Quantity
	}

	cart, err := h.carts.AddItem(c.Request.Context(), middleware.UserID(c), req.ProductID, quantity)
	if err != nil {
		cartError(c, err, true)
		return
	}
	response.OK(c, cart)
}

// SetQuantity handles PUT /cart/items/:product_id.
func (h *CartHandler) SetQuantity(c *gin.Context) {
	productID, ok := uintParam(c, "product_id")
	if !ok {
		return
	}
	var req dto.UpdateCartItemRequest
	if !bindJSON(c, &req) {
		return
	}

	cart, err := h.carts.SetQuantity(c.Request.Context(), middleware.UserID(c), productID, req.Quantity)
	if err != nil {
		cartError(c, err, false)
		return
	}
	response.OK(c, cart)
}

// RemoveItem handles DELETE /cart/items/:product_id.
func (h *CartHandler) RemoveItem(c *gin.Context) {
	productID, ok := uintParam(c, "product_id")
	if !ok {
		return
	}
	cart, err := h.carts.RemoveItem(c.Request.Context(), middleware.UserID(c), productID)
	if err != nil {
		internalError(c, err)
		return
	}
	response.OK(c, cart)
}

// Clear handles DELETE /cart.
func (h *CartHandler) Clear(c *gin.Context) {
	cart, err := h.carts.Clear(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		internalError(c, err)
		return
	}
	response.OK(c, cart)
}

// cartError maps service errors to responses. adding says whether the
// customer was adding more (POST) or setting an exact quantity (PUT),
// which changes how the stock message reads.
func cartError(c *gin.Context, err error, adding bool) {
	var stock *services.NotEnoughStockError
	switch {
	case errors.As(err, &stock):
		response.ErrorWithDetails(c, http.StatusConflict, "not_enough_stock", stockMessage(stock, adding),
			map[string]string{"max": strconv.Itoa(stock.Max), "in_cart": strconv.Itoa(stock.InCart)})
	case errors.Is(err, services.ErrProductUnavailable):
		// Hidden and deleted products look exactly like missing ones (lesson 3).
		response.Error(c, http.StatusNotFound, "product_not_found", "This product doesn't exist or is no longer for sale.")
	case errors.Is(err, services.ErrCartItemNotFound):
		response.Error(c, http.StatusNotFound, "cart_item_not_found", "This product isn't in your cart.")
	default:
		internalError(c, err)
	}
}

func stockMessage(e *services.NotEnoughStockError, adding bool) string {
	switch {
	case e.Max == 0:
		return "This product is out of stock."
	case adding && e.InCart > 0:
		return fmt.Sprintf("You can have at most %d of this product, and %d are already in your cart.", e.Max, e.InCart)
	default:
		return fmt.Sprintf("You can have at most %d of this product.", e.Max)
	}
}
