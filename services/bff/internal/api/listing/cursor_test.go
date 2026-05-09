package listing_test

import (
	"testing"
	"time"

	"github.com/ramonehamilton/mtga-bff/internal/api/listing"
)

func TestCursor_RoundTrip_WithTime(t *testing.T) {
	ts := time.Date(2026, 5, 5, 18, 42, 11, 0, time.UTC)
	original := listing.Cursor{OccurredAt: &ts, ID: "match-uuid-123"}

	encoded := original.Encode()
	if encoded == "" {
		t.Fatal("Encode returned empty string")
	}

	decoded, err := listing.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: want %q got %q", original.ID, decoded.ID)
	}

	if decoded.OccurredAt == nil {
		t.Fatal("OccurredAt is nil after round-trip")
	}

	if !decoded.OccurredAt.Equal(*original.OccurredAt) {
		t.Errorf("OccurredAt: want %v got %v", *original.OccurredAt, *decoded.OccurredAt)
	}
}

func TestCursor_RoundTrip_WithoutTime(t *testing.T) {
	original := listing.Cursor{ID: "12345"}

	decoded, err := listing.DecodeCursor(original.Encode())
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: want %q got %q", original.ID, decoded.ID)
	}

	if decoded.OccurredAt != nil {
		t.Error("OccurredAt should be nil for ID-only cursor")
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := listing.DecodeCursor("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodeCursor_MalformedJSON(t *testing.T) {
	import64 := "bm90LWpzb24=" // base64("not-json")
	_, err := listing.DecodeCursor(import64)
	if err == nil {
		t.Error("expected error for malformed JSON cursor")
	}
}

func TestDecodeCursor_MissingID(t *testing.T) {
	// valid base64+json but no "i" field
	c := listing.Cursor{} // ID is ""
	encoded := c.Encode()
	_, err := listing.DecodeCursor(encoded)
	if err == nil {
		t.Error("expected error for cursor with empty id")
	}
}

func TestDecodeCursor_EmptyString(t *testing.T) {
	_, err := listing.DecodeCursor("")
	if err == nil {
		t.Error("expected error for empty cursor string")
	}
}
