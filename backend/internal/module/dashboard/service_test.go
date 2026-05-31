package dashboard

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/KistametL/WMS/backend/internal/database/generated"
)

// TestBuildAlerts ทดสอบ buildAlerts — pure function ที่แปลง DB rows → StockAlertItem
// ครอบคลุม: out_of_stock, low_stock, qty_available=NULL, empty input
func TestBuildAlerts(t *testing.T) {
	t.Parallel()

	skuID := func(s string) pgtype.UUID {
		var u pgtype.UUID
		_ = u.Scan(s)
		return u
	}

	qtyInt4 := func(v int32) pgtype.Int4 {
		return pgtype.Int4{Int32: v, Valid: true}
	}

	tests := []struct {
		name string
		rows []db.GetStockAlertsRow
		want []StockAlertItem
	}{
		{
			name: "empty input returns empty slice",
			rows: []db.GetStockAlertsRow{},
			want: []StockAlertItem{},
		},
		{
			name: "out_of_stock when qty_available = 0",
			rows: []db.GetStockAlertsRow{
				{
					SkuID:             skuID("00000000-0000-0000-0000-000000000001"),
					SkuCode:           "SKU-001",
					ProductName:       "สินค้า A",
					QtyAvailable:      qtyInt4(0),
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
			},
			want: []StockAlertItem{
				{
					SKUID:             "00000000-0000-0000-0000-000000000001",
					SKUCode:           "SKU-001",
					ProductName:       "สินค้า A",
					QtyAvailable:      0,
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
			},
		},
		{
			name: "low_stock when qty_available > 0 but <= threshold",
			rows: []db.GetStockAlertsRow{
				{
					SkuID:             skuID("00000000-0000-0000-0000-000000000002"),
					SkuCode:           "SKU-002",
					ProductName:       "สินค้า B",
					QtyAvailable:      qtyInt4(3),
					LowStockThreshold: 10,
					AlertType:         "low_stock",
				},
			},
			want: []StockAlertItem{
				{
					SKUID:             "00000000-0000-0000-0000-000000000002",
					SKUCode:           "SKU-002",
					ProductName:       "สินค้า B",
					QtyAvailable:      3,
					LowStockThreshold: 10,
					AlertType:         "low_stock",
				},
			},
		},
		{
			name: "NULL qty_available defaults to 0",
			rows: []db.GetStockAlertsRow{
				{
					SkuID:             skuID("00000000-0000-0000-0000-000000000003"),
					SkuCode:           "SKU-003",
					ProductName:       "สินค้า C",
					QtyAvailable:      pgtype.Int4{Valid: false}, // NULL
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
			},
			want: []StockAlertItem{
				{
					SKUID:             "00000000-0000-0000-0000-000000000003",
					SKUCode:           "SKU-003",
					ProductName:       "สินค้า C",
					QtyAvailable:      0, // default
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
			},
		},
		{
			name: "multiple rows preserve order",
			rows: []db.GetStockAlertsRow{
				{
					SkuID:             skuID("00000000-0000-0000-0000-000000000010"),
					SkuCode:           "SKU-010",
					ProductName:       "สินค้า X",
					QtyAvailable:      qtyInt4(0),
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
				{
					SkuID:             skuID("00000000-0000-0000-0000-000000000011"),
					SkuCode:           "SKU-011",
					ProductName:       "สินค้า Y",
					QtyAvailable:      qtyInt4(2),
					LowStockThreshold: 10,
					AlertType:         "low_stock",
				},
			},
			want: []StockAlertItem{
				{
					SKUID:             "00000000-0000-0000-0000-000000000010",
					SKUCode:           "SKU-010",
					ProductName:       "สินค้า X",
					QtyAvailable:      0,
					LowStockThreshold: 5,
					AlertType:         "out_of_stock",
				},
				{
					SKUID:             "00000000-0000-0000-0000-000000000011",
					SKUCode:           "SKU-011",
					ProductName:       "สินค้า Y",
					QtyAvailable:      2,
					LowStockThreshold: 10,
					AlertType:         "low_stock",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildAlerts(tc.rows)

			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d, want %d", len(got), len(tc.want))
			}

			for i, g := range got {
				w := tc.want[i]
				if g.SKUID != w.SKUID {
					t.Errorf("[%d] SKUID: got %q, want %q", i, g.SKUID, w.SKUID)
				}
				if g.SKUCode != w.SKUCode {
					t.Errorf("[%d] SKUCode: got %q, want %q", i, g.SKUCode, w.SKUCode)
				}
				if g.ProductName != w.ProductName {
					t.Errorf("[%d] ProductName: got %q, want %q", i, g.ProductName, w.ProductName)
				}
				if g.QtyAvailable != w.QtyAvailable {
					t.Errorf("[%d] QtyAvailable: got %d, want %d", i, g.QtyAvailable, w.QtyAvailable)
				}
				if g.LowStockThreshold != w.LowStockThreshold {
					t.Errorf("[%d] LowStockThreshold: got %d, want %d", i, g.LowStockThreshold, w.LowStockThreshold)
				}
				if g.AlertType != w.AlertType {
					t.Errorf("[%d] AlertType: got %q, want %q", i, g.AlertType, w.AlertType)
				}
			}
		})
	}
}
