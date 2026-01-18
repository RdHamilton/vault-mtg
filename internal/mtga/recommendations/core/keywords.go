package core

import (
	"strings"
)

// KeywordCategory represents the type of keyword extracted.
type KeywordCategory string

const (
	CategoryCombat     KeywordCategory = "combat"
	CategoryAbility    KeywordCategory = "ability"
	CategoryMechanic   KeywordCategory = "mechanic"
	CategoryTheme      KeywordCategory = "theme"
	CategoryTrigger    KeywordCategory = "trigger"
	CategoryActivated  KeywordCategory = "activated"
	CategoryProtection KeywordCategory = "protection"
)

// KeywordInfo contains information about an extracted keyword.
type KeywordInfo struct {
	Keyword  string
	Category KeywordCategory
	Weight   float64 // How important this keyword is for synergy (0.0-1.0)
}

// keywordDictionary maps keywords/patterns to their category and weight.
var keywordDictionary = map[string]KeywordInfo{
	// Combat keywords (high synergy weight)
	"flying":        {Keyword: "flying", Category: CategoryCombat, Weight: 0.8},
	"first strike":  {Keyword: "first strike", Category: CategoryCombat, Weight: 0.7},
	"double strike": {Keyword: "double strike", Category: CategoryCombat, Weight: 0.9},
	"deathtouch":    {Keyword: "deathtouch", Category: CategoryCombat, Weight: 0.8},
	"haste":         {Keyword: "haste", Category: CategoryCombat, Weight: 0.6},
	"lifelink":      {Keyword: "lifelink", Category: CategoryCombat, Weight: 0.7},
	"menace":        {Keyword: "menace", Category: CategoryCombat, Weight: 0.7},
	"reach":         {Keyword: "reach", Category: CategoryCombat, Weight: 0.5},
	"trample":       {Keyword: "trample", Category: CategoryCombat, Weight: 0.7},
	"vigilance":     {Keyword: "vigilance", Category: CategoryCombat, Weight: 0.6},

	// Protection/evasion keywords
	"hexproof":       {Keyword: "hexproof", Category: CategoryProtection, Weight: 0.8},
	"indestructible": {Keyword: "indestructible", Category: CategoryProtection, Weight: 0.9},
	"ward":           {Keyword: "ward", Category: CategoryProtection, Weight: 0.7},
	"shroud":         {Keyword: "shroud", Category: CategoryProtection, Weight: 0.7},
	"protection":     {Keyword: "protection", Category: CategoryProtection, Weight: 0.7},

	// Static ability keywords
	"flash":    {Keyword: "flash", Category: CategoryAbility, Weight: 0.6},
	"defender": {Keyword: "defender", Category: CategoryAbility, Weight: 0.3},
	"prowess":  {Keyword: "prowess", Category: CategoryAbility, Weight: 0.8},
	"convoke":  {Keyword: "convoke", Category: CategoryAbility, Weight: 0.7},

	// Set mechanics
	"flashback": {Keyword: "flashback", Category: CategoryMechanic, Weight: 0.8},
	"kicker":    {Keyword: "kicker", Category: CategoryMechanic, Weight: 0.6},
	"adventure": {Keyword: "adventure", Category: CategoryMechanic, Weight: 0.7},
	"transform": {Keyword: "transform", Category: CategoryMechanic, Weight: 0.6},
	"disturb":   {Keyword: "disturb", Category: CategoryMechanic, Weight: 0.7},
	"exploit":   {Keyword: "exploit", Category: CategoryMechanic, Weight: 0.8},
	"escape":    {Keyword: "escape", Category: CategoryMechanic, Weight: 0.8},
	"madness":   {Keyword: "madness", Category: CategoryMechanic, Weight: 0.7},
	"cycling":   {Keyword: "cycling", Category: CategoryMechanic, Weight: 0.6},
	"cascade":   {Keyword: "cascade", Category: CategoryMechanic, Weight: 0.9},
	"mutate":    {Keyword: "mutate", Category: CategoryMechanic, Weight: 0.8},
	"foretell":  {Keyword: "foretell", Category: CategoryMechanic, Weight: 0.7},
	"learn":     {Keyword: "learn", Category: CategoryMechanic, Weight: 0.6},
	"ninjutsu":  {Keyword: "ninjutsu", Category: CategoryMechanic, Weight: 0.8},
	"channel":   {Keyword: "channel", Category: CategoryMechanic, Weight: 0.6},
	"bargain":   {Keyword: "bargain", Category: CategoryMechanic, Weight: 0.7},
	"craft":     {Keyword: "craft", Category: CategoryMechanic, Weight: 0.7},
	"descend":   {Keyword: "descend", Category: CategoryMechanic, Weight: 0.7},
	"threshold": {Keyword: "threshold", Category: CategoryMechanic, Weight: 0.7},
	"delirium":  {Keyword: "delirium", Category: CategoryMechanic, Weight: 0.7},
	"affinity":  {Keyword: "affinity", Category: CategoryMechanic, Weight: 0.8},
	"modular":   {Keyword: "modular", Category: CategoryMechanic, Weight: 0.7},
	"equip":     {Keyword: "equip", Category: CategoryMechanic, Weight: 0.6},
	"crew":      {Keyword: "crew", Category: CategoryMechanic, Weight: 0.7},
	"offspring": {Keyword: "offspring", Category: CategoryMechanic, Weight: 0.7},
	"valiant":   {Keyword: "valiant", Category: CategoryMechanic, Weight: 0.7},
}

