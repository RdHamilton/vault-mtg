package collectionclient

import (
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempSocketPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "test-helper-*.sock")
	require.NoError(t, err)
	_ = f.Close()
	_ = os.Remove(f.Name())
	return f.Name()
}

func startFakeHelper(t *testing.T, sockPath string, resp ScanResponse) {
	t.Helper()
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(sockPath) })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		var req ScanRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("startFakeHelper: decode request: %v", err)
			return
		}
		if err := json.NewEncoder(conn).Encode(resp); err != nil {
			t.Errorf("startFakeHelper: encode response: %v", err)
		}
	}()
}

func TestClient_Scan_Success(t *testing.T) {
	sockPath := tempSocketPath(t)
	expected := ScanResponse{
		Cards:      map[int]int{96804: 3, 96580: 1},
		CapturedAt: time.Now().UTC(),
	}
	startFakeHelper(t, sockPath, expected)

	c := &Client{socketPath: sockPath, timeout: 5 * time.Second}
	resp, err := c.Scan(12345)
	require.NoError(t, err)
	assert.Equal(t, expected.Cards, resp.Cards)
	assert.Equal(t, 2, len(resp.Cards))
}

func TestClient_Scan_HelperError(t *testing.T) {
	sockPath := tempSocketPath(t)
	startFakeHelper(t, sockPath, ScanResponse{Error: "no collection region found"})

	c := &Client{socketPath: sockPath, timeout: 5 * time.Second}
	_, err := c.Scan(12345)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no collection region found")
}

func TestClient_Scan_NoHelper(t *testing.T) {
	c := &Client{socketPath: "/tmp/nonexistent-vaultmtg.sock", timeout: time.Second}
	_, err := c.Scan(12345)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connect to helper")
}

func TestClient_IsHelperRunning_False(t *testing.T) {
	c := &Client{socketPath: "/tmp/nonexistent-vaultmtg.sock", timeout: time.Second}
	assert.False(t, c.IsHelperRunning())
}

func TestClient_IsHelperRunning_True(t *testing.T) {
	sockPath := tempSocketPath(t)
	ln, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(sockPath) })

	c := &Client{socketPath: sockPath, timeout: time.Second}
	assert.True(t, c.IsHelperRunning())
}
