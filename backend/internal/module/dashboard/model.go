package dashboard

import "time"

// ── Overview KPIs ─────────────────────────────────────────────────

// OverviewResponse — ตัวเลขสรุปภาพรวมของระบบ
type OverviewResponse struct {
	OrdersToday      int64   `json:"orders_today"`
	OrdersThisWeek   int64   `json:"orders_this_week"`
	OrdersThisMonth  int64   `json:"orders_this_month"`
	RevenueToday     float64 `json:"revenue_today"`
	RevenueThisWeek  float64 `json:"revenue_this_week"`
	RevenueThisMonth float64 `json:"revenue_this_month"`
	CODPendingCount  int64   `json:"cod_pending_count"`
	CODPendingAmount float64 `json:"cod_pending_amount"`
	TotalActiveSKUs  int64   `json:"total_active_skus"`
}

// ── Order Status Breakdown ─────────────────────────────────────────

// OrderStatusItem — จำนวนออเดอร์ต่อ status
type OrderStatusItem struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// ── Fulfillment Queue ──────────────────────────────────────────────

// FulfillmentQueueResponse — จำนวนออเดอร์ในแต่ละขั้นตอน fulfillment
type FulfillmentQueueResponse struct {
	AwaitingPick int64 `json:"awaiting_pick"`
	InPicking    int64 `json:"in_picking"`
	InPacking    int64 `json:"in_packing"`
	ReadyToShip  int64 `json:"ready_to_ship"`
}

// ── Stock Alerts ───────────────────────────────────────────────────

// StockAlertItem — SKU ที่ stock ต่ำหรือหมด
type StockAlertItem struct {
	SKUID             string `json:"sku_id"`
	SKUCode           string `json:"sku_code"`
	ProductName       string `json:"product_name"`
	QtyAvailable      int32  `json:"qty_available"`
	LowStockThreshold int32  `json:"low_stock_threshold"`
	AlertType         string `json:"alert_type"` // "low_stock" | "out_of_stock"
}

// ── Recent Orders ──────────────────────────────────────────────────

// RecentOrderItem — ออเดอร์ล่าสุด (summary เท่านั้น)
type RecentOrderItem struct {
	ID           string    `json:"id"`
	OrderNumber  string    `json:"order_number"`
	Channel      string    `json:"channel"`
	Status       string    `json:"status"`
	CustomerName *string   `json:"customer_name,omitempty"`
	Total        float64   `json:"total"`
	CreatedAt    time.Time `json:"created_at"`
}

// ── Full Dashboard Response ────────────────────────────────────────

// DashboardResponse — response หลักของ GET /dashboard
type DashboardResponse struct {
	Overview         OverviewResponse         `json:"overview"`
	OrderBreakdown   []OrderStatusItem        `json:"order_status_breakdown"`
	FulfillmentQueue FulfillmentQueueResponse `json:"fulfillment_queue"`
	StockAlerts      []StockAlertItem         `json:"stock_alerts"`
	RecentOrders     []RecentOrderItem        `json:"recent_orders"`
}

// AlertsResponse — response ของ GET /dashboard/alerts (สำหรับ polling)
type AlertsResponse struct {
	StockAlerts []StockAlertItem `json:"stock_alerts"`
	Count       int              `json:"count"`
}
