// Package mtgazone provides a scraper for MTG Arena Zone limited set reviews.
package mtgazone

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

const (
	// BaseURL is the base URL for MTG Arena Zone.
	BaseURL = "https://mtgazone.com"

	// DefaultTimeout for HTTP requests.
	DefaultTimeout = 30 * time.Second

	// DefaultRateLimit is conservative: 2 requests per second (500ms) to be respectful.
	DefaultRateLimit = 500 * time.Millisecond

	// maxResponseSize limits response body to prevent memory exhaustion (10MB).
	maxResponseSize = 10 * 1024 * 1024
)

// Colors represents the color categories in set reviews.
var Colors = []string{"white", "blue", "black", "red", "green", "multicolor", "artifacts-lands"}

// CardRating represents a card rating scraped from MTG Arena Zone.
type CardRating struct {
	CardName string
	Rating   float64 // 0.0-5.0 scale
	Color    string  // white, blue, black, red, green, multicolor, colorless
}

// Scraper fetches and parses MTG Arena Zone set reviews.
type Scraper struct {
	httpClient  *http.Client
	limiter     *rate.Limiter
	setMappings map[string]string // Maps set codes to URL slugs
	mu          sync.RWMutex
}

// ScraperOptions configures the scraper.
type ScraperOptions struct {
	// Timeout for HTTP requests.
	Timeout time.Duration

	// RateLimit controls request frequency.
	RateLimit time.Duration

	// HTTPClient allows custom HTTP client.
	HTTPClient *http.Client
}

// DefaultScraperOptions returns default options.
func DefaultScraperOptions() ScraperOptions {
	return ScraperOptions{
		Timeout:   DefaultTimeout,
		RateLimit: DefaultRateLimit,
	}
}

// NewScraper creates a new MTG Arena Zone scraper.
func NewScraper(options ScraperOptions) *Scraper {
	if options.Timeout == 0 {
		options.Timeout = DefaultTimeout
	}
	if options.RateLimit == 0 {
		options.RateLimit = DefaultRateLimit
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: options.Timeout,
		}
	}

	return &Scraper{
		httpClient:  httpClient,
		limiter:     rate.NewLimiter(rate.Every(options.RateLimit), 1),
		setMappings: buildSetMappings(),
	}
}

// buildSetMappings creates a map of set codes to URL slugs.
// MTG Arena Zone uses different URL patterns for different sets.
func buildSetMappings() map[string]string {
	return map[string]string{
		// 2024-2025 sets
		"TLA": "avatar-the-last-airbender-tla",
		"OM1": "through-the-omenpaths-om1",
		"EOE": "edge-of-eternity-eoe",
		"FIN": "final-fantasy-fin",
		"TDM": "tarkir-dragonstorm-tdm",
		"DFT": "aetherdrift-dft",
		"FDN": "foundations-fdn",
		"DSK": "duskmourn-dsk",
		"BLB": "bloomburrow-blb",
		"MH3": "modern-horizons-3-mh3",
		"OTJ": "outlaws-of-thunder-junction-otj",
		"MKM": "murders-at-karlov-manor-mkm",
		"KTK": "khans-of-tarkir-ktk",
		"LCI": "the-lost-caverns-of-ixalan-lci",
		"WOE": "wilds-of-eldraine-woe",
		"LTR": "the-lord-of-the-rings-tales-of-middle-earth-ltr",
		"MOM": "march-of-the-machine-mom",
		"ONE": "phyrexia-all-will-be-one-one",
		"BRO": "the-brothers-war-bro",
		"DMU": "dominaria-united-dmu",
		"SNC": "streets-of-new-capenna-snc",
		"NEO": "kamigawa-neon-dynasty-neo",
		"VOW": "innistrad-crimson-vow-vow",
		"MID": "innistrad-midnight-hunt-mid",
		"AFR": "dnd-afr", // D&D: Adventures in the Forgotten Realms
		"STX": "strixhaven-stx",
		"KHM": "kaldheim-khm",
		"ZNR": "zendikar-rising-znr",
		"IKO": "ikoria-lair-of-behemoths",
		"THB": "theros-beyond-death",
	}
}

