package order

import (
	"encoding/json"
	"time"
)

// ── Order Status & Channel ─────────────────────────────────────────

const (
	StatusPending     = "pending"
	StatusConfirmed   = "confirmed"
	StatusPicking     = "picking"
	StatusPacking     = "packing"
	StatusReadyToShip = "ready_to_ship"
	StatusShipped     = "shipped"
	StatusDelivered   = "delivered"
	StatusCompleted   = "completed"
	StatusCancelled   = "cancelled"
)

// validTransitions กำหนด state machine ของ order
// key = current status, value = statuses ที่ไปได้
var validTransitions = map[string][]string{
	StatusPending:     {StatusConfirmed, StatusCancelled},
	StatusConfirmed:   {StatusPicking, StatusCancelled},
	StatusPicking:     {StatusPacking, StatusCancelled},
	StatusPacking:     {StatusReadyToShip, StatusCancelled},
	StatusReadyToShip: {StatusShipped},
	StatusShipped:     {StatusDelivered},
	StatusDelivered:   {StatusCompleted},
	StatusCompleted:   {}, // terminal
	StatusCancelled:   {}, // terminal
}

// reservedStatuses คือ statuses ที่ stock ถูก reserve ไว้แล้ว
// ถ้า cancel จาก status เหล่านี้ ต้อง unreserve stock กลับ
var reservedStatuses = map[string]bool{
	StatusConfirmed:   true,
	StatusPicking:     true,
	StatusPacking:     true,
	StatusReadyToShip: true,
}

func isValidTransition(from, to string) bool {
	allowed := validTransitions[from]
	for _, a := range allowed {
		if a == to {
			return true
		}
	}
	return false
}

// ── Responses ─────────────────────────────────────────────────────

type OrderResponse struct {
	ID              string          `json:"id"`
	OrderNumber     string          `json:"order_number"`
	Channel         string          `json:"channel"`
	ChannelOrderID  *string         `json:"channel_order_id,omitempty"`
	Status          string          `json:"status"`
	CustomerName    *string         `json:"customer_name,omitempty"`
	CustomerPhone   *string         `json:"customer_phone,omitempty"`
	ShippingAddress json.RawMessage `json:"shipping_address,omitempty"`
	ShippingMethod  *string         `json:"shipping_method,omitempty"`
	ShippingCost    float64         `json:"shipping_cost"`
	IsCOD           bool            `json:"is_cod"`
	CODAmount       float64         `json:"cod_amount"`
	Subtotal        float64         `json:"subtotal"`
	DiscountTotal   float64         `json:"discount_total"`
	Total           float64         `json:"total"`
	Note            *string         `json:"note,omitempty"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type OrderDetailResponse struct {
	OrderResponse
	ConfirmedAt  *time.Time              `json:"confirmed_at,omitempty"`
	PackedAt     *time.Time              `json:"packed_at,omitempty"`
	ShippedAt    *time.Time              `json:"shipped_at,omitempty"`
	DeliveredAt  *time.Time              `json:"delivered_at,omitempty"`
	CancelledAt  *time.Time              `json:"cancelled_at,omitempty"`
	CancelReason *string                 `json:"cancel_reason,omitempty"`
	Items        []OrderItemResponse     `json:"items"`
	History      []StatusHistoryResponse `json:"history"`
}

type OrderItemResponse struct {
	ID             int64   `json:"id"`
	SKUID          string  `json:"sku_id"`
	SKUCode        string  `json:"sku_code"`
	Name           string  `json:"name"`
	Quantity       int32   `json:"quantity"`
	UnitPrice      float64 `json:"unit_price"`
	DiscountAmount float64 `json:"discount_amount"`
	TotalPrice     float64 `json:"total_price"`
}

type StatusHistoryResponse struct {
	ID         int64     `json:"id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	Note       *string   `json:"note,omitempty"`
	ChangedBy  *string   `json:"changed_by,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
}

// ── Requests ──────────────────────────────────────────────────────

type CreateOrderRequest struct {
	Channel         string                   `json:"channel"        binding:"required,oneof=shopee lazada tiktok pos manual"`
	ChannelOrderID  *string                  `json:"channel_order_id"`
	CustomerName    *string                  `json:"customer_name"`
	CustomerPhone   *string                  `json:"customer_phone"`
	ShippingAddress json.RawMessage          `json:"shipping_address"`
	ShippingMethod  *string                  `json:"shipping_method"`
	ShippingCost    float64                  `json:"shipping_cost"   binding:"min=0"`
	IsCOD           bool                     `json:"is_cod"`
	CODAmount       float64                  `json:"cod_amount"      binding:"min=0"`
	DiscountTotal   float64                  `json:"discount_total"  binding:"min=0"`
	Note            *string                  `json:"note"`
	Items           []CreateOrderItemRequest `json:"items"           binding:"required,min=1,dive"`
}

type CreateOrderItemRequest struct {
	SKUID          string  `json:"sku_id"          binding:"required,uuid"`
	Quantity       int32   `json:"quantity"        binding:"required,min=1"`
	UnitPrice      float64 `json:"unit_price"      binding:"min=0"`
	DiscountAmount float64 `json:"discount_amount" binding:"min=0"`
}

type UpdateOrderRequest struct {
	CustomerName    *string         `json:"customer_name"`
	CustomerPhone   *string         `json:"customer_phone"`
	ShippingAddress json.RawMessage `json:"shipping_address"`
	ShippingMethod  *string         `json:"shipping_method"`
	ShippingCost    *float64        `json:"shipping_cost"  binding:"omitempty,min=0"`
	DiscountTotal   *float64        `json:"discount_total" binding:"omitempty,min=0"`
	Note            *string         `json:"note"`
}

// UpdateStatusRequest — body สำหรับ POST /orders/:id/status
// status ต้องเป็นหนึ่งใน valid statuses เท่านั้น (oneof)
// เพื่อ cancel ควรใช้ POST /orders/:id/cancel แทน (มี reason field)
type UpdateStatusRequest struct {
	Status string  `json:"status" binding:"required,oneof=confirmed picking packing ready_to_ship shipped delivered completed cancelled"`
	Note   *string `json:"note"`
}

type CancelOrderRequest struct {
	Reason *string `json:"reason"`
}

// ListOrdersQuery — query parameters จาก URL
type ListOrdersQuery struct {
	Status  string `form:"status"`
	Channel string `form:"channel"`
	IsCOD   *bool  `form:"is_cod"`
	Search  string `form:"search"`
	Page    int    `form:"page"`
	Limit   int    `form:"limit"`
}

// ListResponse — generic list response
type ListResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}
