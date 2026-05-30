package inventory

import "time"

// ── Stock Level ───────────────────────────────────────────────────

// StockLevelResponse — response สำหรับ GET /inventory/stock และ GET /inventory/stock/:sku_id
type StockLevelResponse struct {
	ID                int64     `json:"id"`
	SKUID             string    `json:"sku_id"`
	SKUCode           string    `json:"sku_code"`
	SKUName           string    `json:"sku_name"`
	QtyOnHand         int32     `json:"qty_on_hand"`
	QtyReserved       int32     `json:"qty_reserved"`
	QtyAvailable      int32     `json:"qty_available"`
	LowStockThreshold int32     `json:"low_stock_threshold"`
	IsLowStock        bool      `json:"is_low_stock"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// UpdateThresholdRequest — body สำหรับ PUT /inventory/stock/:sku_id/threshold
type UpdateThresholdRequest struct {
	LowStockThreshold int32 `json:"low_stock_threshold" binding:"min=0"`
}

// ListStockQuery — query parameters จาก URL
// GET /inventory/stock?page=1&limit=20&low_stock_only=true
type ListStockQuery struct {
	Page         int   `form:"page"`
	Limit        int   `form:"limit"`
	LowStockOnly *bool `form:"low_stock_only"`
}

// ── Stock Movements ───────────────────────────────────────────────

// MovementType ที่รองรับผ่าน manual API
// receive  — รับสินค้าเข้า (qty_on_hand += qty)
// adjust   — ปรับยอดด้วยมือ ใส่จำนวนจริงที่นับได้ (qty_on_hand = qty)
// damage   — สินค้าเสียหาย/สูญหาย (qty_on_hand -= qty)
// return   — รับคืนจากลูกค้า (qty_on_hand += qty)
//
// movement_type อื่น (reserve/unreserve/fulfill/transfer)
// ถูก trigger โดย system เท่านั้น ไม่ expose ผ่าน manual API
const (
	MovementReceive = "receive"
	MovementAdjust  = "adjust"
	MovementDamage  = "damage"
	MovementReturn  = "return"
)

// CreateMovementRequest — body สำหรับ POST /inventory/movements
type CreateMovementRequest struct {
	SKUID         string  `json:"sku_id"        binding:"required,uuid"`
	MovementType  string  `json:"movement_type" binding:"required,oneof=receive adjust damage return"`
	Qty           int32   `json:"qty"           binding:"required,min=1"`
	Note          *string `json:"note"`
	ReferenceType *string `json:"reference_type"`
	ReferenceID   *string `json:"reference_id"`
}

// MovementResponse — response สำหรับ movement endpoints
type MovementResponse struct {
	ID            int64     `json:"id"`
	SKUID         string    `json:"sku_id"`
	MovementType  string    `json:"movement_type"`
	QtyChange     int32     `json:"qty_change"`
	QtyBefore     int32     `json:"qty_before"`
	QtyAfter      int32     `json:"qty_after"`
	ReferenceType *string   `json:"reference_type,omitempty"`
	ReferenceID   *string   `json:"reference_id,omitempty"`
	Note          *string   `json:"note,omitempty"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListMovementsQuery — query parameters จาก URL
// GET /inventory/movements?sku_id=...&movement_type=receive&page=1&limit=20
type ListMovementsQuery struct {
	SKUID        string `form:"sku_id"`
	MovementType string `form:"movement_type"`
	Page         int    `form:"page"`
	Limit        int    `form:"limit"`
}

// ListResponse — generic list response (เหมือนกับ product module)
type ListResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}
