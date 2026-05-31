package dashboard

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

// Service รัน queries ทั้งหมดแบบ parallel-friendly
// แต่ละ query อิสระจากกัน — ไม่มี TX ที่ต้องการ
type Service struct {
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{queries: db.New(pool)}
}

// GetDashboard ดึงข้อมูลทุก section พร้อมกัน (5 queries ต่อ request)
func (s *Service) GetDashboard(ctx context.Context) (*DashboardResponse, error) {
	// ── 1. Overview KPIs ──────────────────────────────────────────
	overview, err := s.queries.GetOrdersOverview(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetOrdersOverview: %w", err)
	}

	cod, err := s.queries.GetCODPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetCODPending: %w", err)
	}

	totalSKUs, err := s.queries.CountActiveSKUs(ctx)
	if err != nil {
		return nil, fmt.Errorf("CountActiveSKUs: %w", err)
	}

	// ── 2. Order Status Breakdown ─────────────────────────────────
	statusRows, err := s.queries.GetOrderStatusBreakdown(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetOrderStatusBreakdown: %w", err)
	}

	// ── 3. Fulfillment Queue ──────────────────────────────────────
	queue, err := s.queries.GetFulfillmentQueueSizes(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetFulfillmentQueueSizes: %w", err)
	}

	// ── 4. Stock Alerts ───────────────────────────────────────────
	alertRows, err := s.queries.GetStockAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetStockAlerts: %w", err)
	}

	// ── 5. Recent Orders ──────────────────────────────────────────
	recentRows, err := s.queries.GetRecentOrders(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetRecentOrders: %w", err)
	}

	// ── Assemble Response ─────────────────────────────────────────

	breakdown := make([]OrderStatusItem, len(statusRows))
	for i, r := range statusRows {
		breakdown[i] = OrderStatusItem{
			Status: r.Status,
			Count:  r.Count,
		}
	}

	alerts := make([]StockAlertItem, len(alertRows))
	for i, r := range alertRows {
		qtyAvailable := int32(0)
		if r.QtyAvailable.Valid {
			qtyAvailable = r.QtyAvailable.Int32
		}
		alerts[i] = StockAlertItem{
			SKUID:             pgutil.UUIDString(r.SkuID),
			SKUCode:           r.SkuCode,
			ProductName:       r.ProductName,
			QtyAvailable:      qtyAvailable,
			LowStockThreshold: r.LowStockThreshold,
			AlertType:         r.AlertType,
		}
	}

	recent := make([]RecentOrderItem, len(recentRows))
	for i, r := range recentRows {
		item := RecentOrderItem{
			ID:          pgutil.UUIDString(r.ID),
			OrderNumber: r.OrderNumber,
			Channel:     r.Channel,
			Status:      r.Status,
			Total:       pgutil.NumericToFloat(r.Total),
			CreatedAt:   r.CreatedAt.Time,
		}
		if r.CustomerName.Valid {
			item.CustomerName = &r.CustomerName.String
		}
		recent[i] = item
	}

	return &DashboardResponse{
		Overview: OverviewResponse{
			OrdersToday:      overview.OrdersToday,
			OrdersThisWeek:   overview.OrdersThisWeek,
			OrdersThisMonth:  overview.OrdersThisMonth,
			RevenueToday:     pgutil.NumericToFloat(overview.RevenueToday),
			RevenueThisWeek:  pgutil.NumericToFloat(overview.RevenueThisWeek),
			RevenueThisMonth: pgutil.NumericToFloat(overview.RevenueThisMonth),
			CODPendingCount:  cod.CodOrderCount,
			CODPendingAmount: pgutil.NumericToFloat(cod.CodPendingAmount),
			TotalActiveSKUs:  totalSKUs,
		},
		OrderBreakdown: breakdown,
		FulfillmentQueue: FulfillmentQueueResponse{
			AwaitingPick: queue.AwaitingPick,
			InPicking:    queue.InPicking,
			InPacking:    queue.InPacking,
			ReadyToShip:  queue.ReadyToShip,
		},
		StockAlerts:  alerts,
		RecentOrders: recent,
	}, nil
}

// GetAlerts ดึงเฉพาะ stock alerts — ใช้สำหรับ frontend polling
func (s *Service) GetAlerts(ctx context.Context) (*AlertsResponse, error) {
	rows, err := s.queries.GetStockAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetStockAlerts: %w", err)
	}

	alerts := make([]StockAlertItem, len(rows))
	for i, r := range rows {
		qtyAvailable := int32(0)
		if r.QtyAvailable.Valid {
			qtyAvailable = r.QtyAvailable.Int32
		}
		alerts[i] = StockAlertItem{
			SKUID:             pgutil.UUIDString(r.SkuID),
			SKUCode:           r.SkuCode,
			ProductName:       r.ProductName,
			QtyAvailable:      qtyAvailable,
			LowStockThreshold: r.LowStockThreshold,
			AlertType:         r.AlertType,
		}
	}

	return &AlertsResponse{
		StockAlerts: alerts,
		Count:       len(alerts),
	}, nil
}
