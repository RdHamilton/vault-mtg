# Milestone 2: Technical Debt & Quality

## Overview
Address technical debt and improve code quality before adding new features.

---

## Issue #799 - Add comprehensive tests for replay_engine.go
**Status**: Todo
**Complexity**: Medium
**Priority**: High

**Current Coverage**: Low (identified in PR #798)

**Test Cases Needed**:
- `TestReplayEngine_Start`
- `TestReplayEngine_Pause`
- `TestReplayEngine_Resume`
- `TestReplayEngine_Stop`
- `TestReplayEngine_SetSpeed`
- `TestReplayEngine_GetStatus`
- `TestReplayEngine_BroadcastEvents`
- `TestReplayEngine_ConcurrentOperations`
- `TestReplayEngine_ErrorHandling`
- `TestReplayEngine_FilteredReplay`

**Files**:
- `internal/daemon/replay_engine.go` (target)
- `internal/daemon/replay_engine_test.go` (create)

---

## Issue #800 - Add Stop method to API WebSocket Hub
**Status**: Todo
**Complexity**: Low
**Priority**: Medium

**Context**: Hub runs goroutine without Stop method, causing test goroutine leaks.

**Implementation**:
```go
// In internal/api/websocket/hub.go
func (h *Hub) Stop() {
    close(h.done) // Add done channel
}
```

**Also needed**:
- Update `TestServer_NewDaemonEventForwarder_UsesServerHub` to call Stop
- Add test for Hub.Stop() method

---

## Issue #795 - Benchmark greenteagc garbage collector
**Status**: Todo
**Complexity**: Low
**Priority**: Low

**Context**: Go 1.24 experimental garbage collector may improve performance.

**Tasks**:
- Create benchmark suite for memory-intensive operations
- Compare default GC vs greenteagc
- Document findings

---

## Issue #793 - Benchmark encoding/json/v2 when stable
**Status**: Todo
**Complexity**: Low
**Priority**: Low

**Context**: New JSON encoder in development may improve serialization performance.

**Tasks**:
- Monitor Go releases for json/v2 stability
- Create JSON benchmark suite
- Compare v1 vs v2 performance

---

## Issue #794 - Add Flight Recorder for daemon debugging
**Status**: Todo
**Complexity**: Medium
**Priority**: Medium

**Scope**:
- Ring buffer of recent daemon events
- Capture on error for diagnostics
- Export for bug reports

---

## Progress Tracking

### Completed
- (none yet)

### In Progress
- (none yet)
