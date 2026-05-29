package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

// SocketPath is the Unix domain socket the helper listens on.
// Permissions are 0666 so the unprivileged daemon process can connect;
// the helper itself validates every request and runs the scan as root.
const SocketPath = "/tmp/com.vaultmtg.collection-helper.sock"

// ScanRequest is sent by the daemon to request a collection scan.
type ScanRequest struct {
	PID int `json:"pid"`
}

// ScanResponse is returned by the helper after scanning.
type ScanResponse struct {
	Cards      map[int]int `json:"cards"` // grp_id → quantity
	CapturedAt time.Time   `json:"captured_at"`
	Error      string      `json:"error,omitempty"`
}

func runServer() error {
	if err := os.Remove(SocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	// Strip only execute bits so the socket is created 0666 (world r/w).
	// The unprivileged daemon must be able to connect; the helper validates
	// every request and performs the privileged scan itself.
	old := syscall.Umask(0o111)
	ln, err := net.Listen("unix", SocketPath)
	syscall.Umask(old)
	if err != nil {
		return fmt.Errorf("listen %s: %w", SocketPath, err)
	}
	defer func() { _ = ln.Close() }()

	if err := os.Chmod(SocketPath, 0o666); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}

	log.Printf("[helper] listening on %s", SocketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	var req ScanRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeError(conn, fmt.Sprintf("decode request: %v", err))
		return
	}

	if req.PID <= 0 {
		writeError(conn, "invalid pid")
		return
	}
	// Verify the process exists before attempting a privileged memory scan.
	p, _ := os.FindProcess(req.PID)
	if err := p.Signal(syscall.Signal(0)); err != nil {
		writeError(conn, fmt.Sprintf("PID %d not found", req.PID))
		return
	}

	log.Printf("[helper] scan requested for PID %d", req.PID)

	cards, err := scanProcess(req.PID)
	if err != nil {
		// COLLECTION_SCAN_DRIFT is the canary trigger string. The T3 CloudWatch log
		// filter (Ray's infra ticket) pattern-matches on this exact string. Do not
		// rename it without updating the alarm filter.
		log.Printf("COLLECTION_SCAN_DRIFT signature_version=%s pid=%d error=%v",
			CollectionSignatureVersion, req.PID, err)
		writeError(conn, err.Error())
		return
	}

	resp := ScanResponse{
		Cards:      cards,
		CapturedAt: time.Now().UTC(),
	}
	if encErr := json.NewEncoder(conn).Encode(resp); encErr != nil {
		log.Printf("[helper] encode response: %v", encErr)
	}
	log.Printf("[helper] scan complete: %d unique cards", len(cards))
}

func writeError(conn net.Conn, msg string) {
	resp := ScanResponse{Error: msg}
	_ = json.NewEncoder(conn).Encode(resp)
}
