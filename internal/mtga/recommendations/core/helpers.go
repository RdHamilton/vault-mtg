package core

import (
	"strings"
)

// containsTypeInTypeLine checks if a card's TypeLine contains a specific type.
func containsTypeInTypeLine(typeLine, targetType string) bool {
	return strings.Contains(strings.ToLower(typeLine), strings.ToLower(targetType))
}

// extractCreatureTypes extracts creature types from a type line.
func extractCreatureTypes(typeLine string) []string {
	types := []string{}

	// Type line format: "Creature — Human Warrior" or "Legendary Creature — Elf Wizard"
	parts := strings.Split(typeLine, "—")
	if len(parts) < 2 {
		parts = strings.Split(typeLine, "-") // Try single dash
	}

	if len(parts) >= 2 {
		// Second part contains creature types
		typesPart := strings.TrimSpace(parts[1])
		individualTypes := strings.Fields(typesPart)
		types = append(types, individualTypes...)
	}

	return types
}
