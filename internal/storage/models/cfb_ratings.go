package models

import "time"

// CFB Constructed playability ratings.
const (
	CFBConstructedStaple     = "Staple"
	CFBConstructedPlayable   = "Playable"
	CFBConstructedFringe     = "Fringe"
	CFBConstructedUnplayable = "Unplayable"
)

// CFBRating represents a ChannelFireball card rating from a set review.
type CFBRating struct {
	ID       int64  `json:"id" db:"id"`
	CardName string `json:"cardName" db:"card_name"`
	SetCode  string `json:"setCode" db:"set_code"`
	ArenaID  *int   `json:"arenaId,omitempty" db:"arena_id"`

	// Limited rating on 0.0-5.0 scale (matching TCGPlayer/MTGAZone format)
	LimitedRating float64 `json:"limitedRating" db:"limited_rating"`
	// Normalized score (0.0-1.0) for internal calculations
	LimitedScore float64 `json:"limitedScore" db:"limited_score"`

	// Constructed rating (Staple, Playable, Fringe, Unplayable)
	ConstructedRating string  `json:"constructedRating" db:"constructed_rating"`
	ConstructedScore  float64 `json:"constructedScore" db:"constructed_score"`

	// Archetype fit (e.g., "Best in Aggro", "Flexible", "Control only")
	ArchetypeFit string `json:"archetypeFit,omitempty" db:"archetype_fit"`

	// Commentary from the CFB review
	Commentary string `json:"commentary,omitempty" db:"commentary"`

	// Source information
	SourceURL string `json:"sourceUrl,omitempty" db:"source_url"`
	Author    string `json:"author,omitempty" db:"author"`

	// Metadata
	ImportedAt time.Time `json:"importedAt" db:"imported_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}

// LimitedRatingToScore converts a 0-5 rating to a normalized 0-1 score.
func LimitedRatingToScore(rating float64) float64 {
	if rating < 0 {
		return 0
	}
	if rating > 5 {
		return 1
	}
	return rating / 5.0
}

// LimitedRatingToGrade converts a 0-5 numerical rating to a display letter grade.
// This is used for UI display purposes only.
func LimitedRatingToGrade(rating float64) string {
	switch {
	case rating >= 4.75:
		return "A+"
	case rating >= 4.25:
		return "A"
	case rating >= 3.75:
		return "A-"
	case rating >= 3.25:
		return "B+"
	case rating >= 2.75:
		return "B"
	case rating >= 2.25:
		return "B-"
	case rating >= 1.75:
		return "C+"
	case rating >= 1.25:
		return "C"
	case rating >= 0.75:
		return "C-"
	case rating >= 0.25:
		return "D"
	default:
		return "F"
	}
}

// ConstructedRatingToScore converts a constructed playability rating to a numeric score (0.0-1.0).
func ConstructedRatingToScore(rating string) float64 {
	scores := map[string]float64{
		CFBConstructedStaple:     1.00,
		CFBConstructedPlayable:   0.70,
		CFBConstructedFringe:     0.40,
		CFBConstructedUnplayable: 0.10,
	}
	if score, ok := scores[rating]; ok {
		return score
	}
	return 0.5 // Default for unknown ratings
}

// CFBRatingImport represents the structure for importing CFB ratings from JSON.
type CFBRatingImport struct {
	CardName          string  `json:"card_name"`
	SetCode           string  `json:"set_code"`
	LimitedRating     float64 `json:"limited_rating"`
	ConstructedRating string  `json:"constructed_rating,omitempty"`
	ArchetypeFit      string  `json:"archetype_fit,omitempty"`
	Commentary        string  `json:"commentary,omitempty"`
	SourceURL         string  `json:"source_url,omitempty"`
	Author            string  `json:"author,omitempty"`
}
