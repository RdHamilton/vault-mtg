// Internal test file for the keychain package (package keychain — not
// keychain_test) so we can swap the unexported keyringGet seam to inject
// per-service-name behavior that go-keyring's MockInitWithError cannot
// express.  See #2255.
package keychain

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zalando/go-keyring"
)

// errCorruptedLegacy simulates a non-ErrNotFound failure reading the legacy
// keychain entry (e.g., access denied, I/O error, corrupted blob).  Any error
// that is NOT errors.Is(err, keyring.ErrNotFound) routes Get() into the
// corrupted-legacy branch in keychain.go.
var errCorruptedLegacy = errors.New("keychain: access denied")

// swapKeyringGet replaces the package-level keyringGet variable with fn for
// the duration of the test, restoring the original on cleanup.  This lets a
// test inject per-service-name behavior that go-keyring's MockInitWithError
// cannot express (MockInitWithError returns the same error for every call).
func swapKeyringGet(t *testing.T, fn func(service, user string) (string, error)) {
	t.Helper()
	orig := keyringGet
	keyringGet = fn
	t.Cleanup(func() { keyringGet = orig })
}

// captureLog redirects the default logger to a buffer for the duration of the
// test and returns the buffer.  Used to assert that branch-specific log lines
// fire (or do not fire).
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	return &buf
}

// TestGet_CorruptedLegacyEntry verifies the corrupted-legacy branch in Get():
// when ServiceNameNew is absent (ErrNotFound) and ServiceNameLegacy returns a
// non-ErrNotFound error (corrupted / unreadable entry), Get() must log a
// warning and fall through to ErrNotFound so callers initiate PKCE re-auth
// rather than crashing.
//
// This test uses a per-service-name swap of keyringGet because go-keyring's
// MockInitWithError applies a single error to every call and cannot express
// "ErrNotFound on new + corruption on legacy" (#2255).
func TestGet_CorruptedLegacyEntry(t *testing.T) {
	swapKeyringGet(t, func(service, user string) (string, error) {
		switch service {
		case ServiceNameNew:
			return "", keyring.ErrNotFound
		case ServiceNameLegacy:
			return "", errCorruptedLegacy
		default:
			t.Fatalf("unexpected service name: %q", service)
			return "", nil
		}
	})

	logBuf := captureLog(t)

	_, err := Get()
	// Must return ErrNotFound (not a raw keyring error) so callers that check
	// errors.Is(err, ErrNotFound) can trigger re-auth cleanly.
	assert.ErrorIs(t, err, ErrNotFound,
		"corrupted legacy entry must fall through to ErrNotFound, not crash")

	// Confirm we actually hit the corrupted-legacy branch (not the
	// neither-entry-present branch) by checking the warning was logged.
	logged := logBuf.String()
	assert.True(t,
		strings.Contains(logged, "could not read legacy entry") &&
			strings.Contains(logged, errCorruptedLegacy.Error()),
		"corrupted-legacy branch must log a warning identifying the underlying error; got: %q", logged)
}

// TestGet_NeitherEntryPresent verifies the "neither entry present" branch
// (fresh install / wiped keychain): ServiceNameNew returns ErrNotFound AND
// ServiceNameLegacy returns ErrNotFound, so Get() returns ErrNotFound without
// logging a corruption warning.
//
// This complements TestGet_NotFound (which uses the in-memory mock with an
// empty store) and TestGet_NeitherEntryPresent_GlobalMockError (which uses
// MockInitWithError(ErrNotFound)) by explicitly asserting the no-warning
// behavior through the seam.
func TestGet_NeitherEntryPresent(t *testing.T) {
	swapKeyringGet(t, func(service, user string) (string, error) {
		return "", keyring.ErrNotFound
	})

	logBuf := captureLog(t)

	_, err := Get()
	assert.ErrorIs(t, err, ErrNotFound,
		"absent entries on both names must return ErrNotFound")

	// The corruption-warning log line must NOT appear for the neither-present case.
	assert.NotContains(t, logBuf.String(), "could not read legacy entry",
		"neither-present branch must not log a corruption warning")
}
