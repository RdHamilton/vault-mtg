package search

import (
	"strings"
	"unicode"
)

// SearchField identifies which database column(s) a term targets.
type SearchField int

const (
	FieldAll     SearchField = iota // no prefix: search name, text, types
	FieldType                       // t: prefix -> types column
	FieldText                       // o: prefix -> text (oracle text) column
	FieldKeyword                    // k: prefix -> text column (keyword match)
)

// SearchTerm represents one parsed token from the query string.
type SearchTerm struct {
	Field SearchField
	Value string
}

// ParsedQuery is the output of Parse.
type ParsedQuery struct {
	Terms []SearchTerm
}

// IsEmpty returns true if no meaningful search terms were parsed.
func (pq *ParsedQuery) IsEmpty() bool {
	return len(pq.Terms) == 0
}

// prefixes maps recognized prefix characters to their SearchField.
var prefixes = map[byte]SearchField{
	't': FieldType,
	'T': FieldType,
	'o': FieldText,
	'O': FieldText,
	'k': FieldKeyword,
	'K': FieldKeyword,
}

// Parse parses a search query string into structured search terms.
//
// Supported syntax:
//   - Plain text searches name, oracle text, and types: "lightning bolt"
//   - t:value searches types only: "t:creature", "t:goblin"
//   - o:value searches oracle text only: "o:draw", 'o:"draw a card"'
//   - k:value searches oracle text for keywords: "k:flying"
//   - Double quotes group multi-word values: 'o:"deals damage"'
//   - Multiple prefixes are AND-ed: "t:creature o:damage"
//   - Mixed prefix + bare text: "t:creature bolt"
//   - Consecutive bare words are joined: "lightning bolt" → single FieldAll term
func Parse(query string) *ParsedQuery {
	query = strings.TrimSpace(query)
	if query == "" {
		return &ParsedQuery{}
	}

	var terms []SearchTerm
	var bareWords []string
	pos := 0

	// Note: parsing uses byte indexing which is correct for the ASCII prefix
	// operators (t:, o:, k:, quotes, whitespace). Multi-byte UTF-8 characters
	// in search values are preserved correctly since we only split on ASCII
	// delimiters and pass through value bytes untouched.
	for pos < len(query) {
		// Skip whitespace (ASCII whitespace is single-byte, safe with byte indexing)
		for pos < len(query) && unicode.IsSpace(rune(query[pos])) {
			pos++
		}
		if pos >= len(query) {
			break
		}

		// Check for recognized prefix (single char followed by ':')
		if pos+1 < len(query) && query[pos+1] == ':' {
			if field, ok := prefixes[query[pos]]; ok {
				// Flush accumulated bare words before the prefix term
				if len(bareWords) > 0 {
					terms = append(terms, SearchTerm{Field: FieldAll, Value: strings.Join(bareWords, " ")})
					bareWords = nil
				}

				pos += 2 // skip past "X:"

				// Skip whitespace after colon (lenient)
				for pos < len(query) && unicode.IsSpace(rune(query[pos])) {
					pos++
				}

				value := readValue(query, &pos)
				if value != "" {
					terms = append(terms, SearchTerm{Field: field, Value: value})
				}
				continue
			}
		}

		// No prefix — read as bare word
		word := readValue(query, &pos)
		if word != "" {
			bareWords = append(bareWords, word)
		}
	}

	// Flush remaining bare words
	if len(bareWords) > 0 {
		terms = append(terms, SearchTerm{Field: FieldAll, Value: strings.Join(bareWords, " ")})
	}

	return &ParsedQuery{Terms: terms}
}

// readValue reads the next value token starting at query[*pos].
// If the value starts with a double quote, it reads until the closing quote
// (or end of string). Otherwise, it reads until whitespace or end of string.
// Tokens must be separated by whitespace; adjacent prefixes without spaces
// (e.g., "t:creatureo:damage") will not be split.
func readValue(query string, pos *int) string {
	if *pos >= len(query) {
		return ""
	}

	// Quoted value
	if query[*pos] == '"' {
		*pos++ // skip opening quote
		start := *pos
		for *pos < len(query) && query[*pos] != '"' {
			*pos++
		}
		value := query[start:*pos]
		if *pos < len(query) {
			*pos++ // skip closing quote
		}
		return strings.TrimSpace(value)
	}

	// Unquoted value — read until whitespace
	start := *pos
	for *pos < len(query) && !unicode.IsSpace(rune(query[*pos])) {
		*pos++
	}
	return query[start:*pos]
}