// GetSetRatings fetches all card ratings for a set.
func (s *Scraper) GetSetRatings(ctx context.Context, setCode string) ([]CardRating, error) {
	setCode = strings.ToUpper(setCode)

	slug, ok := s.GetSetMapping(setCode)
	if !ok {
		// Try constructing a default slug
		slug = strings.ToLower(setCode)
		log.Printf("[MTGAZone] No mapping for set %s, trying default slug: %s", setCode, slug)
	}

	var allRatings []CardRating
	var errors []string

	for _, color := range Colors {
		// Rate limit
		if err := s.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter error: %w", err)
		}

		ratings, err := s.fetchColorReview(ctx, slug, color)
		if err != nil {
			// Log but continue - some colors might not have reviews yet
			log.Printf("[MTGAZone] Failed to fetch %s %s: %v", setCode, color, err)
			errors = append(errors, fmt.Sprintf("%s: %v", color, err))
			continue
		}

		allRatings = append(allRatings, ratings...)
		log.Printf("[MTGAZone] Fetched %d %s cards for %s", len(ratings), color, setCode)
	}

	if len(allRatings) == 0 && len(errors) > 0 {
		return nil, fmt.Errorf("failed to fetch any ratings for %s: %v", setCode, errors)
	}

	log.Printf("[MTGAZone] Total: fetched %d cards for %s", len(allRatings), setCode)
	return allRatings, nil
}

// fetchColorReview fetches ratings for a specific color.
func (s *Scraper) fetchColorReview(ctx context.Context, setSlug, color string) ([]CardRating, error) {
	// Construct URL
	url := fmt.Sprintf("%s/%s-limited-set-review-%s/", BaseURL, setSlug, color)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("review not found (404)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return s.parseRatings(string(body), color)
}

// parseRatings extracts card ratings from HTML content.
func (s *Scraper) parseRatings(htmlContent, color string) ([]CardRating, error) {
	var ratings []CardRating

	// MTG Arena Zone uses various patterns for ratings
	// Pattern 1: "Card Name: X.X" or "Card Name – X.X"
	// Pattern 2: Headers with card names followed by rating text
	// Pattern 3: Rating in format "X.X/5" or "X.X // 5"

	// First, let's try to parse using HTML structure
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract text content and look for rating patterns
	textContent := extractTextContent(doc)

	// Look for card name + rating patterns
	ratings = append(ratings, extractRatingsFromText(textContent, color)...)

	// Also try to find structured card data
	ratings = append(ratings, extractStructuredRatings(doc, color)...)

	// Deduplicate
	ratings = deduplicateRatings(ratings)

	return ratings, nil
}

// extractTextContent extracts all text from HTML.
func extractTextContent(n *html.Node) string {
	var sb strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
			sb.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return sb.String()
}

// extractRatingsFromText uses regex to find card names and ratings in text.
func extractRatingsFromText(text, color string) []CardRating {
	var ratings []CardRating

	// Pattern: "Card Name: X.X" or "Card Name – X.X/5" or "Card Name // X.X"
	// Card names typically start with capital letters and may contain commas, apostrophes
	patterns := []*regexp.Regexp{
		// "Card Name: 3.5" or "Card Name: 3.5/5"
		regexp.MustCompile(`(?i)([A-Z][A-Za-z',\s\-]+?):\s*(\d+\.?\d*)\s*(?:/\s*5)?`),
		// "Card Name – 3.5" (en-dash)
		regexp.MustCompile(`(?i)([A-Z][A-Za-z',\s\-]+?)\s*[–—]\s*(\d+\.?\d*)\s*(?:/\s*5)?`),
		// "Rating: 3.5" after card name in header
		regexp.MustCompile(`(?i)([A-Z][A-Za-z',\s\-]{2,30})\s+Rating:\s*(\d+\.?\d*)`),
	}

	seen := make(map[string]bool)

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				cardName := strings.TrimSpace(match[1])
				ratingStr := match[2]

				// Skip common false positives
				if isCommonWord(cardName) {
					continue
				}

				rating, err := strconv.ParseFloat(ratingStr, 64)
				if err != nil || rating < 0 || rating > 5 {
					continue
				}

				// Normalize card name
				cardName = normalizeCardName(cardName)
				if cardName == "" || seen[cardName] {
					continue
				}
				seen[cardName] = true

				ratings = append(ratings, CardRating{
					CardName: cardName,
					Rating:   rating,
					Color:    color,
				})
			}
		}
	}

	return ratings
}

