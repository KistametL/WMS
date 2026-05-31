package user

import (
	"errors"
	"time"
)

// ── Sentinel Errors ───────────────────────────────────────────────

var (
	ErrNotFound             = errors.New("user not found")
	ErrRoleNotFound         = errors.New("role not found")
	ErrDuplicateEmail       = errors.New("email already exists")
	ErrInvalidPassword      = errors.New("current password is incorrect")
	ErrCannotDeleteSelf     = errors.New("cannot delete your own account")
	ErrCannotDeactivateSelf = errors.New("cannot deactivate your own account")
)

// ── Responses ─────────────────────────────────────────────────────

// UserResponse — ใช้ใน list view
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserDetailResponse — ใช้ใน detail view (รวม roles + permissions)
type UserDetailResponse struct {
	UserResponse
	Roles       []RoleResponse `json:"roles"`
	Permissions []string       `json:"permissions"`
}

// RoleResponse — role ที่ user มี
type RoleResponse struct {
	ID          int32   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// ── Requests ──────────────────────────────────────────────────────

type ListUsersQuery struct {
	Search   string `form:"search"`
	IsActive *bool  `form:"is_active"`
	Page     int    `form:"page"`
	Limit    int    `form:"limit"`
}

type CreateUserRequest struct {
	Email     string  `json:"email"      binding:"required,email"`
	Password  string  `json:"password"   binding:"required,min=8"`
	FirstName string  `json:"first_name" binding:"required,min=1,max=100"`
	LastName  string  `json:"last_name"  binding:"required,min=1,max=100"`
	RoleIDs   []int32 `json:"role_ids"`
}

type UpdateUserRequest struct {
	Email     *string `json:"email"      binding:"omitempty,email"`
	FirstName *string `json:"first_name" binding:"omitempty,min=1,max=100"`
	LastName  *string `json:"last_name"  binding:"omitempty,min=1,max=100"`
	IsActive  *bool   `json:"is_active"`
}

type AssignRoleRequest struct {
	RoleID int32 `json:"role_id" binding:"required"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password"     binding:"required,min=8"`
}

// ── Pagination ────────────────────────────────────────────────────

type ListResponse[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}
