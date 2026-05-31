// Package pgutil provides shared helpers for converting between Go types and
// pgx/pgtype types.  Centralising these conversions avoids duplicating the
// same boilerplate across every service package.
package pgutil

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

// ParseUUID parses a UUID string into a pgtype.UUID.
// Returns an error wrapping the original scan failure if s is not a valid UUID.
func ParseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return id, nil
}

// UUIDString formats a pgtype.UUID as a lowercase hyphenated UUID string.
// Returns "" when the UUID is not valid (NULL).
func UUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u.Bytes[0:4],
		u.Bytes[4:6],
		u.Bytes[6:8],
		u.Bytes[8:10],
		u.Bytes[10:16],
	)
}

// NormalizePagination clamps page and limit to sane values and returns the
// SQL OFFSET.  limit is capped at 100 to prevent accidental large fetches.
func NormalizePagination(page, limit int) (p, l, offset int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return page, limit, (page - 1) * limit
}

// NumericToFloat converts pgtype.Numeric → float64.
// Formula: value = Int × 10^Exp
// Uses big.Rat for negative exponents (typical for prices: NUMERIC(15,2) → Exp=-2).
// Returns 0 for NULL, NaN, or Infinity values.
func NumericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite || n.Int == nil {
		return 0
	}
	if n.Exp >= 0 {
		exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n.Exp)), nil)
		val := new(big.Int).Mul(n.Int, exp)
		f, _ := new(big.Float).SetInt(val).Float64()
		return f
	}
	// negative exponent (e.g. price: 10050 × 10^-2 = 100.50)
	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-n.Exp)), nil)
	rat := new(big.Rat).SetFrac(n.Int, div)
	f, _ := rat.Float64()
	return f
}

// ToNumeric converts float64 → pgtype.Numeric via string representation.
// pgtype.Numeric.Scan accepts string input (pgx v5).
func ToNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

// ToText converts *string → pgtype.Text (NULL when nil).
func ToText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// OptText returns a valid pgtype.Text for non-empty strings, NULL otherwise.
func OptText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
