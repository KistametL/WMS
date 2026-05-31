package fulfillment

import (
	"errors"
	"time"
)

// ── Sentinel Errors ───────────────────────────────────────────────

var (
	ErrNotFound          = errors.New("order not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrScanMismatch      = errors.New("scanned items do not match order items")
	ErrAlreadyShipped    = errors.New("shipment record already exists for this order")
	ErrInsufficientStock = errors.New("insufficient stock")
)

// fulfillmentStatuses — statuses ที่ fulfillment workflow ดูแล
// ใช้ validate query param ใน list endpoint
var fulfillmentStatuses = map[string]bool{
	"confirmed":     true,
	"picking":       true,
	"packing":       true,
	"ready_to_ship": true,
}

// ── Responses ─────────────────────────────────────────────────────

// FulfillmentOrderResponse — ใช้ใน queue views (list endpoints)
type FulfillmentOrderResponse struct {
	ID            string    `json:"id"`
	OrderNumber   string    `json:"order_number"`
	Channel       string    `json:"channel"`
	Status        string    `json:"status"`
	CustomerName  *string   `json:"customer_name,omitempty"`
	CustomerPhone *string   `json:"customer_phone,omitempty"`
	Total         float64   `json:"total"`
	IsCOD         bool      `json:"is_cod"`
	CODAmount     float64   `json:"cod_amount"`
	Note          *string   `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ShipmentResponse — ใช้ตอบกลับหลัง ship
type ShipmentResponse struct {
	ID             string    `json:"id"`
	OrderID        string    `json:"order_id"`
	Courier        string    `json:"courier"`
	TrackingNumber string    `json:"tracking_number"`
	LabelURL       *string   `json:"label_url,omitempty"`
	ShippedBy      *string   `json:"shipped_by,omitempty"`
	ShippedAt      time.Time `json:"shipped_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// TransitionResponse — ตอบกลับหลัง status transition (start-pick / confirm-pick / confirm-pack)
type TransitionResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// ── Requests ──────────────────────────────────────────────────────

// QueueQuery — query params สำหรับ GET /fulfillment/orders
type QueueQuery struct {
	// status ต้องเป็น fulfillment status เท่านั้น (ตรวจใน handler)
	Status string `form:"status"`
	Page   int    `form:"page"`
	Limit  int    `form:"limit"`
}

// ShipRequest — body สำหรับ POST /fulfillment/orders/:id/ship
type ShipRequest struct {
	Courier        string  `json:"courier"         binding:"required,oneof=kerry flash jnt spx thaipost manual"`
	TrackingNumber string  `json:"tracking_number" binding:"required,min=1"`
	LabelURL       *string `json:"label_url"`
}

// ConfirmPickRequest — body สำหรับ POST /fulfillment/orders/:id/confirm-pick
// scanned_items: รายการที่ staff scan จริงๆ ใน warehouse
// service จะ validate ว่าตรงกับ order items ครบถ้วน
type ConfirmPickRequest struct {
	ScannedItems []ScannedItem `json:"scanned_items" binding:"required,min=1,dive"`
}

// ScannedItem — sku_code + quantity ที่ staff scan
type ScannedItem struct {
	SKUCode  string `json:"sku_code"  binding:"required"`
	Quantity int32  `json:"quantity"  binding:"required,min=1"`
}

// ── Pagination ────────────────────────────────────────────────────

// ListResponse — generic pagination wrapper
type ListResponse[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"total_pages"`
}
