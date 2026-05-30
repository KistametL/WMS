// Package pgutil provides shared helpers for converting between Go types and
// pgx/pgtype types.  Centralising these conversions avoids duplicating the
// same boilerplate across every service package.
package pgutil

import (
	"fmt"

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
