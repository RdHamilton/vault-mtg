// Package updatecheck polls the BFF for the latest daemon version.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/mod/semver"
)

// VersionResponse is the JSON body returned by GET /api/v1/daemon/version.
type VersionResponse struct {
	Latest      string `json:"latest"`
	ReleasedAt  string `json:"released_at"`
	DownloadURL string `json:"download_url"`
}

// Check fetches the latest daemon version from the BFF and logs a warning if
// a newer version is available. All errors are logged at INFO level and swallowed.
// If currentVersion is "dev", the check is skipped entirely.
func Check(ctx context.Context, baseURL string, currentVersion string) {
	if currentVersion == "dev" {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// baseURL is cfg.CloudAPIURL which already contains the /api/v1 prefix
	// (e.g. https://staging-api.vaultmtg.app/api/v1) — append only the
	// path segment, not a redundant /api/v1.
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"/daemon/version", nil)
	if err != nil {
		log.Printf("[updatecheck] failed to build request: %v", err)
		return
	}
	req.Header.Set("User-Agent", fmt.Sprintf("mtga-daemon/%s", currentVersion))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[updatecheck] version check failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[updatecheck] version check returned %d", resp.StatusCode)
		return
	}

	var vr VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		log.Printf("[updatecheck] failed to decode response: %v", err)
		return
	}

	// semver.Compare requires a "v" prefix.
	current := "v" + currentVersion
	latest := "v" + vr.Latest
	if semver.Compare(latest, current) > 0 {
		log.Printf("[mtga-daemon] WARN: new version available: %s (current: %s) — %s", vr.Latest, currentVersion, vr.DownloadURL)
	}
}
