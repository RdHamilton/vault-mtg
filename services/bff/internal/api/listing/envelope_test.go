package listing_test

import (
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/listing"
)

type testRow struct {
	ID        string
	Timestamp time.Time
}

func cursorFor(r testRow) listing.Cursor {
	return listing.Cursor{OccurredAt: &r.Timestamp, ID: r.ID}
}

func makeRows(n int) []testRow {
	rows := make([]testRow, n)
	for i := range rows {
		ts := time.Date(2026, 5, 5, 0, i, 0, 0, time.UTC)
		rows[i] = testRow{ID: "r" + string(rune('a'+i)), Timestamp: ts}
	}

	return rows
}

func TestBuildEnvelope_FirstPageHasMore(t *testing.T) {
	// fetch limit+1 rows → has_more should be true
	rows := makeRows(6) // limit=5, fetched 6
	env := listing.BuildEnvelope(rows, 5, cursorFor)

	if len(env.Data) != 5 {
		t.Errorf("data length: want 5 got %d", len(env.Data))
	}

	if !env.Page.HasMore {
		t.Error("has_more should be true")
	}

	if env.Page.NextCursor == nil {
		t.Error("next_cursor should not be nil when has_more=true")
	}

	if env.Page.Limit != 5 {
		t.Errorf("limit: want 5 got %d", env.Page.Limit)
	}
}

func TestBuildEnvelope_LastPage(t *testing.T) {
	// exactly limit rows → has_more=false
	rows := makeRows(5)
	env := listing.BuildEnvelope(rows, 5, cursorFor)

	if len(env.Data) != 5 {
		t.Errorf("data length: want 5 got %d", len(env.Data))
	}

	if env.Page.HasMore {
		t.Error("has_more should be false")
	}

	if env.Page.NextCursor != nil {
		t.Error("next_cursor should be nil when has_more=false")
	}
}

func TestBuildEnvelope_Empty(t *testing.T) {
	env := listing.BuildEnvelope([]testRow{}, 50, cursorFor)

	if env.Data == nil {
		t.Error("data should be empty slice, not nil")
	}

	if len(env.Data) != 0 {
		t.Errorf("data length: want 0 got %d", len(env.Data))
	}

	if env.Page.HasMore {
		t.Error("has_more should be false for empty result")
	}

	if env.Page.NextCursor != nil {
		t.Error("next_cursor should be nil for empty result")
	}
}

func TestBuildEnvelope_NilSlice(t *testing.T) {
	// repository may return nil slice when no rows found
	env := listing.BuildEnvelope[testRow](nil, 50, cursorFor)

	if env.Data == nil {
		t.Error("data should be [] not nil after BuildEnvelope")
	}
}

func TestBuildEnvelope_CursorPointsToLastRow(t *testing.T) {
	rows := makeRows(6) // limit=5, +1 for has_more detection
	env := listing.BuildEnvelope(rows, 5, cursorFor)

	if env.Page.NextCursor == nil {
		t.Fatal("next_cursor is nil")
	}

	// decode the cursor and verify it points to the 5th row (index 4)
	decoded, err := listing.DecodeCursor(*env.Page.NextCursor)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}

	want := cursorFor(rows[4]) // last row in the returned page

	if decoded.ID != want.ID {
		t.Errorf("cursor ID: want %q got %q", want.ID, decoded.ID)
	}
}
