package handlers

import (
	"encoding/json"
	"net/http"

	contract "github.com/RdHamilton/MTGA-Companion/services/contract"
	"github.com/ramonehamilton/mtga-bff/internal/config"
)

// DaemonVersionHandler serves GET /api/v1/daemon/version.
// No authentication is required — version metadata is public.
type DaemonVersionHandler struct {
	cfg *config.Config
}

// NewDaemonVersionHandler constructs a DaemonVersionHandler.
func NewDaemonVersionHandler(cfg *config.Config) *DaemonVersionHandler {
	return &DaemonVersionHandler{cfg: cfg}
}

// GetDaemonVersion handles GET /api/v1/daemon/version.
// Returns the latest published daemon version, its release timestamp, and a
// download URL pointing at the corresponding GitHub Releases tag.
func (h *DaemonVersionHandler) GetDaemonVersion(w http.ResponseWriter, r *http.Request) {
	resp := contract.DaemonVersionResponse{
		Latest:      h.cfg.DaemonLatestVersion,
		ReleasedAt:  h.cfg.DaemonReleasedAt,
		DownloadURL: "https://github.com/RdHamilton/MTGA-Companion/releases/tag/daemon/v" + h.cfg.DaemonLatestVersion,
		Changelog:   "",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Response headers already sent; log only.
		_ = err
	}
}
