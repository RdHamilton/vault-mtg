// Package handlers provides HTTP request handlers for the BFF service.
package handlers

import (
	"crypto/sha256"
	"fmt"
)

// hashAccountID returns a privacy-safe representation of accountID for
// PostHog: SHA-256 hex, first 16 characters.  No raw PII is ever sent.
// The input must already be the string form of the account id (e.g.
// strconv.FormatInt(accountID, 10)).
func hashAccountID(accountID string) string {
	sum := sha256.Sum256([]byte(accountID))
	return fmt.Sprintf("%x", sum)[:16]
}