// themePatterns are patterns for identifying card themes.
var themePatterns = []struct {
	pattern  string
	keyword  string
	category KeywordCategory
	weight   float64
}{
	// Token generation
	{"create a token", "tokens", CategoryTheme, 0.9},
	{"creates a token", "tokens", CategoryTheme, 0.9},
	{"create two", "tokens", CategoryTheme, 0.9},
	{"create three", "tokens", CategoryTheme, 0.9},

	// Counter themes
	{"+1/+1 counter", "+1/+1 counters", CategoryTheme, 0.9},
	{"put a counter", "counters", CategoryTheme, 0.7},
	{"-1/-1 counter", "-1/-1 counters", CategoryTheme, 0.8},

	// Graveyard themes
	{"from your graveyard", "graveyard", CategoryTheme, 0.9},
	{"in your graveyard", "graveyard", CategoryTheme, 0.8},
	{"mill", "mill", CategoryTheme, 0.8},

	// Sacrifice themes
	{"sacrifice a", "sacrifice", CategoryTheme, 0.9},
	{"when.*dies", "death triggers", CategoryTheme, 0.8},
	{"whenever.*dies", "death triggers", CategoryTheme, 0.8},

	// Life themes
	{"gain life", "lifegain", CategoryTheme, 0.7},
	{"whenever you gain life", "lifegain payoff", CategoryTheme, 0.9},
	{"lose life", "drain", CategoryTheme, 0.7},

	// Draw/card advantage
	{"draw a card", "card draw", CategoryTheme, 0.7},
	{"draw cards", "card draw", CategoryTheme, 0.8},
	{"scry", "scry", CategoryTheme, 0.6},
	{"surveil", "surveil", CategoryTheme, 0.7},

	// Combat triggers
	{"deals combat damage", "combat damage", CategoryTheme, 0.8},
	{"whenever.*attacks", "attack triggers", CategoryTheme, 0.8},
	{"can't be blocked", "evasion", CategoryTheme, 0.8},

	// ETB triggers
	{"when.*enters the battlefield", "ETB", CategoryTrigger, 0.9},
	{"when.*enters", "enters triggers", CategoryTrigger, 0.8},

	// Spell themes
	{"whenever you cast a noncreature spell", "spells matter", CategoryTheme, 0.9},
	{"instant or sorcery", "spells matter", CategoryTheme, 0.8},
	{"magecraft", "spells matter", CategoryTheme, 0.9},

	// Artifact/enchantment themes
	{"artifact you control", "artifacts matter", CategoryTheme, 0.8},
	{"enchantment you control", "enchantments matter", CategoryTheme, 0.8},
	{"aura", "auras", CategoryTheme, 0.7},
	{"equipment", "equipment", CategoryTheme, 0.7},

	// Creature-specific themes
	{"creatures you control get", "anthem", CategoryTheme, 0.9},
	{"other creatures you control get", "anthem", CategoryTheme, 0.9},

	// Landfall
	{"whenever a land enters the battlefield under your control", "landfall", CategoryTheme, 0.9},
	{"landfall", "landfall", CategoryTheme, 0.9},

	// Mana themes
	{"search your library for a.*land", "land ramp", CategoryTheme, 0.8},
	{"add one mana", "mana ramp", CategoryTheme, 0.6},
	{"add two mana", "mana ramp", CategoryTheme, 0.7},
}

// ExtractKeywords extracts keywords from card oracle text.
func ExtractKeywords(text string) []string {
	keywords := extractKeywordsFromText(text)
	result := make([]string, 0, len(keywords))
	for kw := range keywords {
		result = append(result, kw)
	}
	return result
}

