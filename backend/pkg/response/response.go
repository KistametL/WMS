package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard JSON envelope for all API responses.
// `any` is the idiomatic alias for interface{} since Go 1.18
// (Google Go Style Guide §Alias-Types, 2024).
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Success: true, Data: data})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{Success: false, Error: msg})
}

func Unauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, Response{Success: false, Error: msg})
}

func Forbidden(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, Response{Success: false, Error: msg})
}

func NotFound(c *gin.Context, msg string) {
	c.JSON(http.StatusNotFound, Response{Success: false, Error: msg})
}

// Conflict writes a 409 Conflict response.
// Use when a request cannot be completed due to a conflict with the current
// state of the resource — e.g. a unique constraint violation (RFC 9110 §15.5.10).
func Conflict(c *gin.Context, msg string) {
	c.JSON(http.StatusConflict, Response{Success: false, Error: msg})
}

// UnprocessableEntity writes a 422 Unprocessable Content response.
// Use when the request is syntactically valid but semantically incorrect —
// e.g. insufficient stock, or a business rule violation (RFC 9110 §15.5.23).
func UnprocessableEntity(c *gin.Context, msg string) {
	c.JSON(http.StatusUnprocessableEntity, Response{Success: false, Error: msg})
}

func InternalError(c *gin.Context) {
	c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "internal server error"})
}
