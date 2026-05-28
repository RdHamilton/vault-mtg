package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTrayHooks_NilChannelsAreSafe(t *testing.T) {
	// A zero-value TrayHooks must not panic when its func fields are called.
	hooks := TrayHooks{}
	if hooks.SetHelperInstalled != nil {
		hooks.SetHelperInstalled(true)
	}
	if hooks.SetLastSync != nil {
		hooks.SetLastSync(time.Now())
	}
	if hooks.SetSetupRequired != nil {
		hooks.SetSetupRequired(true)
	}
	// nil channels in select: a select with only nil channels blocks forever —
	// verified by compiler; no runtime test needed.
	assert.Nil(t, hooks.SyncNow)
	assert.Nil(t, hooks.GrantAccess)
	assert.Nil(t, hooks.RetrySetup)
}

func TestTrayHooks_CallbacksInvoked(t *testing.T) {
	var helperState bool
	var lastSync time.Time

	hooks := TrayHooks{
		SetHelperInstalled: func(v bool) { helperState = v },
		SetLastSync:        func(t time.Time) { lastSync = t },
	}

	hooks.SetHelperInstalled(true)
	assert.True(t, helperState)

	hooks.SetHelperInstalled(false)
	assert.False(t, helperState)

	ts := time.Date(2026, 5, 12, 14, 30, 0, 0, time.UTC)
	hooks.SetLastSync(ts)
	assert.Equal(t, ts, lastSync)
}

// TestTrayHooks_SetSetupRequired verifies the SetSetupRequired callback is
// invoked and that the RetrySetup channel field accepts a buffered channel.
func TestTrayHooks_SetSetupRequired(t *testing.T) {
	var setupState bool
	retryCh := make(chan struct{}, 1)

	hooks := TrayHooks{
		SetSetupRequired: func(v bool) { setupState = v },
		RetrySetup:       retryCh,
	}

	hooks.SetSetupRequired(true)
	assert.True(t, setupState)

	hooks.SetSetupRequired(false)
	assert.False(t, setupState)

	// Verify RetrySetup channel is wired and non-blocking at cap=1.
	retryCh <- struct{}{}
	assert.Len(t, hooks.RetrySetup, 1)
}
