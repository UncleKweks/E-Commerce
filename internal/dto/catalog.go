package dto

import (
	"time"

	"github.com/UncleKweks/E-Commerce/internal/models"
)

// The max= limits below protect the database and the frontend from absurd
// values: a 10 MB product name, or a price of 9 quintillion.
// price_cents max=100000000 is 1,000,000.00.

// ---- Categories ----

// CreateCategoryRequest is the body of POST /admin/categories.
type CreateCategoryRequest struct {
	Name        string `json:"name" binding:"required,notblank,max=100"`
	Description string `json:"description" binding:"max=1000"`
	// A pointer, so "not sent" (nil, use the default: true) is different from "false".
	IsActive *bool `json:"is_active"`
}

// UpdateCategoryRequest is the body of PATCH /admin/categories/:id.
// Every field is a pointer: nil means "leave it alone", so a client can
// change one field without resending the others.
type UpdateCategoryRequest struct {
	Name        *string `json:"name" binding:"omitempty,notblank,max=100"`
	Description *string `json:"description" binding:"omitempty,max=1000"`
	IsActive    *bool   `json:"is_active"`
}

// CategoryResponse is the public view of a category.
type CategoryResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewCategoryResponse converts a database category into its public view.
func NewCategoryResponse(c *models.Category) *CategoryResponse {
	return &CategoryResponse{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		IsActive:    c.IsActive,
		CreatedAt:   c.CreatedAt.UTC(),
		UpdatedAt:   c.UpdatedAt.UTC(),
	}
}

// NewCategoryList converts a slice. It returns [] rather than null when empty,
// so clients can always loop over the result.
func NewCategoryList(cats []models.Category) []*CategoryResponse {
	out := make([]*CategoryResponse, 0, len(cats))
	for i := range cats {
		out = append(out, NewCategoryResponse(&cats[i]))
	}
	return out
}

// ---- Products ----

// CreateProductRequest is the body of POST /admin/products.
type CreateProductRequest struct {
	CategoryID  uint   `json:"category_id" binding:"required"`
	Name        string `json:"name" binding:"required,notblank,max=200"`
	Description string `json:"description" binding:"max=5000"`
	// A pointer so that 0 (a free item) passes "required", while leaving it out fails.
	PriceCents *int64 `json:"price_cents" binding:"required,min=0,max=100000000"`
	Stock      int    `json:"stock" binding:"min=0,max=1000000"`
	SKU        string `json:"sku" binding:"required,sku,max=64"`
	IsActive   *bool  `json:"is_active"`
}

// UpdateProductRequest is the body of PATCH /admin/products/:id. nil = unchanged.
type UpdateProductRequest struct {
	CategoryID  *uint   `json:"category_id" binding:"omitempty,min=1"`
	Name        *string `json:"name" binding:"omitempty,notblank,max=200"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
	PriceCents  *int64  `json:"price_cents" binding:"omitempty,min=0,max=100000000"`
	Stock       *int    `json:"stock" binding:"omitempty,min=0,max=1000000"`
	SKU         *string `json:"sku" binding:"omitempty,sku,max=64"`
	IsActive    *bool   `json:"is_active"`
}

// ProductListQuery is the query string of GET /products,
// e.g. ?category_id=2&q=shirt&min_price=1000&sort=price_asc&page=2.
type ProductListQuery struct {
	Page       int    `form:"page" binding:"omitempty,min=1,max=10000"`
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	CategoryID uint   `form:"category_id"`
	Search     string `form:"q" binding:"max=100"`
	MinPrice   *int64 `form:"min_price" binding:"omitempty,min=0"`
	MaxPrice   *int64 `form:"max_price" binding:"omitempty,min=0"`
	InStock    bool   `form:"in_stock"`
	Sort       string `form:"sort" binding:"omitempty,oneof=newest price_asc price_desc name"`
}

// CategoryRef is the short form of a category, embedded in a product.
type CategoryRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// ProductResponse is the public view of a product.
type ProductResponse struct {
	ID          uint         `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	PriceCents  int64        `json:"price_cents"`
	Stock       int          `json:"stock"`
	SKU         string       `json:"sku"`
	IsActive    bool         `json:"is_active"`
	Category    *CategoryRef `json:"category,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// NewProductResponse converts a database product into its public view.
func NewProductResponse(p *models.Product) *ProductResponse {
	r := &ProductResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
		Stock:       p.Stock,
		SKU:         p.SKU,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt.UTC(),
		UpdatedAt:   p.UpdatedAt.UTC(),
	}
	if p.Category != nil {
		r.Category = &CategoryRef{ID: p.Category.ID, Name: p.Category.Name}
	}
	return r
}

// NewProductList converts a slice, returning [] rather than null when empty.
func NewProductList(products []models.Product) []*ProductResponse {
	out := make([]*ProductResponse, 0, len(products))
	for i := range products {
		out = append(out, NewProductResponse(&products[i]))
	}
	return out
}
