package inventory

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

// RegisterRoutes registers inventory routes under the given protected router group.
// All routes already go through RequireAuth (applied by caller).
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	inv := protected.Group("/inventory")

	// ── Stock Levels ────────────────────────────────────────────────
	stock := inv.Group("/stock")
	{
		stock.GET("", middleware.RequirePermission("inventory:read"), h.ListStock)
		stock.GET("/:sku_id", middleware.RequirePermission("inventory:read"), h.GetStock)
		stock.PUT("/:sku_id/threshold", middleware.RequirePermission("inventory:write"), h.UpdateThreshold)
	}

	// ── Movements ───────────────────────────────────────────────────
	movements := inv.Group("/movements")
	{
		movements.GET("", middleware.RequirePermission("inventory:read"), h.ListMovements)
		movements.POST("", middleware.RequirePermission("inventory:write"), h.CreateMovement)
		movements.GET("/:id", middleware.RequirePermission("inventory:read"), h.GetMovement)
	}
}

// ── Stock Level Handlers ──────────────────────────────────────────

// ListStock godoc
// GET /inventory/stock?page=1&limit=20&low_stock_only=true
func (h *Handler) ListStock(c *gin.Context) {
	var q ListStockQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ListStock(c.Request.Context(), q)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// GetStock godoc
// GET /inventory/stock/:sku_id
func (h *Handler) GetStock(c *gin.Context) {
	skuID := c.Param("sku_id")

	result, err := h.service.GetStockBySKU(c.Request.Context(), skuID)
	if err != nil {
		if errors.Is(err, ErrStockNotFound) || errors.Is(err, ErrNotFound) {
			response.NotFound(c, "stock level not found — create a movement first to initialize")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// UpdateThreshold godoc
// PUT /inventory/stock/:sku_id/threshold
func (h *Handler) UpdateThreshold(c *gin.Context) {
	skuID := c.Param("sku_id")

	var req UpdateThresholdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdateLowStockThreshold(c.Request.Context(), skuID, req.LowStockThreshold)
	if err != nil {
		if errors.Is(err, ErrStockNotFound) || errors.Is(err, ErrNotFound) {
			response.NotFound(c, "stock level not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// ── Movement Handlers ─────────────────────────────────────────────

// CreateMovement godoc
// POST /inventory/movements
func (h *Handler) CreateMovement(c *gin.Context) {
	var req CreateMovementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID, _ := c.Get(middleware.CtxUserID)
	userIDStr, _ := userID.(string)

	result, err := h.service.CreateMovement(c.Request.Context(), req, userIDStr)
	if err != nil {
		switch {
		case errors.Is(err, ErrInsufficientStock):
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrNegativeStock):
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "SKU not found")
		default:
			response.InternalError(c)
		}
		return
	}
	response.Created(c, result)
}

// GetMovement godoc
// GET /inventory/movements/:id
func (h *Handler) GetMovement(c *gin.Context) {
	id, err := parseMovementID(c)
	if err != nil {
		response.BadRequest(c, "id must be a positive integer")
		return
	}

	result, err := h.service.GetMovement(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "movement not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// ListMovements godoc
// GET /inventory/movements?sku_id=...&movement_type=receive&page=1&limit=20
func (h *Handler) ListMovements(c *gin.Context) {
	var q ListMovementsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ListMovements(c.Request.Context(), q)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// ── Helper ────────────────────────────────────────────────────────

func parseMovementID(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}
