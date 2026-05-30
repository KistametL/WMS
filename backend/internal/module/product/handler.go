package product

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/KistametL/WMS/backend/internal/middleware"
	"github.com/KistametL/WMS/backend/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all product routes under the given protected router group.
// All routes require RequireAuth (already applied by the caller).
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// ── Categories ────────────────────────────────────────────────────
	cats := protected.Group("/categories")
	{
		cats.GET("", middleware.RequirePermission("product:read"), h.ListCategories)
		cats.POST("", middleware.RequirePermission("product:write"), h.CreateCategory)
		cats.GET("/:id", middleware.RequirePermission("product:read"), h.GetCategory)
		cats.PATCH("/:id", middleware.RequirePermission("product:write"), h.UpdateCategory)
		cats.DELETE("/:id", middleware.RequirePermission("product:write"), h.DeleteCategory)
	}

	// ── Products ──────────────────────────────────────────────────────
	prods := protected.Group("/products")
	{
		prods.GET("", middleware.RequirePermission("product:read"), h.ListProducts)
		prods.POST("", middleware.RequirePermission("product:write"), h.CreateProduct)
		prods.GET("/:id", middleware.RequirePermission("product:read"), h.GetProduct)
		prods.PATCH("/:id", middleware.RequirePermission("product:write"), h.UpdateProduct)
		prods.DELETE("/:id", middleware.RequirePermission("product:write"), h.DeleteProduct)

		// SKUs nested under product
		prods.GET("/:id/skus", middleware.RequirePermission("product:read"), h.ListSKUs)
		prods.POST("/:id/skus", middleware.RequirePermission("product:write"), h.CreateSKU)
	}

	// ── SKUs (top-level for PATCH / DELETE by SKU ID) ─────────────────
	skus := protected.Group("/skus")
	{
		skus.PATCH("/:id", middleware.RequirePermission("product:write"), h.UpdateSKU)
		skus.DELETE("/:id", middleware.RequirePermission("product:write"), h.DeleteSKU)
	}
}

// ── Categories ────────────────────────────────────────────────────────────────

func (h *Handler) ListCategories(c *gin.Context) {
	result, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.BadRequest(c, "id must be a positive integer")
		return
	}

	result, err := h.service.GetCategory(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "category not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreateCategory(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Created(c, result)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.BadRequest(c, "id must be a positive integer")
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdateCategory(c.Request.Context(), id, req, middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "category not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteCategory(c *gin.Context) {
	id, err := parseID(c, "id")
	if err != nil {
		response.BadRequest(c, "id must be a positive integer")
		return
	}

	if err := h.service.DeleteCategory(c.Request.Context(), id); err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, gin.H{"message": "category deleted"})
}

// ── Products ──────────────────────────────────────────────────────────────────

func (h *Handler) ListProducts(c *gin.Context) {
	var q ListProductsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ListProducts(c.Request.Context(), q)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetProduct(c *gin.Context) {
	id := c.Param("id")

	result, err := h.service.GetProduct(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "product not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreateProduct(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.Created(c, result)
}

func (h *Handler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")

	var req UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdateProduct(c.Request.Context(), id, req, middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "product not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteProduct(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "product not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, gin.H{"message": "product deleted"})
}

// ── SKUs ──────────────────────────────────────────────────────────────────────

func (h *Handler) ListSKUs(c *gin.Context) {
	productID := c.Param("id")

	result, err := h.service.GetProduct(c.Request.Context(), productID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "product not found")
			return
		}
		response.InternalError(c)
		return
	}
	// Return only the SKUs array from the product detail
	response.OK(c, result.SKUs)
}

func (h *Handler) CreateSKU(c *gin.Context) {
	productID := c.Param("id")

	var req CreateSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.CreateSKU(c.Request.Context(), productID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "product not found")
		case errors.Is(err, ErrDuplicateSKU):
			response.BadRequest(c, "sku_code already exists")
		default:
			response.InternalError(c)
		}
		return
	}
	response.Created(c, result)
}

func (h *Handler) UpdateSKU(c *gin.Context) {
	id := c.Param("id")

	var req UpdateSKURequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdateSKU(c.Request.Context(), id, req, middleware.GetUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "sku not found")
		case errors.Is(err, ErrDuplicateSKU):
			response.BadRequest(c, "sku_code already exists")
		default:
			response.InternalError(c)
		}
		return
	}
	response.OK(c, result)
}

func (h *Handler) DeleteSKU(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteSKU(c.Request.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "sku not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, gin.H{"message": "sku deleted"})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// parseID parses a URL parameter as a positive int32.
func parseID(c *gin.Context, param string) (int32, error) {
	n, err := strconv.ParseInt(c.Param(param), 10, 32)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid id")
	}
	return int32(n), nil
}
