package seventeenlands

// ColorRating holds per-color-combination statistics returned by the 17Lands
// /color_ratings/data endpoint.
//
// The API returns raw wins/games integers, not a pre-computed win rate. The
// endpoint also includes summary rows (is_summary=true) with an integer short_name
// that callers must filter out before persisting. See WinRate() for the computed
// ratio and the plan on vault-mtg-tickets#46 for the full root-cause analysis.
type ColorRating struct {
	ColorName string `json:"color_name"` // e.g. "Mono-White"
	ShortName string `json:"short_name"` // e.g. "W", "WU" — canonical MTG color key
	Wins      int    `json:"wins"`
	Games     int    `json:"games"`
	IsSummary bool   `json:"is_summary"`
}

// WinRate returns the computed win rate (wins/games). Returns 0 when Games == 0
// to avoid division by zero.
func (c ColorRating) WinRate() float64 {
	if c.Games == 0 {
		return 0
	}
	return float64(c.Wins) / float64(c.Games)
}
