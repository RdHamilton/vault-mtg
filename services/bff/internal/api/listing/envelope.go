package listing

// Page holds cursor-based pagination metadata returned in every list response.
// Clients check has_more to determine whether more data is available and pass
// next_cursor to the next request to fetch the following page.
type Page struct {
	// NextCursor is the opaque cursor to pass on the next request. null when
	// has_more is false.
	NextCursor *string `json:"next_cursor"`
	// HasMore is true when more rows exist after this page.
	HasMore bool `json:"has_more"`
	// Limit is the effective limit applied to this response (post-clamping).
	Limit int `json:"limit"`
}

// ListEnvelope is the standard JSON wrapper for all BFF list endpoint responses.
// data contains the rows for this page; page contains the pagination metadata.
type ListEnvelope[T any] struct {
	Data []T  `json:"data"`
	Page Page `json:"page"`
}

// BuildEnvelope constructs a ListEnvelope from a rows slice and pagination
// metadata. The caller must fetch limit+1 rows from the repository; BuildEnvelope
// detects the extra row to set has_more=true and trims the slice back to limit.
//
// cursorFn extracts the Cursor for the last returned row. It is only called
// when has_more is true.
func BuildEnvelope[T any](rows []T, limit int, cursorFn func(T) Cursor) ListEnvelope[T] {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	var nextCursor *string
	if hasMore && len(rows) > 0 {
		c := cursorFn(rows[len(rows)-1])
		s := c.Encode()
		nextCursor = &s
	}

	// Return an empty slice rather than nil so that JSON serialisation produces
	// [] instead of null.
	if rows == nil {
		rows = []T{}
	}

	return ListEnvelope[T]{
		Data: rows,
		Page: Page{
			NextCursor: nextCursor,
			HasMore:    hasMore,
			Limit:      limit,
		},
	}
}
