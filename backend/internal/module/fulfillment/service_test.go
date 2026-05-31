package fulfillment

import (
	"testing"
)

// ── TestConfirmPickValidation ─────────────────────────────────────
// ทดสอบ logic การ validate scanned items vs order items
// ไม่ต้องการ DB — ทดสอบ map comparison logic โดยตรง

// validateScan mirrors the validation logic in ConfirmPick
// แยกออกมาเพื่อ testability โดยไม่ต้องการ DB connection
func validateScan(orderItems map[string]int32, scannedItems []ScannedItem) error {
	scanned := make(map[string]int32, len(scannedItems))
	for _, s := range scannedItems {
		scanned[s.SKUCode] += s.Quantity
	}

	// ทุก item ใน order ต้อง scan ครบถ้วน
	for code, qty := range orderItems {
		if scanned[code] != qty {
			return ErrScanMismatch
		}
	}
	// ไม่มี item แปลกปลอมที่ไม่อยู่ใน order
	for code := range scanned {
		if _, ok := orderItems[code]; !ok {
			return ErrScanMismatch
		}
	}
	return nil
}

func TestConfirmPickValidation(t *testing.T) {
	tests := []struct {
		name        string
		orderItems  map[string]int32 // sku_code → qty
		scanned     []ScannedItem
		wantErr     bool
	}{
		{
			name:       "exact match — single item",
			orderItems: map[string]int32{"SKU001": 2},
			scanned:    []ScannedItem{{SKUCode: "SKU001", Quantity: 2}},
			wantErr:    false,
		},
		{
			name:       "exact match — multiple items",
			orderItems: map[string]int32{"SKU001": 2, "SKU002": 1, "SKU003": 3},
			scanned: []ScannedItem{
				{SKUCode: "SKU003", Quantity: 3},
				{SKUCode: "SKU001", Quantity: 2},
				{SKUCode: "SKU002", Quantity: 1},
			},
			wantErr: false,
		},
		{
			name:       "scanned quantity less than expected",
			orderItems: map[string]int32{"SKU001": 3},
			scanned:    []ScannedItem{{SKUCode: "SKU001", Quantity: 2}},
			wantErr:    true,
		},
		{
			name:       "scanned quantity more than expected",
			orderItems: map[string]int32{"SKU001": 2},
			scanned:    []ScannedItem{{SKUCode: "SKU001", Quantity: 3}},
			wantErr:    true,
		},
		{
			name:       "missing item — not scanned at all",
			orderItems: map[string]int32{"SKU001": 2, "SKU002": 1},
			scanned:    []ScannedItem{{SKUCode: "SKU001", Quantity: 2}},
			wantErr:    true,
		},
		{
			name:       "extra item — not in order",
			orderItems: map[string]int32{"SKU001": 2},
			scanned: []ScannedItem{
				{SKUCode: "SKU001", Quantity: 2},
				{SKUCode: "SKU999", Quantity: 1},
			},
			wantErr: true,
		},
		{
			name:       "wrong sku entirely",
			orderItems: map[string]int32{"SKU001": 1},
			scanned:    []ScannedItem{{SKUCode: "SKU002", Quantity: 1}},
			wantErr:    true,
		},
		{
			name:       "duplicate scan entries accumulate correctly",
			orderItems: map[string]int32{"SKU001": 3},
			scanned: []ScannedItem{
				{SKUCode: "SKU001", Quantity: 1},
				{SKUCode: "SKU001", Quantity: 2}, // scan ทีละชิ้น รวมแล้ว = 3
			},
			wantErr: false,
		},
		{
			name:       "empty order should not crash",
			orderItems: map[string]int32{},
			scanned:    []ScannedItem{},
			wantErr:    false,
		},
		{
			name:       "scan item when order is empty",
			orderItems: map[string]int32{},
			scanned:    []ScannedItem{{SKUCode: "SKU001", Quantity: 1}},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScan(tt.orderItems, tt.scanned)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateScan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				// ตรวจว่า error เป็น ErrScanMismatch เสมอ
				if err != ErrScanMismatch {
					t.Errorf("validateScan() error type = %v, want ErrScanMismatch", err)
				}
			}
		})
	}
}

// ── TestParseUserID ───────────────────────────────────────────────

func TestParseUserID(t *testing.T) {
	tests := []struct {
		name      string
		userID    string
		wantValid bool
	}{
		{
			name:      "valid UUID",
			userID:    "550e8400-e29b-41d4-a716-446655440000",
			wantValid: true,
		},
		{
			name:      "empty string",
			userID:    "",
			wantValid: false,
		},
		{
			name:      "invalid UUID",
			userID:    "not-a-uuid",
			wantValid: false,
		},
		{
			name:      "nil-like UUID",
			userID:    "00000000-0000-0000-0000-000000000000",
			wantValid: true, // valid UUID format แม้จะเป็น zero value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUserID(tt.userID)
			if result.Valid != tt.wantValid {
				t.Errorf("parseUserID(%q).Valid = %v, want %v", tt.userID, result.Valid, tt.wantValid)
			}
		})
	}
}