// extractStructuredRatings looks for card data in HTML structure.
func extractStructuredRatings(doc *html.Node, color string) []CardRating {
	var ratings []CardRating

	// Look for card tooltip elements which often contain card names
	var findCards func(*html.Node)
	findCards = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Look for elements with card-related classes or data attributes
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "card") {
					// Found a card element, try to extract name
					cardName := getCardNameFromElement(n)
					if cardName != "" {
						// Try to find associated rating
						rating := findNearbyRating(n)
						if rating >= 0 && rating <= 5 {
							ratings = append(ratings, CardRating{
								CardName: cardName,
								Rating:   rating,
								Color:    color,
							})
						}
					}
				}
				if attr.Key == "data-cimg" || attr.Key == "data-card" {
					// Card name might be in data attribute
					cardName := normalizeCardName(attr.Val)
					if cardName != "" {
						rating := findNearbyRating(n)
						if rating >= 0 && rating <= 5 {
							ratings = append(ratings, CardRating{
								CardName: cardName,
								Rating:   rating,
								Color:    color,
							})
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findCards(c)
		}
	}
	findCards(doc)

	return ratings
}

// getCardNameFromElement extracts card name from an HTML element.
func getCardNameFromElement(n *html.Node) string {
	// Check data attributes first
	for _, attr := range n.Attr {
		if attr.Key == "data-cimg" || attr.Key == "data-card" || attr.Key == "title" {
			name := normalizeCardName(attr.Val)
			if name != "" {
				return name
			}
		}
	}

	// Check text content
	text := extractTextContent(n)
	text = strings.TrimSpace(text)
	if len(text) > 0 && len(text) < 50 {
		return normalizeCardName(text)
	}

	return ""
}

// findNearbyRating looks for a rating value near an element.
func findNearbyRating(n *html.Node) float64 {
	// Get parent and siblings' text
	if n.Parent != nil {
		text := extractTextContent(n.Parent)
		rating := extractFirstRating(text)
		if rating >= 0 {
			return rating
		}
	}
	return -1
}

// extractFirstRating finds the first valid rating in text.
func extractFirstRating(text string) float64 {
	pattern := regexp.MustCompile(`(\d+\.?\d*)\s*(?:/\s*5)?`)
	matches := pattern.FindStringSubmatch(text)
	if len(matches) >= 2 {
		rating, err := strconv.ParseFloat(matches[1], 64)
		if err == nil && rating >= 0 && rating <= 5 {
			return rating
		}
	}
	return -1
}

// normalizeCardName cleans up a card name.
func normalizeCardName(name string) string {
	// Remove extra whitespace
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	// Remove common artifacts
	name = strings.TrimPrefix(name, "//")
	name = strings.TrimSuffix(name, "//")

	// Handle double-faced cards - take the front face
	if strings.Contains(name, "//") {
		parts := strings.SplitN(name, "//", 2)
		name = strings.TrimSpace(parts[0])
	}

	// Skip if too short or too long
	if len(name) < 2 || len(name) > 50 {
		return ""
	}

	return name
}

// isCommonWord returns true if the string is a common word, not a card name.
func isCommonWord(s string) bool {
	common := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"has": true, "his": true, "how": true, "its": true, "may": true,
		"new": true, "now": true, "old": true, "see": true, "way": true,
		"who": true, "did": true, "get": true, "let": true, "put": true,
		"say": true, "too": true, "use": true, "rating": true, "score": true,
		"card": true, "limited": true, "draft": true, "pick": true,
		"white": true, "blue": true, "black": true, "red": true, "green": true,
		"colorless": true, "multicolor": true, "artifact": true, "artifacts": true,
		"introduction": true, "review": true, "conclusion": true,
	}
	return common[strings.ToLower(s)]
}

// deduplicateRatings removes duplicate card ratings.
func deduplicateRatings(ratings []CardRating) []CardRating {
	seen := make(map[string]bool)
	var result []CardRating

	for _, r := range ratings {
		key := strings.ToLower(r.CardName)
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}

	return result
}

// AddSetMapping adds or updates a set code to URL slug mapping.
func (s *Scraper) AddSetMapping(setCode, slug string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setMappings[strings.ToUpper(setCode)] = slug
}

// GetSetMapping returns the URL slug for a set code.
func (s *Scraper) GetSetMapping(setCode string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slug, ok := s.setMappings[strings.ToUpper(setCode)]
	return slug, ok
}
