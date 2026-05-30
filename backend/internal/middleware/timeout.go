package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestTimeout wraps each request's context with a deadline so that
// long-running DB queries / pool waits are cancelled automatically.
//
// ขาดไป fix: ป้องกัน request ค้างนานเกินไปกรณี DB ช้า / connection pool หมด
// pgx / pgxpool respect context cancellation — query จะถูก cancel ทันทีที่ deadline ถึง
//
// Usage in main.go:
//
//	protected.Use(middleware.RequestTimeout(30 * time.Second))
func RequestTimeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
