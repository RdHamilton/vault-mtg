package seventeenlands

// CardRating holds per-card statistics returned by the 17Lands card ratings API.
type CardRating struct {
	MtgaID    int     `json:"mtga_id"`
	Name      string  `json:"name"`
	ALSA      float64 `json:"avg_seen"`
	ATA       float64 `json:"avg_pick"`
	GIHWR     float64 `json:"ever_drawn_win_rate"`
	OHW       float64 `json:"opening_hand_win_rate"`
	GDWR      float64 `json:"drawn_improvement_win_rate"`
	SeenCount int     `json:"seen_count"`
	PickCount int     `json:"pick_count"`
}

// ColorRating holds per-color-combination win-rate data returned by the
// 17Lands /color_ratings/data endpoint.
type ColorRating struct {
	ColorCombination string  `json:"color_combination"`
	WinRate          float64 `json:"win_rate"`
	GamesPlayed      int     `json:"games"`
}
