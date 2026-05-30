package order

import (
	"errors"

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

// RegisterRoutes registers all order routes under the protected router group.
// All routes already go through RequireAuth (applied by caller).
//
// Routes:
//
//	GET    /orders              — list with filters
//	POST   /orders              — create
//	GET    /orders/:id          — get detail
//	PATCH  /orders/:id          — update customer/shipping/note
//	POST   /orders/:id/status   — transition status (state machine)
//	POST   /orders/:id/cancel   — cancel (dedicated, with reason)
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	orders := protected.Group("/orders")
	{
		orders.GET("", middleware.RequirePermission("order:read"), h.ListOrders)
		orders.POST("", middleware.RequirePermission("order:write"), h.CreateOrder)
		orders.GET("/:id", middleware.RequirePermission("order:read"), h.GetOrder)
		orders.PATCH("/:id", middleware.RequirePermission("order:write"), h.UpdateOrder)
		orders.POST("/:id/status", middleware.RequirePermission("order:write"), h.UpdateStatus)
		orders.POST("/:id/cancel", middleware.RequirePermission("order:write"), h.CancelOrder)
	}
}

// ── Handlers ──────────────────────────────────────────────────────

// ListOrders godoc
// GET /orders?status=pending&channel=manual&is_cod=false&search=ORD&page=1&limit=20
func (h *Handler) ListOrders(c *gin.Context) {
	var q ListOrdersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.ListOrders(c.Request.Context(), q)
	if err != nil {
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// CreateOrder godoc
// POST /orders
func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)

	result, err := h.service.CreateOrder(c.Request.Context(), req, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrSKUNotFound),
			errors.Is(err, ErrSKUInactive),
			errors.Is(err, ErrInsufficientStock):
			// 422: business rule violation (SKU ไม่มี / inactive / stock ไม่พอ)
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrDuplicateChannelOrder):
			// 409: channel webhook ส่งซ้ำ หรือ duplicate request
			response.Conflict(c, err.Error())
		default:
			// 500: DB error หรือ unexpected — ไม่ส่ง raw error ออก (security)
			response.InternalError(c)
		}
		return
	}
	response.Created(c, result)
}

// GetOrder godoc
// GET /orders/:id
func (h *Handler) GetOrder(c *gin.Context) {
	id := c.Param("id")

	result, err := h.service.GetOrder(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "order not found")
			return
		}
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// UpdateOrder godoc
// PATCH /orders/:id
func (h *Handler) UpdateOrder(c *gin.Context) {
	id := c.Param("id")

	var req UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := h.service.UpdateOrder(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrOrderNotEditable):
			response.UnprocessableEntity(c, err.Error())
		default:
			response.InternalError(c)
		}
		return
	}
	response.OK(c, result)
}

// UpdateStatus godoc
// POST /orders/:id/status
// Body: {"status": "confirmed", "note": "..."}
func (h *Handler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")

	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)

	result, err := h.service.UpdateStatus(c.Request.Context(), id, req, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrInsufficientStock):
			response.UnprocessableEntity(c, err.Error())
		default:
			response.InternalError(c)
		}
		return
	}
	response.OK(c, result)
}

// CancelOrder godoc
// POST /orders/:id/cancel
// Body: {"reason": "..."}  (optional)
func (h *Handler) CancelOrder(c *gin.Context) {
	id := c.Param("id")

	var req CancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)

	result, err := h.service.CancelOrder(c.Request.Context(), id, req, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		default:
			response.InternalError(c)
		}
		return
	}
	response.OK(c, result)
}

// ── Helper ────────────────────────────────────────────────────────

func getUserID(c *gin.Context) string {
	v, _ := c.Get(middleware.CtxUserID)
	s, _ := v.(string)
	return s
}
