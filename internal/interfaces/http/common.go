package http

import "strconv"

// listResponse is the envelope every paginated list endpoint returns, per
// docs/10-api-contracts.md#conventions (cursor-based, never offset-based).
type listResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func parseLimit(raw string) int {
	if raw == "" {
		return 0 // service layer applies its own default
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
