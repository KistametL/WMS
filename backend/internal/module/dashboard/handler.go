package dashboard

import (
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

// RegisterRoutes registers dashboard routes.
//
// Routes:
//
//	GET /dashboard         — full dashboard (overview + breakdown + queue + alerts + recent)
//	GET /dashboard/alerts  — stock alerts only (for frontend polling)
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// ต้องลง /dashboard/alerts ก่อน /dashboard เพื่อไม่ให้ router conflict
	// (Gin ใช้ exact match ก่อน wildcard แต่ group ต่างกันไม่ conflict)
	dashboard := protected.Group("/dashboard")
	{
		dashboard.GET("", middleware.RequirePermission("report:read"), h.GetDashboard)
		dashboard.GET("/alerts", middleware.RequirePermission("report:read"), h.GetAlerts)
	}
}

// GetDashboard godoc
// GET /dashboard
// ดึงข้อมูลครบทุก section — ใช้โหลดครั้งแรกเปิดหน้า dashboard
func (h *Handler) GetDashboard(c *gin.Context) {
	result, err := h.service.GetDashboard(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "dashboard: GetDashboard failed",
			"error", err,
		)
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}

// GetAlerts godoc
// GET /dashboard/alerts
// ดึงเฉพาะ stock alerts — frontend poll ทุก N วินาที
func (h *Handler) GetAlerts(c *gin.Context) {
	result, err := h.service.GetAlerts(c.Request.Context())
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "dashboard: GetAlerts failed",
			"error", err,
		)
		response.InternalError(c)
		return
	}
	response.OK(c, result)
}
