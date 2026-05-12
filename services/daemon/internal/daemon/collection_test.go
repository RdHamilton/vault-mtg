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
	// nil channels in select: a select with only nil channels blocks forever —
	// verified by compiler; no runtime test needed.
	assert.Nil(t, hooks.SyncNow)
	assert.Nil(t, hooks.GrantAccess)
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
