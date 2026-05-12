//go:build darwin

package daemon

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildOsaScript_ContainsAdminPrivileges(t *testing.T) {
	script := buildOsaScript("/usr/local/bin/helper", "/usr/local/lib/install")
	assert.Contains(t, script, "with administrator privileges")
	assert.Contains(t, script, "install-helper.sh")
}

func TestBuildOsaScript_PathWithSpaces(t *testing.T) {
	script := buildOsaScript("/Applications/Vault MTG/helper", "/Applications/Vault MTG/install")
	// Paths with spaces must be shell-quoted.
	assert.Contains(t, script, "'")
	assert.Contains(t, script, "install-helper.sh")
	assert.Contains(t, script, "Vault MTG")
}

func TestBuildOsaScript_PathWithSingleQuote(t *testing.T) {
	script := buildOsaScript("/tmp/vault's/helper", "/tmp/vault's/install")
	// Single quotes inside paths must be escaped as '\''.
	assert.Contains(t, script, `'\''`)
}

func TestBuildOsaScript_NoShellInjection(t *testing.T) {
	// A path containing shell metacharacters must be wrapped in single-quotes.
	script := buildOsaScript("/tmp/helper;rm -rf /", "/tmp/install;rm -rf /")
	// The dangerous helper path must appear fully inside single-quotes.
	assert.Contains(t, script, "'/tmp/helper;rm -rf /'")
}

func TestShellQuote_PlainPath(t *testing.T) {
	assert.Equal(t, "'/usr/local/bin'", shellQuote("/usr/local/bin"))
}

func TestShellQuote_PathWithSpaces(t *testing.T) {
	assert.Equal(t, "'/path/with spaces'", shellQuote("/path/with spaces"))
}

func TestShellQuote_PathWithSingleQuote(t *testing.T) {
	got := shellQuote("/it's here")
	assert.True(t, strings.HasPrefix(got, "'"))
	assert.Contains(t, got, `'\''`)
}
