// Package collectionclient communicates with the VaultMTG collection helper
// daemon over a Unix socket. The helper runs as root and performs the
// task_for_pid memory scan; this client runs as the unprivileged user.
package collectionclient

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// SocketPath must match the path in the helper binary.
const SocketPath = "/tmp/com.vaultmtg.collection-helper.sock"

// ScanRequest is sent to the helper.
type ScanRequest struct {
	PID int `json:"pid"`
}

// ScanResponse is returned by the helper.
type ScanResponse struct {
	Cards      map[int]int `json:"cards"`
	CapturedAt time.Time   `json:"captured_at"`
	Error      string      `json:"error,omitempty"`
}

// Client connects to the collection helper socket.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// New returns a Client using the default socket path.
func New() *Client {
	return &Client{socketPath: SocketPath, timeout: 10 * time.Second}
}

// IsHelperRunning returns true if the helper socket is present and accepts connections.
func (c *Client) IsHelperRunning() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Scan requests a collection scan for the given MTGA PID.
// Returns the card map (grp_id → quantity) and the time the scan completed.
func (c *Client) Scan(pid int) (*ScanResponse, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to helper: %w (is the helper installed?)", err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(c.timeout))

	req := ScanRequest{PID: pid}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send scan request: %w", err)
	}

	var resp ScanResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("helper error: %s", resp.Error)
	}
	return &resp, nil
}
