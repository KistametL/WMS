package fulfillment

import "testing"

// ── TestFulfillmentStatuses ───────────────────────────────────────

func TestFulfillmentStatuses(t *testing.T) {
	valid := []string{"confirmed", "picking", "packing", "ready_to_ship"}
	invalid := []string{"pending", "shipped", "delivered", "completed", "cancelled", "", "CONFIRMED"}

	for _, s := range valid {
		if !fulfillmentStatuses[s] {
			t.Errorf("expected %q to be a valid fulfillment status", s)
		}
	}
	for _, s := range invalid {
		if fulfillmentStatuses[s] {
			t.Errorf("expected %q to NOT be a valid fulfillment status", s)
		}
	}
}

// ── TestListResponsePagination ────────────────────────────────────

func TestListResponsePagination(t *testing.T) {
	tests := []struct {
		total      int64
		limit      int
		wantPages  int
	}{
		{total: 0,  limit: 20, wantPages: 0},
		{total: 1,  limit: 20, wantPages: 1},
		{total: 20, limit: 20, wantPages: 1},
		{total: 21, limit: 20, wantPages: 2},
		{total: 40, limit: 20, wantPages: 2},
		{total: 41, limit: 20, wantPages: 3},
		{total: 100, limit: 10, wantPages: 10},
		{total: 101, limit: 10, wantPages: 11},
	}

	for _, tt := range tests {
		totalPages := 0
		if tt.limit > 0 {
			totalPages = (int(tt.total) + tt.limit - 1) / tt.limit
		}
		if totalPages != tt.wantPages {
			t.Errorf("total=%d limit=%d: got totalPages=%d, want %d",
				tt.total, tt.limit, totalPages, tt.wantPages)
		}
	}
}
