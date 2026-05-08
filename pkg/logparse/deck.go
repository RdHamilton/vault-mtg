package logparse

import (
	"encoding/json"
	"strings"
	"time"
)

// DeckLibrary represents all player decks.
type DeckLibrary struct {
	Decks         map[string]*PlayerDeck
	TotalDecks    int
	DecksByFormat map[string][]*PlayerDeck
}

// PlayerDeck represents a single saved deck.
type PlayerDeck struct {
	DeckID      string
	Name        string
	Format      string
	Description string
	MainDeck    []DeckCard
	Sideboard   []DeckCard
	Created     time.Time
	Modified    time.Time
	LastPlayed  *time.Time
}

// ParseDecks extracts saved player decks from log entries.
func ParseDecks(entries []*LogEntry) (*DeckLibrary, error) {
	library := &DeckLibrary{
		Decks:         make(map[string]*PlayerDeck),
		DecksByFormat: make(map[string][]*PlayerDeck),
	}

	seenDecks := make(map[string]bool)

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsJSON {
			continue
		}

		deck := parseDeckUpsertV2(entry)
		if deck == nil || deck.DeckID == "" {
			continue
		}

		if seenDecks[deck.DeckID] {
			continue
		}
		seenDecks[deck.DeckID] = true

		library.Decks[deck.DeckID] = deck
		library.DecksByFormat[deck.Format] = append(library.DecksByFormat[deck.Format], deck)
	}

	library.TotalDecks = len(library.Decks)

	if library.TotalDecks == 0 {
		return nil, nil
	}

	return library, nil
}

// parseDeckUpsertV2 parses deck data from DeckUpsertDeckV2 log entries.
func parseDeckUpsertV2(entry *LogEntry) *PlayerDeck {
	requestStr, ok := entry.JSON["request"].(string)
	if !ok || requestStr == "" {
		return nil
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal([]byte(requestStr), &requestData); err != nil {
		return nil
	}

	summaryData, ok := requestData["Summary"].(map[string]interface{})
	if !ok {
		return nil
	}

	deck := &PlayerDeck{
		MainDeck:  []DeckCard{},
		Sideboard: []DeckCard{},
	}

	if id, ok := summaryData["DeckId"].(string); ok {
		deck.DeckID = id
	}
	if deck.DeckID == "" {
		return nil
	}

	if name, ok := summaryData["Name"].(string); ok {
		deck.Name = cleanDeckName(name)
	}

	if desc, ok := summaryData["Description"].(string); ok {
		deck.Description = desc
	}

	if attrsData, ok := summaryData["Attributes"].([]interface{}); ok {
		for _, attrData := range attrsData {
			attrMap, ok := attrData.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := attrMap["name"].(string)
			value, _ := attrMap["value"].(string)

			switch name {
			case "Format":
				deck.Format = value
			case "LastPlayed":
				if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
					value = value[1 : len(value)-1]
				}
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					deck.LastPlayed = &t
				}
			case "LastUpdated":
				if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
					value = value[1 : len(value)-1]
				}
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					deck.Modified = t
				}
			}
		}
	}

	if deck.Format == "" {
		deck.Format = "Unknown"
	}

	deckData, ok := requestData["Deck"].(map[string]interface{})
	if !ok {
		return deck
	}

	if mainDeckData, ok := deckData["MainDeck"].([]interface{}); ok {
		deck.MainDeck = parseDeckCards(mainDeckData)
	}

	if sideboardData, ok := deckData["Sideboard"].([]interface{}); ok {
		deck.Sideboard = parseDeckCards(sideboardData)
	}

	return deck
}

// parseDeckCards parses an array of card objects into DeckCard slice.
func parseDeckCards(cardsData []interface{}) []DeckCard {
	var cards []DeckCard

	for _, cardData := range cardsData {
		cardMap, ok := cardData.(map[string]interface{})
		if !ok {
			continue
		}

		card := DeckCard{}

		if cardID, ok := cardMap["cardId"].(float64); ok {
			card.CardID = int(cardID)
		} else if cardID, ok := cardMap["CardId"].(float64); ok {
			card.CardID = int(cardID)
		} else if cardID, ok := cardMap["card_id"].(float64); ok {
			card.CardID = int(cardID)
		}

		if quantity, ok := cardMap["quantity"].(float64); ok {
			card.Quantity = int(quantity)
		} else if quantity, ok := cardMap["Quantity"].(float64); ok {
			card.Quantity = int(quantity)
		}

		if card.CardID > 0 && card.Quantity > 0 {
			cards = append(cards, card)
		}
	}

	return cards
}

// cleanDeckName converts MTGA localization keys to readable deck names.
func cleanDeckName(name string) string {
	if !strings.HasPrefix(name, "?=?Loc/") {
		return name
	}

	lastSlash := strings.LastIndex(name, "/")
	if lastSlash == -1 || lastSlash >= len(name)-1 {
		return name
	}

	identifier := name[lastSlash+1:]
	return strings.ReplaceAll(identifier, "_", " ")
}
