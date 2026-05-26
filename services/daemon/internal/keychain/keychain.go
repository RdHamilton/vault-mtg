// Package keychain provides CGO-free OS keychain access for daemon API key storage.
//
// Service names (ADR-022 Phase 2):
//
//	ServiceNameNew    = "com.vaultmtg.daemon"     (production — all writes go here)
//	ServiceNameLegacy = "com.mtga-companion.daemon" (read-only fallback for upgrade)
//
// Account key:  api-key
//
// On macOS, go-keyring uses the Keychain Services API via security(1) subprocess —
// no CGO required.  On Windows it uses the Windows Credential Manager via
// golang.org/x/sys/windows syscalls — also CGO-free.
// Both targets cross-compile cleanly from a macOS/Linux CI runner.
//
// Upgrade migration (ADR-022 Constraint 1):
// On startup, Get() first tries ServiceNameNew.  If the entry is absent it tries
// ServiceNameLegacy; when found there, it copies the key forward to ServiceNameNew
// (so subsequent reads hit the new name) and logs the migration at INFO.  The
// legacy entry is RETAINED — never deleted — to allow safe downgrade. Deletion of
// the legacy entry is deferred to Phase 6.
//
// See ADR-020 §Keychain Storage for the full design rationale.
package keychain

import (
	"errors"
	"fmt"
	"log"

	"github.com/zalando/go-keyring"
)

const (
	// ServiceNameNew is the current OS keychain service identifier for the daemon
	// (ADR-022 Phase 2 brand rename).  All writes target this name.
	ServiceNameNew = "com.vaultmtg.daemon"

	// ServiceNameLegacy is the pre-rename OS keychain service identifier retained
	// for read-only upgrade migration.  Do NOT write to this name in production code.
	// This constant is used ONLY in the Get() fallback branch and in tests.
	// Deletion of the legacy entry is deferred to Phase 6.
	ServiceNameLegacy = "com.mtga-companion.daemon"

	// AccountKey is the OS keychain account name under which the API key is stored.
	AccountKey = "api-key"
)

// ErrNotFound is returned when no API key is stored in the keychain for this daemon.
var ErrNotFound = errors.New("keychain: api key not found")

// keyringGet is the package-level indirection for keyring.Get used by Get().
// Tests substitute this variable to inject per-service-name behavior that
// the go-keyring mock backend cannot express (MockInitWithError applies the
// same error to every call).  Production code calls keyring.Get directly via
// this variable; do not reassign outside of tests.
var keyringGet = keyring.Get

// Get retrieves the daemon API key from the OS keychain.
//
// Migration path (ADR-022 Constraint 1):
//  1. Try ServiceNameNew.  If found → return it.
//  2. Try ServiceNameLegacy.  If found → copy key forward to ServiceNameNew,
//     log the migration at INFO, and return the key.  The legacy entry is
//     retained (NOT deleted) for downgrade safety.
//  3. Neither entry present → return ErrNotFound (triggers normal PKCE re-auth).
//
// A corrupted / unreadable legacy entry is treated as absent and falls through
// to ErrNotFound so the caller initiates re-auth rather than crashing.
func Get() (string, error) {
	// ── 1. Try new service name first ────────────────────────────────────────
	val, err := keyringGet(ServiceNameNew, AccountKey)
	if err == nil {
		return val, nil
	}
	if !isNotFound(err) {
		return "", fmt.Errorf("keychain: get %q: %w", ServiceNameNew, err)
	}

	// ── 2. Fall back to legacy service name ──────────────────────────────────
	legacyVal, legacyErr := keyringGet(ServiceNameLegacy, AccountKey)
	if legacyErr != nil {
		if isNotFound(legacyErr) {
			// Neither entry present — fresh install or wiped keychain.
			return "", ErrNotFound
		}
		// Corrupted / unreadable legacy entry: log a warning and fall through
		// to ErrNotFound so normal PKCE re-auth is triggered rather than crashing.
		log.Printf("[keychain] warn: could not read legacy entry %q: %v — falling through to re-auth", ServiceNameLegacy, legacyErr)
		return "", ErrNotFound
	}

	// ── 3. Copy forward to new service name ──────────────────────────────────
	// The legacy entry is RETAINED (not deleted) for downgrade safety.
	// Deletion of the legacy entry is deferred to Phase 6.
	if copyErr := keyring.Set(ServiceNameNew, AccountKey, legacyVal); copyErr != nil {
		log.Printf("[keychain] warn: could not copy legacy keychain entry to %q: %v — proceeding with legacy key", ServiceNameNew, copyErr)
	} else {
		log.Printf("[keychain] INFO: migrated keychain entry from %q to %q (legacy entry retained for downgrade safety)", ServiceNameLegacy, ServiceNameNew)
	}

	return legacyVal, nil
}

// Set stores the daemon API key in the OS keychain under ServiceNameNew,
// creating or replacing any existing entry.
// The legacy ServiceNameLegacy entry is never written by this function.
func Set(apiKey string) error {
	if err := keyring.Set(ServiceNameNew, AccountKey, apiKey); err != nil {
		return fmt.Errorf("keychain: set: %w", err)
	}
	return nil
}

// Delete removes the daemon API key from the OS keychain (ServiceNameNew only).
// The legacy entry is NOT deleted — it is retained for downgrade safety.
// Returns nil if no key was stored (idempotent).
func Delete() error {
	err := keyring.Delete(ServiceNameNew, AccountKey)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("keychain: delete: %w", err)
	}
	return nil
}

// isNotFound detects the go-keyring "not found" sentinel.
// go-keyring returns keyring.ErrNotFound — compare by value.
func isNotFound(err error) bool {
	return errors.Is(err, keyring.ErrNotFound)
}
