package fulfillment

import (
	"errors"
	"log/slog"

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

// RegisterRoutes registers all fulfillment routes under the protected router group.
//
// Routes:
//
//	GET  /fulfillment/orders                      — queue/picking/packing list
//	POST /fulfillment/orders/:id/start-pick       — confirmed → picking
//	POST /fulfillment/orders/:id/confirm-pick     — picking → packing (+ barcode scan validation)
//	POST /fulfillment/orders/:id/confirm-pack     — packing → ready_to_ship
//	POST /fulfillment/orders/:id/ship             — ready_to_ship → shipped (+ shipment record)
//	GET  /fulfillment/orders/:id/shipment         — get shipment details
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	grp := protected.Group("/fulfillment/orders")
	{
		grp.GET("", middleware.RequirePermission("fulfillment:read"), h.ListOrders)
		grp.POST("/:id/start-pick", middleware.RequirePermission("fulfillment:write"), h.StartPick)
		grp.POST("/:id/confirm-pick", middleware.RequirePermission("fulfillment:write"), h.ConfirmPick)
		grp.POST("/:id/confirm-pack", middleware.RequirePermission("fulfillment:write"), h.ConfirmPack)
		grp.POST("/:id/ship", middleware.RequirePermission("fulfillment:write"), h.Ship)
		grp.GET("/:id/shipment", middleware.RequirePermission("fulfillment:read"), h.GetShipment)
	}
}

// ── Handlers ──────────────────────────────────────────────────────

// ListOrders godoc
// GET /fulfillment/orders?status=confirmed&page=1&limit=20
// status ต้องเป็น: confirmed | picking | packing | ready_to_ship (default: confirmed)
func (h *Handler) ListOrders(c *gin.Context) {
	var q QueueQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// validate status เป็น fulfillment status เท่านั้น
	if q.Status != "" && !fulfillmentStatuses[q.Status] {
		response.BadRequest(c, "status must be one of: confirmed, picking, packing, ready_to_ship")
		return
	}

	result, err := h.service.GetQueue(c.Request.Context(), q)
	if err != nil {
		h.internalError(c, "ListOrders", err)
		return
	}
	response.OK(c, result)
}

// StartPick godoc
// POST /fulfillment/orders/:id/start-pick
// transitions: confirmed → picking
func (h *Handler) StartPick(c *gin.Context) {
	id := c.Param("id")
	userID := getUserID(c)

	result, err := h.service.StartPick(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "StartPick", err)
		}
		return
	}
	response.OK(c, result)
}

// ConfirmPick godoc
// POST /fulfillment/orders/:id/confirm-pick
// Body: {"scanned_items": [{"sku_code": "SKU001", "quantity": 2}]}
// transitions: picking → packing (หลัง validate barcode scan)
func (h *Handler) ConfirmPick(c *gin.Context) {
	id := c.Param("id")

	var req ConfirmPickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)

	result, err := h.service.ConfirmPick(c.Request.Context(), id, req, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrScanMismatch):
			// 422: items ที่ scan ไม่ตรงกับ order — business rule violation
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "ConfirmPick", err)
		}
		return
	}
	response.OK(c, result)
}

// ConfirmPack godoc
// POST /fulfillment/orders/:id/confirm-pack
// transitions: packing → ready_to_ship
func (h *Handler) ConfirmPack(c *gin.Context) {
	id := c.Param("id")
	userID := getUserID(c)

	result, err := h.service.ConfirmPack(c.Request.Context(), id, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "ConfirmPack", err)
		}
		return
	}
	response.OK(c, result)
}

// Ship godoc
// POST /fulfillment/orders/:id/ship
// Body: {"courier": "kerry", "tracking_number": "TH123456789", "label_url": "..."}
// transitions: ready_to_ship → shipped + สร้าง shipment record
func (h *Handler) Ship(c *gin.Context) {
	id := c.Param("id")

	var req ShipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userID := getUserID(c)

	result, err := h.service.Ship(c.Request.Context(), id, req, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "order not found")
		case errors.Is(err, ErrInvalidTransition):
			response.UnprocessableEntity(c, err.Error())
		case errors.Is(err, ErrAlreadyShipped):
			response.Conflict(c, err.Error())
		case errors.Is(err, ErrInsufficientStock):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "Ship", err)
		}
		return
	}
	response.OK(c, result)
}

// GetShipment godoc
// GET /fulfillment/orders/:id/shipment
func (h *Handler) GetShipment(c *gin.Context) {
	id := c.Param("id")

	result, err := h.service.GetShipment(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "shipment not found")
			return
		}
		h.internalError(c, "GetShipment", err)
		return
	}
	response.OK(c, result)
}

// ── Helpers ───────────────────────────────────────────────────────

func getUserID(c *gin.Context) string {
	v, _ := c.Get(middleware.CtxUserID)
	s, _ := v.(string)
	return s
}

func (h *Handler) internalError(c *gin.Context, op string, err error) {
	slog.ErrorContext(c.Request.Context(), "fulfillment: unexpected error",
		"op", op,
		"method", c.Request.Method,
		"path", c.FullPath(),
		"order_id", c.Param("id"),
		"user_id", getUserID(c),
		"error", err,
	)
	response.InternalError(c)
}
