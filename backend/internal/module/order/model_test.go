package order

import "testing"

// ── State Machine ─────────────────────────────────────────────────

func TestIsValidTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		// valid forward transitions
		{StatusPending, StatusConfirmed, true},
		{StatusPending, StatusCancelled, true},
		{StatusConfirmed, StatusPicking, true},
		{StatusConfirmed, StatusCancelled, true},
		{StatusPicking, StatusPacking, true},
		{StatusPicking, StatusCancelled, true},
		{StatusPacking, StatusReadyToShip, true},
		{StatusPacking, StatusCancelled, true},
		{StatusReadyToShip, StatusShipped, true},
		{StatusShipped, StatusDelivered, true},
		{StatusDelivered, StatusCompleted, true},

		// invalid skips / backwards
		{StatusPending, StatusPicking, false},
		{StatusPending, StatusShipped, false},
		{StatusConfirmed, StatusShipped, false},
		{StatusConfirmed, StatusDelivered, false},
		{StatusPicking, StatusShipped, false},
		{StatusShipped, StatusPending, false},
		{StatusDelivered, StatusPending, false},

		// can't cancel from ready_to_ship / shipped / delivered / completed
		{StatusReadyToShip, StatusCancelled, false},
		{StatusShipped, StatusCancelled, false},
		{StatusDelivered, StatusCancelled, false},
		{StatusCompleted, StatusCancelled, false},

		// terminal states: no way out
		{StatusCompleted, StatusPending, false},
		{StatusCompleted, StatusConfirmed, false},
		{StatusCancelled, StatusPending, false},
		{StatusCancelled, StatusConfirmed, false},

		// unknown status
		{"", StatusConfirmed, false},
		{StatusPending, "", false},
		{"unknown", StatusConfirmed, false},
	}

	for _, tc := range cases {
		t.Run(tc.from+"→"+tc.to, func(t *testing.T) {
			got := isValidTransition(tc.from, tc.to)
			if got != tc.want {
				t.Errorf("isValidTransition(%q, %q) = %v, want %v",
					tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// ── Reserved Statuses ─────────────────────────────────────────────

func TestReservedStatuses(t *testing.T) {
	// statuses ที่ stock ถูก reserve ไว้แล้ว — cancel จาก state เหล่านี้ต้อง unreserve
	reserved := []string{
		StatusConfirmed,
		StatusPicking,
		StatusPacking,
		StatusReadyToShip,
	}
	notReserved := []string{
		StatusPending,
		StatusShipped,
		StatusDelivered,
		StatusCompleted,
		StatusCancelled,
	}

	for _, s := range reserved {
		if !reservedStatuses[s] {
			t.Errorf("expected %q to be in reservedStatuses, but it is not", s)
		}
	}
	for _, s := range notReserved {
		if reservedStatuses[s] {
			t.Errorf("expected %q NOT to be in reservedStatuses, but it is", s)
		}
	}
}

// ── TotalPages Calculation ────────────────────────────────────────
// ตรวจ ceiling division สำหรับ pagination ที่ถูกต้อง

func TestTotalPagesCeilingDivision(t *testing.T) {
	cases := []struct {
		total     int
		limit     int
		wantPages int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 20, 5},
		{101, 20, 6},
		{1, 1, 1},
		{250, 100, 3}, // 250/100 = 2.5 → 3
	}

	for _, tc := range cases {
		total := int64(tc.total)
		limit := tc.limit
		var got int
		if limit > 0 {
			got = (int(total) + limit - 1) / limit
		}
		if got != tc.wantPages {
			t.Errorf("totalPages(total=%d, limit=%d) = %d, want %d",
				tc.total, tc.limit, got, tc.wantPages)
		}
	}
}
