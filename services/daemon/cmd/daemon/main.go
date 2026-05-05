// Command mtga-daemon watches MTGA Player.log and forwards events to the BFF.
// Configuration is loaded from a JSON file (default: %APPDATA%\mtga-companion\daemon.json
// on Windows; ~/.mtga-companion/daemon.json on macOS/Linux) and can be overridden with
// environment variables. The cloud API URL is never hardcoded.
//
// Environment variables:
//
//	MTGA_DAEMON_CLOUD_API_URL  Base URL of the cloud API / BFF (required if not in config file)
//	MTGA_DAEMON_API_KEY        Bearer token for BFF authentication
//	MTGA_DAEMON_LOG_PATH       Override MTGA log file path (auto-detected by default)
//	MTGA_DAEMON_ACCOUNT_ID     MTGA account ID to tag events
//
// Flags:
//
//	-config <path>   Path to JSON config file
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/ramonehamilton/mtga-daemon/internal/config"
	"github.com/ramonehamilton/mtga-daemon/internal/daemon"
)

// Version is the build-time version string injected via -ldflags -X main.Version=<ver>.
// Defaults to "dev" for local builds.
var Version = "dev"

func main() {
	defaultCfgPath := defaultConfigPath()
	cfgPath := flag.String("config", defaultCfgPath, "path to JSON config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	log.Printf("[mtga-daemon] version=%s", Version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	svc := daemon.New(cfg)
	log.Printf("[mtga-daemon] starting, cloud_api=%s", cfg.CloudAPIURL)

	if err := svc.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// defaultConfigPath returns the platform-appropriate default config path:
//   - Windows: %APPDATA%\mtga-companion\daemon.json
//   - macOS/Linux: ~/.mtga-companion/daemon.json
//
// The -config flag overrides this; Task Scheduler on Windows always passes
// -config explicitly, so the default is only used when running the binary
// directly without that flag.
func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "mtga-companion", "daemon.json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "daemon.json"
	}
	return filepath.Join(home, ".mtga-companion", "daemon.json")
}
