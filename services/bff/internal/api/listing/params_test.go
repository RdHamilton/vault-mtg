package listing_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/listing"
)

var testSortAllowlist = map[string]struct{}{
	"occurred_at": {},
	"updated_at":  {},
}

func TestParseListParams_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	p, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Limit != listing.DefaultLimit {
		t.Errorf("Limit: want %d got %d", listing.DefaultLimit, p.Limit)
	}

	if p.Sort != "occurred_at" {
		t.Errorf("Sort: want occurred_at got %s", p.Sort)
	}

	if p.Order != "desc" {
		t.Errorf("Order: want desc got %s", p.Order)
	}

	if p.Cursor != nil {
		t.Error("Cursor should be nil for first page")
	}
}

func TestParseListParams_LimitClamped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=999", nil)
	p, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Limit != listing.MaxLimit {
		t.Errorf("Limit: want %d got %d", listing.MaxLimit, p.Limit)
	}
}

func TestParseListParams_LimitExplicit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?limit=25", nil)
	p, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Limit != 25 {
		t.Errorf("Limit: want 25 got %d", p.Limit)
	}
}

func TestParseListParams_InvalidLimit(t *testing.T) {
	for _, bad := range []string{"0", "-1", "abc", ""} {
		if bad == "" {
			continue // empty is the default path
		}

		req := httptest.NewRequest(http.MethodGet, "/?limit="+bad, nil)
		_, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
		if err == nil {
			t.Errorf("expected error for limit=%q", bad)
		}
	}
}

func TestParseListParams_UnknownSortField(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?sort=unknown_field", nil)
	_, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err == nil {
		t.Error("expected error for unknown sort field")
	}
}

func TestParseListParams_ValidSort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?sort=updated_at&order=asc", nil)
	p, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Sort != "updated_at" {
		t.Errorf("Sort: want updated_at got %s", p.Sort)
	}

	if p.Order != "asc" {
		t.Errorf("Order: want asc got %s", p.Order)
	}
}

func TestParseListParams_InvalidOrder(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?order=sideways", nil)
	_, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err == nil {
		t.Error("expected error for invalid order value")
	}
}

func TestParseListParams_ValidCursor(t *testing.T) {
	ts := time.Date(2026, 5, 5, 18, 42, 11, 0, time.UTC)
	c := listing.Cursor{OccurredAt: &ts, ID: "m1"}
	encoded := c.Encode()

	req := httptest.NewRequest(http.MethodGet, "/?cursor="+encoded, nil)
	p, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Cursor == nil {
		t.Fatal("Cursor should not be nil")
	}

	if p.Cursor.ID != "m1" {
		t.Errorf("Cursor.ID: want m1 got %s", p.Cursor.ID)
	}
}

func TestParseListParams_MalformedCursor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?cursor=not-a-valid-cursor!!!", nil)
	_, err := listing.ParseListParams(req, testSortAllowlist, "occurred_at")
	if err == nil {
		t.Error("expected error for malformed cursor")
	}
}