// extractKeywordsFromText extracts keywords from card text using pattern matching.
func extractKeywordsFromText(text string) map[string]bool {
	keywords := make(map[string]bool)
	lowerText := strings.ToLower(text)

	// Extract keywords from dictionary
	for key, info := range keywordDictionary {
		if strings.Contains(lowerText, key) {
			keywords[info.Keyword] = true
		}
	}

	// Extract theme patterns
	for _, pattern := range themePatterns {
		if containsPattern(lowerText, pattern.pattern) {
			keywords[pattern.keyword] = true
		}
	}

	return keywords
}

// ExtractKeywordsWithInfo extracts keywords with full category and weight information.
func ExtractKeywordsWithInfo(text string) []KeywordInfo {
	var result []KeywordInfo
	seen := make(map[string]bool)
	lowerText := strings.ToLower(text)

	// Extract keywords from dictionary
	for key, info := range keywordDictionary {
		if strings.Contains(lowerText, key) && !seen[info.Keyword] {
			result = append(result, info)
			seen[info.Keyword] = true
		}
	}

	// Extract theme patterns
	for _, pattern := range themePatterns {
		if containsPattern(lowerText, pattern.pattern) && !seen[pattern.keyword] {
			result = append(result, KeywordInfo{
				Keyword:  pattern.keyword,
				Category: pattern.category,
				Weight:   pattern.weight,
			})
			seen[pattern.keyword] = true
		}
	}

	return result
}

// GetKeywordWeight returns the synergy weight for a keyword.
func GetKeywordWeight(keyword string) float64 {
	lowerKeyword := strings.ToLower(keyword)

	if info, ok := keywordDictionary[lowerKeyword]; ok {
		return info.Weight
	}

	for _, pattern := range themePatterns {
		if pattern.keyword == keyword {
			return pattern.weight
		}
	}

	return 0.5 // Default weight
}

// GetKeywordCategory returns the category for a keyword.
func GetKeywordCategory(keyword string) KeywordCategory {
	lowerKeyword := strings.ToLower(keyword)

	if info, ok := keywordDictionary[lowerKeyword]; ok {
		return info.Category
	}

	for _, pattern := range themePatterns {
		if pattern.keyword == keyword {
			return pattern.category
		}
	}

	return CategoryAbility // Default category
}

// containsPattern performs simple pattern matching with .* wildcard support.
func containsPattern(text, pattern string) bool {
	if !strings.Contains(pattern, ".*") {
		return strings.Contains(text, pattern)
	}

	parts := strings.Split(pattern, ".*")
	if len(parts) != 2 {
		return strings.Contains(text, parts[0])
	}

	idx := strings.Index(text, parts[0])
	if idx == -1 {
		return false
	}

	remaining := text[idx+len(parts[0]):]
	return strings.Contains(remaining, parts[1])
}

// CalculateKeywordSynergy calculates synergy between two cards based on shared keywords.
func CalculateKeywordSynergy(card1Keywords, card2Keywords []KeywordInfo) float64 {
	if len(card1Keywords) == 0 || len(card2Keywords) == 0 {
		return 0.0
	}

	// Build lookup for card1 keywords by category
	card1ByCategory := make(map[KeywordCategory][]KeywordInfo)
	for _, kw := range card1Keywords {
		card1ByCategory[kw.Category] = append(card1ByCategory[kw.Category], kw)
	}

	totalSynergy := 0.0
	matchCount := 0

	for _, kw2 := range card2Keywords {
		// Check for exact keyword match
		for _, kw1 := range card1Keywords {
			if kw1.Keyword == kw2.Keyword {
				// Exact match - use average of weights
				totalSynergy += (kw1.Weight + kw2.Weight) / 2
				matchCount++
			}
		}

		// Check for same category match (weaker synergy)
		if categoryKws, ok := card1ByCategory[kw2.Category]; ok {
			for _, kw1 := range categoryKws {
				if kw1.Keyword != kw2.Keyword {
					// Same category but different keyword - partial synergy
					totalSynergy += (kw1.Weight + kw2.Weight) / 4
					matchCount++
				}
			}
		}
	}

	if matchCount == 0 {
		return 0.0
	}

	// Normalize by the number of possible matches
	maxMatches := len(card1Keywords) * len(card2Keywords)
	normalized := totalSynergy / float64(maxMatches)

	if normalized > 1.0 {
		return 1.0
	}
	return normalized
}
