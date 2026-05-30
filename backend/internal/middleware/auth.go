package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/KistametL/WMS/backend/internal/config"
	"github.com/KistametL/WMS/backend/pkg/response"
)

// Context keys — ใช้ prefix "wms." เพื่อป้องกัน collision กับ library อื่น
const (
	CtxUserID      = "wms.user_id"
	CtxEmail       = "wms.email"
	CtxRoles       = "wms.roles"
	CtxPermissions = "wms.permissions"
)

// RequireAuth validates the JWT access token in the Authorization header.
//
// Flow:
//  1. Extract "Bearer <token>" from Authorization header
//  2. Verify signature using HS256 + JWT_SECRET
//  3. Check token has not expired (jwt library handles this)
//  4. Store claims in gin.Context for downstream handlers
//
// ถ้า token ไม่ valid → 401 Unauthorized ทันที (c.Abort)
func RequireAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "authorization header is required")
			c.Abort()
			return
		}

		// ต้องเป็นรูปแบบ "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "authorization header format must be: Bearer <token>")
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// Parse และ validate token (signature + expiry)
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			// ป้องกัน algorithm confusion attack — ตรวจสอบว่าเป็น HS256 เท่านั้น
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JWT.Secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || !token.Valid {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Unauthorized(c, "invalid token claims")
			c.Abort()
			return
		}

		// Store claims ใน context
		c.Set(CtxUserID, claims["user_id"])
		c.Set(CtxEmail, claims["email"])
		c.Set(CtxRoles, claims["roles"])
		c.Set(CtxPermissions, claims["permissions"])

		c.Next()
	}
}

// RequirePermission checks that the authenticated user has AT LEAST ONE
// of the specified permissions.
//
// ตัวอย่างการใช้งาน:
//
//	products := api.Group("/products")
//	products.Use(middleware.RequireAuth(cfg))
//	products.GET("", middleware.RequirePermission("products.read"), handler.List)
//	products.POST("", middleware.RequirePermission("products.write"), handler.Create)
//
// ถ้า user ไม่มี permission ที่กำหนด → 403 Forbidden
func RequirePermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userPerms := getStringSlice(c, CtxPermissions)

		for _, required := range permissions {
			for _, has := range userPerms {
				if has == required {
					c.Next()
					return
				}
			}
		}

		response.Forbidden(c, "you do not have permission to perform this action")
		c.Abort()
	}
}

// RequireRole checks that the authenticated user has AT LEAST ONE of the
// specified roles.
//
// ตัวอย่างการใช้งาน:
//
//	admin := api.Group("/admin")
//	admin.Use(middleware.RequireAuth(cfg), middleware.RequireRole("super_admin", "admin"))
//
// ถ้า user ไม่มี role ที่กำหนด → 403 Forbidden
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := getStringSlice(c, CtxRoles)

		for _, required := range roles {
			for _, has := range userRoles {
				if has == required {
					c.Next()
					return
				}
			}
		}

		response.Forbidden(c, "insufficient role to access this resource")
		c.Abort()
	}
}

// ── Helper functions ──────────────────────────────────────────────────────────
// ใช้ใน handler เพื่อดึงข้อมูล user ออกจาก context โดยไม่ต้อง type-cast เอง

// GetUserID returns the authenticated user's ID from the context.
func GetUserID(c *gin.Context) string {
	return getString(c, CtxUserID)
}

// GetEmail returns the authenticated user's email from the context.
func GetEmail(c *gin.Context) string {
	return getString(c, CtxEmail)
}

// GetRoles returns the authenticated user's roles from the context.
func GetRoles(c *gin.Context) []string {
	return getStringSlice(c, CtxRoles)
}

// GetPermissions returns the authenticated user's permissions from the context.
func GetPermissions(c *gin.Context) []string {
	return getStringSlice(c, CtxPermissions)
}

// HasPermission checks if the current user has a specific permission.
// Useful for conditional logic inside handlers.
func HasPermission(c *gin.Context, permission string) bool {
	for _, p := range GetPermissions(c) {
		if p == permission {
			return true
		}
	}
	return false
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func getString(c *gin.Context, key string) string {
	val, exists := c.Get(key)
	if !exists {
		return ""
	}
	str, _ := val.(string)
	return str
}

// JWT stores arrays as []any (JSON decoded), not []string
// ต้องแปลงก่อนใช้งาน
func getStringSlice(c *gin.Context, key string) []string {
	val, exists := c.Get(key)
	if !exists {
		return nil
	}

	// ถ้า set มาเป็น []string ตรงๆ (เช่น จาก unit test)
	if ss, ok := val.([]string); ok {
		return ss
	}

	// jwt.MapClaims decode array เป็น []any
	if raw, ok := val.([]any); ok {
		result := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}

	return nil
}
