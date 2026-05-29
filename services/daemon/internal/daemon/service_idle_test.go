package daemon

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errMTGANotFound is the sentinel error returned by the fake defaultLogPathFn
// when MTGA is not installed.
var errMTGANotFound = errors.New("no log files found in any MTGA log directories")

// TestIdleUntilMTGADetected_ContextCancel verifies that idleUntilMTGADetected
// returns context.Canceled when the context is cancelled while MTGA is absent,
// and that the tray hook is called with true on entry and false on exit.
func TestIdleUntilMTGADetected_ContextCancel(t *testing.T) {
	// Arrange: shorten the poll interval so the test runs fast.
	origInterval := mtgaDetectInterval
	mtgaDetectInterval = 10 * time.Millisecond
	defer func() { mtgaDetectInterval = origInterval }()

	// Override DefaultLogPath to always return "not found".
	origFn := defaultLogPathFn
	defaultLogPathFn = func() (string, error) { return "", errMTGANotFound }
	defer func() { defaultLogPathFn = origFn }()

	var hookCalls []bool
	svc := &Service{
		trayHooks: TrayHooks{
			SetWaitingForArena: func(v bool) { hookCalls = append(hookCalls, v) },
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay — enough for at least one poll tick.
	time.AfterFunc(50*time.Millisecond, cancel)

	err := svc.idleUntilMTGADetected(ctx)

	require.ErrorIs(t, err, context.Canceled)
	// Hook must have been called with true (entry) then false (defer exit).
	require.GreaterOrEqual(t, len(hookCalls), 2, "expected at least entry+exit hook calls")
	assert.True(t, hookCalls[0], "first hook call should be true (entering idle)")
	assert.False(t, hookCalls[len(hookCalls)-1], "last hook call should be false (exiting idle)")
}

// TestIdleUntilMTGADetected_DetectsOnSecondPoll verifies that idleUntilMTGADetected
// returns nil as soon as DefaultLogPath succeeds, and that the tray hook is called
// with true on entry and false on exit.
func TestIdleUntilMTGADetected_DetectsOnSecondPoll(t *testing.T) {
	origInterval := mtgaDetectInterval
	mtgaDetectInterval = 10 * time.Millisecond
	defer func() { mtgaDetectInterval = origInterval }()

	origFn := defaultLogPathFn
	var callCount atomic.Int32
	defaultLogPathFn = func() (string, error) {
		n := callCount.Add(1)
		if n < 2 {
			return "", errMTGANotFound
		}
		return "/fake/Player.log", nil
	}
	defer func() { defaultLogPathFn = origFn }()

	var hookCalls []bool
	svc := &Service{
		trayHooks: TrayHooks{
			SetWaitingForArena: func(v bool) { hookCalls = append(hookCalls, v) },
		},
	}

	ctx := context.Background()
	err := svc.idleUntilMTGADetected(ctx)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(hookCalls), 2, "expected at least entry+exit hook calls")
	assert.True(t, hookCalls[0], "first hook call should be true (entering idle)")
	assert.False(t, hookCalls[len(hookCalls)-1], "last hook call should be false (exiting idle)")
}

// TestIdleUntilMTGADetected_NilHook verifies that idleUntilMTGADetected does not
// panic when SetWaitingForArena is nil in TrayHooks.
func TestIdleUntilMTGADetected_NilHook(t *testing.T) {
	origInterval := mtgaDetectInterval
	mtgaDetectInterval = 10 * time.Millisecond
	defer func() { mtgaDetectInterval = origInterval }()

	origFn := defaultLogPathFn
	var callCount atomic.Int32
	defaultLogPathFn = func() (string, error) {
		n := callCount.Add(1)
		if n < 2 {
			return "", errMTGANotFound
		}
		return "/fake/Player.log", nil
	}
	defer func() { defaultLogPathFn = origFn }()

	// SetWaitingForArena is nil — must not panic.
	svc := &Service{
		trayHooks: TrayHooks{},
	}

	require.NotPanics(t, func() {
		err := svc.idleUntilMTGADetected(context.Background())
		require.NoError(t, err)
	})
}
