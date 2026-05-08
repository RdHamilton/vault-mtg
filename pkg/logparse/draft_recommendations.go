package logparse

// DraftRecommendation represents a recommended pick for a draft.
type DraftRecommendation struct {
	CardID    string // Arena 2026.58+: string card ID
	Priority  int    // 1-5, where 5 is highest priority
	Reason    string // Explanation for the recommendation
	Archetype string // Suggested archetype
}

// DraftRecommendations represents recommendations for a draft pick.
type DraftRecommendations struct {
	TopPicks     []DraftRecommendation // Top 3 recommended picks
	Alternatives []DraftRecommendation // Alternative picks for different archetypes
}

// GetDraftRecommendations provides basic draft recommendations based on pack contents and previous picks.
func GetDraftRecommendations(packCards []string, previousPicks []DraftPick) DraftRecommendations {
	recommendations := DraftRecommendations{
		TopPicks:     []DraftRecommendation{},
		Alternatives: []DraftRecommendation{},
	}

	if len(packCards) == 0 {
		return recommendations
	}

	for i, cardID := range packCards {
		if i >= 3 {
			break
		}

		priority := 5 - i
		reason := "Basic recommendation"
		if i == 0 {
			reason = "First pick in pack"
		}

		recommendation := DraftRecommendation{
			CardID:   cardID,
			Priority: priority,
			Reason:   reason,
		}

		if i < 3 {
			recommendations.TopPicks = append(recommendations.TopPicks, recommendation)
		} else {
			recommendations.Alternatives = append(recommendations.Alternatives, recommendation)
		}
	}

	return recommendations
}
