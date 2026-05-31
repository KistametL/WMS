package product

import (
	"encoding/json"
	"time"
)

// ── Category ──────────────────────────────────────────────────────

type CategoryResponse struct {
	ID          int32     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	ParentID    *int32    `json:"parent_id,omitempty"`
	IsActive    bool      `json:"is_active"`
	UpdatedBy   *string   `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name        string  `json:"name"        binding:"required,min=1,max=100"`
	Description *string `json:"description"`
	ParentID    *int32  `json:"parent_id"`
}

type UpdateCategoryRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=1,max=100"`
	Description *string `json:"description"`
	ParentID    *int32  `json:"parent_id"`
	IsActive    *bool   `json:"is_active"`
}

// ── Product ───────────────────────────────────────────────────────

type ProductResponse struct {
	ID          string    `json:"id"`
	CategoryID  *int32    `json:"category_id,omitempty"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	UpdatedBy   *string   `json:"updated_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProductDetailResponse — ใช้ตอน GET /products/:id (รวม SKUs ด้วย)
type ProductDetailResponse struct {
	ProductResponse
	SKUs []SKUResponse `json:"skus"`
}

type CreateProductRequest struct {
	CategoryID  *int32  `json:"category_id"`
	Name        string  `json:"name"        binding:"required,min=1,max=255"`
	Description *string `json:"description"`
}

type UpdateProductRequest struct {
	CategoryID  *int32  `json:"category_id"`
	Name        *string `json:"name"        binding:"omitempty,min=1,max=255"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// ListProductsQuery — query parameters จาก URL
// GET /products?page=1&limit=20&search=ถุงเท้า&category_id=1&is_active=true
type ListProductsQuery struct {
	Page       int    `form:"page"`
	Limit      int    `form:"limit"`
	Search     string `form:"search"`
	CategoryID *int32 `form:"category_id"`
	IsActive   *bool  `form:"is_active"`
}

// ListResponse — response สำหรับ list endpoint ทุกตัว
type ListResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

// ── SKU ───────────────────────────────────────────────────────────

type SKUResponse struct {
	ID             string          `json:"id"`
	ProductID      string          `json:"product_id"`
	SKUCode        string          `json:"sku_code"`
	Name           string          `json:"name"`
	CostPrice      float64         `json:"cost_price"`
	SellingPrice   float64         `json:"selling_price"`
	CompareAtPrice *float64        `json:"compare_at_price,omitempty"`
	WeightGrams    *int32          `json:"weight_grams,omitempty"`
	Attributes     json.RawMessage `json:"attributes,omitempty"`
	IsActive       bool            `json:"is_active"`
	UpdatedBy      *string         `json:"updated_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type CreateSKURequest struct {
	SKUCode        string          `json:"sku_code"       binding:"required,min=1,max=100"`
	Name           string          `json:"name"           binding:"required,min=1,max=255"`
	CostPrice      float64         `json:"cost_price"     binding:"min=0"`
	SellingPrice   float64         `json:"selling_price"  binding:"min=0"`
	CompareAtPrice *float64        `json:"compare_at_price"`
	WeightGrams    *int32          `json:"weight_grams"`
	Attributes     json.RawMessage `json:"attributes"`
}

type UpdateSKURequest struct {
	SKUCode        *string         `json:"sku_code"        binding:"omitempty,min=1,max=100"`
	Name           *string         `json:"name"            binding:"omitempty,min=1,max=255"`
	CostPrice      *float64        `json:"cost_price"      binding:"omitempty,min=0"`
	SellingPrice   *float64        `json:"selling_price"   binding:"omitempty,min=0"`
	CompareAtPrice *float64        `json:"compare_at_price"`
	WeightGrams    *int32          `json:"weight_grams"`
	Attributes     json.RawMessage `json:"attributes"`
	IsActive       *bool           `json:"is_active"`
}

// ── SKU Label ─────────────────────────────────────────────────────

// SKULabelResponse — ใช้สำหรับ GET /skus/:id/label
// backend return ข้อมูลดิบ → frontend render barcode + print
type SKULabelResponse struct {
	SKUID        string  `json:"sku_id"`
	SKUCode      string  `json:"sku_code"`
	Name         string  `json:"name"`
	BarcodeValue string  `json:"barcode_value"` // ค่าที่ encode เป็น barcode (= sku_code)
	BarcodeType  string  `json:"barcode_type"`  // "CODE128"
	SellingPrice float64 `json:"selling_price"`
	CostPrice    float64 `json:"cost_price"`
}
