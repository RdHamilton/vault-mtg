package listing

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Cursor holds the keyset position for a list query. It is opaquely encoded
// as base64(JSON) and returned in page.next_cursor. Clients pass it back
// unmodified on subsequent requests.
//
// OccurredAt is the primary sort key for time-ordered lists (matches, drafts,
// decks). ID is the unique row identifier used as a tiebreaker when two rows
// share the same OccurredAt value. For non-temporal lists (collection) only ID
// is populated.
type Cursor struct {
	OccurredAt *time.Time `json:"t,omitempty"`
	ID         string     `json:"i"`
}

// Encode serialises the cursor to an opaque base64-encoded string suitable for
// inclusion in a page.next_cursor response field.
func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque cursor string produced by Cursor.Encode.
// Returns an error for any input that is not a valid cursor — callers should
// translate this to a 400 response.
func DecodeCursor(s string) (Cursor, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor: not valid base64")
	}

	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, fmt.Errorf("invalid cursor: malformed JSON")
	}

	if c.ID == "" {
		return Cursor{}, fmt.Errorf("invalid cursor: missing id field")
	}

	return c, nil
}
