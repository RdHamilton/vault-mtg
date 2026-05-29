//go:build darwin

// Command collection-helper runs as a root launchd daemon and exposes a Unix
// socket at /tmp/com.vaultmtg.collection-helper.sock. The VaultMTG daemon
// connects to this socket to request a collection scan. The helper calls
// task_for_pid against the running MTGA process and returns the card inventory
// as JSON.
//
// Installation (performed by the tray "Grant Access" flow):
//
//	sudo cp collection-helper /Library/Application\ Support/VaultMTG/
//	sudo cp com.vaultmtg.collection-helper.plist /Library/LaunchDaemons/
//	sudo launchctl load /Library/LaunchDaemons/com.vaultmtg.collection-helper.plist
package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "collection-helper must run as root")
		os.Exit(1)
	}

	log.SetPrefix("[collection-helper] ")
	log.SetFlags(log.Ldate | log.Ltime)

	log.Printf("starting (pid=%d)", os.Getpid())
	// Emit the active signature version at startup so CloudWatch / on-call triage
	// can correlate a COLLECTION_SCAN_DRIFT alarm with the known-good signature.
	// mtga_build=unknown until v0.3.5 adds Info.plist detection (ADR-040 §G4).
	log.Printf("signature_version=%s mtga_build=unknown note=%q",
		CollectionSignatureVersion, knownSignatureVersions[CollectionSignatureVersion])
	if err := runServer(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
