package contract

// DaemonVersionResponse is the wire type returned by GET /api/v1/daemon/version.
// The endpoint requires no authentication; version metadata is public.
type DaemonVersionResponse struct {
	Latest      string `json:"latest"`
	ReleasedAt  string `json:"released_at"`
	DownloadURL string `json:"download_url"`
	Changelog   string `json:"changelog"`
}
