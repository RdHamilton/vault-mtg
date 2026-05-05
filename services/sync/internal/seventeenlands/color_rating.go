package seventeenlands

// ColorRating holds per-color-combination win-rate statistics returned by the
// 17Lands color ratings API (/color_ratings/data).
//
// The API returns one entry per color combination played in the draft format
// (e.g. "WU", "BRG").  win_rate and games_played map directly to the
// draft_color_ratings table columns of the same name.
type ColorRating struct {
	ColorCombination string  `json:"color_name"`
	WinRate          float64 `json:"win_rate"`
	GamesPlayed      int     `json:"games"`
}
