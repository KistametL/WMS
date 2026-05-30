package order

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// ── numericToFloat ────────────────────────────────────────────────

func TestNumericToFloat(t *testing.T) {
	cases := []struct {
		name string
		n    pgtype.Numeric
		want float64
	}{
		{
			name: "invalid/null",
			n:    pgtype.Numeric{Valid: false},
			want: 0,
		},
		{
			name: "NaN",
			n:    pgtype.Numeric{Valid: true, NaN: true},
			want: 0,
		},
		{
			name: "infinity (N5 fix)",
			n:    pgtype.Numeric{Valid: true, InfinityModifier: pgtype.Infinity},
			want: 0,
		},
		{
			name: "negative infinity (N5 fix)",
			n:    pgtype.Numeric{Valid: true, InfinityModifier: pgtype.NegativeInfinity},
			want: 0,
		},
		{
			name: "zero",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(0), Exp: 0},
			want: 0,
		},
		{
			name: "100.50 — typical price (10050 * 10^-2)",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(10050), Exp: -2},
			want: 100.50,
		},
		{
			name: "0.01 — minimum unit price (1 * 10^-2)",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(1), Exp: -2},
			want: 0.01,
		},
		{
			name: "1000 — whole number (1000 * 10^0)",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(1000), Exp: 0},
			want: 1000,
		},
		{
			name: "positive exponent (1 * 10^3 = 1000)",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(1), Exp: 3},
			want: 1000,
		},
		{
			name: "negative value (-99.99)",
			n:    pgtype.Numeric{Valid: true, Int: big.NewInt(-9999), Exp: -2},
			want: -99.99,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := numericToFloat(tc.n)
			if got != tc.want {
				t.Errorf("numericToFloat(...) = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── toNumeric / round-trip ────────────────────────────────────────

func TestToNumericRoundTrip(t *testing.T) {
	// toNumeric → numericToFloat ต้องได้ค่าเดิมกลับมา
	cases := []float64{0, 0.01, 1, 99.99, 100.50, 1234.56, 10000}
	for _, v := range cases {
		got := numericToFloat(toNumeric(v))
		if got != v {
			t.Errorf("round-trip(%v) = %v, want %v", v, got, v)
		}
	}
}

// ── validateShippingAddress ───────────────────────────────────────

func TestValidateShippingAddress(t *testing.T) {
	cases := []struct {
		name    string
		input   json.RawMessage
		wantErr bool
	}{
		{
			name:    "nil (optional, not provided)",
			input:   nil,
			wantErr: false,
		},
		{
			name:    "empty bytes (not provided)",
			input:   json.RawMessage{},
			wantErr: false,
		},
		{
			name:    `"null" literal`,
			input:   json.RawMessage(`null`),
			wantErr: false,
		},
		{
			name:    "valid object",
			input:   json.RawMessage(`{"street":"123 Main St","city":"Bangkok"}`),
			wantErr: false,
		},
		{
			name:    "empty object",
			input:   json.RawMessage(`{}`),
			wantErr: false,
		},
		{
			name:    "string instead of object",
			input:   json.RawMessage(`"123 Main St"`),
			wantErr: true,
		},
		{
			name:    "array instead of object",
			input:   json.RawMessage(`["street","city"]`),
			wantErr: true,
		},
		{
			name:    "number instead of object",
			input:   json.RawMessage(`12345`),
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   json.RawMessage(`{not valid json`),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateShippingAddress(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateShippingAddress() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
