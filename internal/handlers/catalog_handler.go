package handlers

import (
	"errors"
	"net/http"

	"github.com/UncleKweks/E-Commerce/internal/dto"
	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/UncleKweks/E-Commerce/internal/services"
	"github.com/gin-gonic/gin"
)

// Page size used when the client doesn't send ?page_size=.
const defaultPageSize = 20

// CatalogHandler serves the category and product endpoints, both the public
// ones and the admin ones.
type CatalogHandler struct {
	catalog *services.CatalogService
}

// NewCatalogHandler returns a CatalogHandler.
func NewCatalogHandler(catalog *services.CatalogService) *CatalogHandler {
	return &CatalogHandler{catalog: catalog}
}

// ---- Categories ----

// ListCategories handles GET /categories (active only).
func (h *CatalogHandler) ListCategories(c *gin.Context) { h.listCategories(c, false) }

// AdminListCategories handles GET /admin/categories (including hidden ones).
func (h *CatalogHandler) AdminListCategories(c *gin.Context) { h.listCategories(c, true) }

func (h *CatalogHandler) listCategories(c *gin.Context, includeInactive bool) {
	cats, err := h.catalog.ListCategories(c.Request.Context(), includeInactive)
	if err != nil {
		internalError(c, err)
		return
	}
	response.OK(c, dto.NewCategoryList(cats))
}

// CreateCategory handles POST /admin/categories.
func (h *CatalogHandler) CreateCategory(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	cat, err := h.catalog.CreateCategory(c.Request.Context(), &req)
	if err != nil {
		categoryError(c, err)
		return
	}
	response.Created(c, dto.NewCategoryResponse(cat))
}

// UpdateCategory handles PATCH /admin/categories/:id.
func (h *CatalogHandler) UpdateCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req dto.UpdateCategoryRequest
	if !bindJSON(c, &req) {
		return
	}
	cat, err := h.catalog.UpdateCategory(c.Request.Context(), id, &req)
	if err != nil {
		categoryError(c, err)
		return
	}
	response.OK(c, dto.NewCategoryResponse(cat))
}

// DeleteCategory handles DELETE /admin/categories/:id.
func (h *CatalogHandler) DeleteCategory(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.catalog.DeleteCategory(c.Request.Context(), id); err != nil {
		categoryError(c, err)
		return
	}
	response.NoContent(c)
}

func categoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrCategoryNotFound):
		response.Error(c, http.StatusNotFound, "category_not_found", "Category not found.")
	case errors.Is(err, services.ErrCategoryNameTaken):
		response.Error(c, http.StatusConflict, "category_name_taken", "A category with this name already exists.")
	case errors.Is(err, services.ErrCategoryInUse):
		response.Error(c, http.StatusConflict, "category_in_use",
			"This category still has products. Move or delete them first.")
	default:
		internalError(c, err)
	}
}

// ---- Products ----

// ListProducts handles GET /products: active products only.
func (h *CatalogHandler) ListProducts(c *gin.Context) { h.listProducts(c, false) }

// AdminListProducts handles GET /admin/products, including hidden products.
func (h *CatalogHandler) AdminListProducts(c *gin.Context) { h.listProducts(c, true) }

func (h *CatalogHandler) listProducts(c *gin.Context, includeInactive bool) {
	var q dto.ProductListQuery
	if !bindQuery(c, &q) {
		return
	}
	if q.MinPrice != nil && q.MaxPrice != nil && *q.MinPrice > *q.MaxPrice {
		response.ValidationError(c, map[string]string{"min_price": "must not be greater than max_price"})
		return
	}

	filter := services.ProductFilter{
		Page:            q.Page,
		PageSize:        q.PageSize,
		CategoryID:      q.CategoryID,
		Search:          q.Search,
		MinPrice:        q.MinPrice,
		MaxPrice:        q.MaxPrice,
		InStock:         q.InStock,
		Sort:            q.Sort,
		IncludeInactive: includeInactive,
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = defaultPageSize
	}

	products, total, err := h.catalog.ListProducts(c.Request.Context(), &filter)
	if err != nil {
		internalError(c, err)
		return
	}
	response.Paged(c, dto.NewProductList(products), response.NewPageMeta(filter.Page, filter.PageSize, total))
}

// GetProduct handles GET /products/:id.
func (h *CatalogHandler) GetProduct(c *gin.Context) { h.getProduct(c, false) }

// AdminGetProduct handles GET /admin/products/:id, including hidden products.
func (h *CatalogHandler) AdminGetProduct(c *gin.Context) { h.getProduct(c, true) }

func (h *CatalogHandler) getProduct(c *gin.Context, includeInactive bool) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	p, err := h.catalog.GetProduct(c.Request.Context(), id, includeInactive)
	if err != nil {
		productError(c, err)
		return
	}
	response.OK(c, dto.NewProductResponse(p))
}

// CreateProduct handles POST /admin/products.
func (h *CatalogHandler) CreateProduct(c *gin.Context) {
	var req dto.CreateProductRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.catalog.CreateProduct(c.Request.Context(), &req)
	if err != nil {
		productError(c, err)
		return
	}
	response.Created(c, dto.NewProductResponse(p))
}

// UpdateProduct handles PATCH /admin/products/:id.
func (h *CatalogHandler) UpdateProduct(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req dto.UpdateProductRequest
	if !bindJSON(c, &req) {
		return
	}
	p, err := h.catalog.UpdateProduct(c.Request.Context(), id, &req)
	if err != nil {
		productError(c, err)
		return
	}
	response.OK(c, dto.NewProductResponse(p))
}

// DeleteProduct handles DELETE /admin/products/:id.
func (h *CatalogHandler) DeleteProduct(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.catalog.DeleteProduct(c.Request.Context(), id); err != nil {
		productError(c, err)
		return
	}
	response.NoContent(c)
}

func productError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrProductNotFound):
		response.Error(c, http.StatusNotFound, "product_not_found", "Product not found.")
	case errors.Is(err, services.ErrCategoryNotFound):
		// The product's category_id points at nothing: that's bad input (422),
		// not a missing page (404).
		response.ValidationError(c, map[string]string{"category_id": "does not exist"})
	case errors.Is(err, services.ErrSKUTaken):
		response.Error(c, http.StatusConflict, "sku_taken", "Another product already uses this SKU.")
	default:
		internalError(c, err)
	}
}
