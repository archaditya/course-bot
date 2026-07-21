package postgres

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// Cursor pagination per docs/10-api-contracts.md#conventions ("never
// offset-based"). Encodes (created_at, id) of the last row seen so the next
// page can resume with a stable `WHERE (created_at, id) > (?, ?)` predicate
// — offset pagination breaks under concurrent inserts, this doesn't.
func encodeCursor(createdAt time.Time, id string) string {
	raw := fmt.Sprintf("%s|%s", createdAt.Format(time.RFC3339Nano), id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (createdAt time.Time, id string, hasCursor bool, err error) {
	if cursor == "" {
		return time.Time{}, "", false, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", false, fmt.Errorf("postgres: invalid cursor: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", false, fmt.Errorf("postgres: invalid cursor shape")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", false, fmt.Errorf("postgres: invalid cursor timestamp: %w", err)
	}
	return t, parts[1], true, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 20
	}
	return limit
}
