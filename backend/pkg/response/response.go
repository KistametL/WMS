package response

import "github.com/gin-gonic/gin"

// Response is the standard JSON envelope for all API responses.
// `any` is the idiomatic alias for interface{} since Go 1.18
// (Google Go Style Guide §Alias-Types, 2024).
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, Response{Success: true, Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(201, Response{Success: true, Data: data})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(400, Response{Success: false, Error: msg})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(401, Response{Success: false, Error: msg})
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(403, Response{Success: false, Error: msg})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(404, Response{Success: false, Error: msg})
}

// Conflict writes a 409 Conflict response.
// Use when a request cannot be completed due to a conflict with the current
// state of the resource — e.g. a unique constraint violation (RFC 9110 §15.5.10).
func Conflict(c *gin.Context, msg string) {
	c.JSON(409, Response{Success: false, Error: msg})
}

func InternalError(c *gin.Context) {
	c.JSON(500, Response{Success: false, Error: "internal server error"})
}
