package listing

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	// DefaultLimit is the default page size for all list endpoints.
	DefaultLimit = 50
	// MaxLimit is the maximum page size; requests above this are silently clamped.
	MaxLimit = 200
)

// ListParams holds the parsed and validated pagination parameters extracted
// from an HTTP request. Handlers pass relevant fields to their repository
// methods.
type ListParams struct {
	// Cursor is the keyset position from the previous page. Nil on the first page.
	Cursor *Cursor
	// Limit is the effective page size after clamping.
	Limit int
	// Sort is the active sort field name (validated against the endpoint's allowlist).
	Sort string
	// Order is "asc" or "desc".
	Order string
}

// ParseListParams parses cursor, limit, sort, and order from the HTTP request.
//
//   - sortAllowlist maps allowed sort field names to struct{}. Unknown sort fields
//     return a 400-worthy error.
//   - defaultSort is the sort field applied when ?sort is absent.
//
// Unknown query parameters (filters etc.) are not validated here; each handler
// is responsible for its own filter parameters.
func ParseListParams(r *http.Request, sortAllowlist map[string]struct{}, defaultSort string) (ListParams, error) {
	p := ListParams{
		Limit: DefaultLimit,
		Sort:  defaultSort,
		Order: "desc",
	}

	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return ListParams{}, fmt.Errorf("invalid limit: must be a positive integer")
		}

		if n > MaxLimit {
			n = MaxLimit
		}

		p.Limit = n
	}

	if s := r.URL.Query().Get("cursor"); s != "" {
		c, err := DecodeCursor(s)
		if err != nil {
			return ListParams{}, err
		}

		p.Cursor = &c
	}

	if s := r.URL.Query().Get("sort"); s != "" {
		if _, ok := sortAllowlist[s]; !ok {
			return ListParams{}, fmt.Errorf("unknown sort field: %q", s)
		}

		p.Sort = s
	}

	if s := r.URL.Query().Get("order"); s != "" {
		if s != "asc" && s != "desc" {
			return ListParams{}, fmt.Errorf("invalid order: must be asc or desc")
		}

		p.Order = s
	}

	return p, nil
}
