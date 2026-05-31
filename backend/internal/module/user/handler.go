package user

import (
	"errors"
	"log/slog"
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

// RegisterRoutes registers all user management routes.
//
// Routes:
//
//	GET    /users                      — list users
//	POST   /users                      — create user
//	GET    /users/:id                  — get user detail
//	PATCH  /users/:id                  — update user info / deactivate
//	DELETE /users/:id                  — soft delete user
//	POST   /users/:id/roles            — assign role
//	DELETE /users/:id/roles/:roleId    — remove role
//	POST   /users/:id/reset-password   — admin resets password
//	POST   /users/me/password          — user changes own password
//	GET    /roles                      — list available roles
func (h *Handler) RegisterRoutes(protected *gin.RouterGroup) {
	// /users/me/password ต้องลงทะเบียนก่อน /users/:id
	// เพื่อไม่ให้ Gin treat "me" เป็น :id
	protected.POST("/users/me/password", h.ChangePassword)

	users := protected.Group("/users")
	{
		users.GET("", middleware.RequirePermission("user:read"), h.ListUsers)
		users.POST("", middleware.RequirePermission("user:write"), h.CreateUser)
		users.GET("/:id", middleware.RequirePermission("user:read"), h.GetUser)
		users.PATCH("/:id", middleware.RequirePermission("user:write"), h.UpdateUser)
		users.DELETE("/:id", middleware.RequirePermission("user:write"), h.DeleteUser)
		users.POST("/:id/roles", middleware.RequirePermission("user:write"), h.AssignRole)
		users.DELETE("/:id/roles/:roleId", middleware.RequirePermission("user:write"), h.RemoveRole)
		users.POST("/:id/reset-password", middleware.RequirePermission("user:write"), h.ResetPassword)
	}

	protected.GET("/roles", middleware.RequirePermission("user:read"), h.ListRoles)
}

// ── Handlers ──────────────────────────────────────────────────────

// ListUsers godoc
// GET /users?search=สมชาย&is_active=true&page=1&limit=20
func (h *Handler) ListUsers(c *gin.Context) {
	var q ListUsersQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.service.ListUsers(c.Request.Context(), q)
	if err != nil {
		h.internalError(c, "ListUsers", err)
		return
	}
	response.OK(c, result)
}

// CreateUser godoc
// POST /users
func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	createdByID := getUserID(c)
	result, err := h.service.CreateUser(c.Request.Context(), req, createdByID)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateEmail):
			response.Conflict(c, err.Error())
		case errors.Is(err, ErrRoleNotFound):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "CreateUser", err)
		}
		return
	}
	response.Created(c, result)
}

// GetUser godoc
// GET /users/:id
func (h *Handler) GetUser(c *gin.Context) {
	result, err := h.service.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		h.internalError(c, "GetUser", err)
		return
	}
	response.OK(c, result)
}

// UpdateUser godoc
// PATCH /users/:id
func (h *Handler) UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	requesterID := getUserID(c)
	result, err := h.service.UpdateUser(c.Request.Context(), c.Param("id"), requesterID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
		case errors.Is(err, ErrDuplicateEmail):
			response.Conflict(c, err.Error())
		case errors.Is(err, ErrCannotDeactivateSelf):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "UpdateUser", err)
		}
		return
	}
	response.OK(c, result)
}

// DeleteUser godoc
// DELETE /users/:id
func (h *Handler) DeleteUser(c *gin.Context) {
	requesterID := getUserID(c)
	if err := h.service.DeleteUser(c.Request.Context(), c.Param("id"), requesterID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
		case errors.Is(err, ErrCannotDeleteSelf):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "DeleteUser", err)
		}
		return
	}
	response.OK(c, gin.H{"message": "user deleted"})
}

// AssignRole godoc
// POST /users/:id/roles
// Body: {"role_id": 2}
func (h *Handler) AssignRole(c *gin.Context) {
	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	assignedByID := getUserID(c)
	if err := h.service.AssignRole(c.Request.Context(), c.Param("id"), req.RoleID, assignedByID); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
		case errors.Is(err, ErrRoleNotFound):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "AssignRole", err)
		}
		return
	}
	response.OK(c, gin.H{"message": "role assigned"})
}

// RemoveRole godoc
// DELETE /users/:id/roles/:roleId
func (h *Handler) RemoveRole(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("roleId"), 10, 32)
	if err != nil || roleID <= 0 {
		response.BadRequest(c, "roleId must be a positive integer")
		return
	}
	if err := h.service.RemoveRole(c.Request.Context(), c.Param("id"), int32(roleID)); err != nil { //nolint:gosec
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		h.internalError(c, "RemoveRole", err)
		return
	}
	response.OK(c, gin.H{"message": "role removed"})
}

// ResetPassword godoc
// POST /users/:id/reset-password
// Body: {"new_password": "..."}
func (h *Handler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.service.ResetPassword(c.Request.Context(), c.Param("id"), req); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		h.internalError(c, "ResetPassword", err)
		return
	}
	response.OK(c, gin.H{"message": "password reset"})
}

// ChangePassword godoc
// POST /users/me/password
// Body: {"current_password": "...", "new_password": "..."}
func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := getUserID(c)
	if err := h.service.ChangePassword(c.Request.Context(), userID, req); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
		case errors.Is(err, ErrInvalidPassword):
			response.UnprocessableEntity(c, err.Error())
		default:
			h.internalError(c, "ChangePassword", err)
		}
		return
	}
	response.OK(c, gin.H{"message": "password changed"})
}

// ListRoles godoc
// GET /roles
func (h *Handler) ListRoles(c *gin.Context) {
	result, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		h.internalError(c, "ListRoles", err)
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
	slog.ErrorContext(c.Request.Context(), "user: unexpected error",
		"op", op,
		"method", c.Request.Method,
		"path", c.FullPath(),
		"target_id", c.Param("id"),
		"user_id", getUserID(c),
		"error", err,
	)
	response.InternalError(c)
}
