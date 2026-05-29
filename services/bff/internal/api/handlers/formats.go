package handlers

import "strings"

// knownFormats is the canonical set of MTGA format strings accepted by
// history and stats endpoints. Comparison is always case-insensitive — use
// IsKnownFormat rather than looking up this map directly.
var knownFormats = map[string]struct{}{
	"standard":  {},
	"historic":  {},
	"brawl":     {},
	"limited":   {},
	"draft":     {},
	"sealed":    {},
	"alchemy":   {},
	"explorer":  {},
	"timeless":  {},
	"gladiator": {},
	"pauper":    {},
}

// IsKnownFormat reports whether s is a recognised MTGA format string.
// The comparison is case-insensitive.
func IsKnownFormat(s string) bool {
	_, ok := knownFormats[strings.ToLower(s)]
	return ok
}
