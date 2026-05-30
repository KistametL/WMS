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
