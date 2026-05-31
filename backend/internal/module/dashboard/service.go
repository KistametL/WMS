package dashboard

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
	"github.com/KistametL/WMS/backend/internal/pgutil"
)

// Service รัน dashboard queries แบบ parallel ด้วย errgroup
// แต่ละ query อิสระจากกัน — ไม่มี TX ที่ต้องการ
type Service struct {
	queries *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{queries: db.New(pool)}
}

// GetDashboard ดึงข้อมูลทุก section พร้อมกัน (7 queries, parallel)
// response time ≈ query ที่ช้าที่สุด แทนที่จะเป็น sum ของทุก query
func (s *Service) GetDashboard(ctx context.Context) (*DashboardResponse, error) {
	// ── Pre-declare result variables ──────────────────────────────
	// แต่ละ goroutine เขียนตัวแปรของตัวเองเท่านั้น — ไม่มี data race
	var (
		overview   db.GetOrdersOverviewRow
		cod        db.GetCODPendingRow
		totalSKUs  int64
		statusRows []db.GetOrderStatusBreakdownRow
		queue      db.GetFulfillmentQueueSizesRow
		alertRows  []db.GetStockAlertsRow
		recentRows []db.GetRecentOrdersRow
	)

	// errgroup.WithContext: ถ้า goroutine ใดล้มเหลว → cancel context ทันที
	// goroutine อื่นที่กำลังรอ DB จะ return error จาก cancelled context
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		overview, err = s.queries.GetOrdersOverview(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		cod, err = s.queries.GetCODPending(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		totalSKUs, err = s.queries.CountActiveSKUs(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		statusRows, err = s.queries.GetOrderStatusBreakdown(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		queue, err = s.queries.GetFulfillmentQueueSizes(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		alertRows, err = s.queries.GetStockAlerts(gctx)
		return err
	})

	g.Go(func() error {
		var err error
		recentRows, err = s.queries.GetRecentOrders(gctx)
		return err
	})

	// รอทุก goroutine — คืน error แรกที่เจอ (ถ้ามี)
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("dashboard: %w", err)
	}

	// ── Assemble Response ─────────────────────────────────────────

	breakdown := make([]OrderStatusItem, len(statusRows))
	for i, r := range statusRows {
		breakdown[i] = OrderStatusItem{
			Status: r.Status,
			Count:  r.Count,
		}
	}

	alerts := buildAlerts(alertRows)

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
// query เดียว ไม่ต้อง errgroup
func (s *Service) GetAlerts(ctx context.Context) (*AlertsResponse, error) {
	rows, err := s.queries.GetStockAlerts(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetStockAlerts: %w", err)
	}
	alerts := buildAlerts(rows)
	return &AlertsResponse{
		StockAlerts: alerts,
		Count:       len(alerts),
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────

// buildAlerts แปลง DB rows → StockAlertItem slice
// แยกออกมาเพราะใช้ทั้ง GetDashboard และ GetAlerts
func buildAlerts(rows []db.GetStockAlertsRow) []StockAlertItem {
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
	return alerts
}
