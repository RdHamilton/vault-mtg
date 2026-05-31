package logreader

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// DraftPackPayload is the parsed/emitted payload for a "draft.pack" event.
//
// It is the common output shape both draft formats normalise into: Premier
// (ParsePremierDraftNotify, #338) and BotDraft / QuickDraft
// (ParseBotDraftStatusPack, #337). The JSON tags below describe the daemon-emit
// (BFF contract) serialisation, not the raw MTGA wire line — the two formats
// have distinct wire shapes (see the parser sources).
//
// Emitted shape:
//
//	{"CourseName":"QuickDraft_SOS_20260526","draftPack":{"PackCards":[102470,102645],"SelfPick":1}}
type DraftPackPayload struct {
	// CourseName is the event identifier, e.g. "PremierDraft_BLB".
	//
	// NOTE: CourseName is EMPTY for Premier drafts — the real Draft.Notify
	// wire line carries no CourseName, only draftId. The draftstate session
	// key falls back to DraftID when CourseName is empty (see #338). #337's
	// BotDraft path DOES populate CourseName, so do not assume it is always set.
	CourseName string `json:"CourseName"`
	// DraftPack holds the offered card list and pick position.
	DraftPack DraftPackDetail `json:"draftPack"`
	// DraftID is the stable per-draft UUID. Premier drafts (Draft.Notify) set
	// it; BotDraft (#337) leaves it empty. Additive, non-breaking BFF field.
	DraftID string `json:"draft_id,omitempty"`
}

// DraftPackDetail contains the cards in the pack and the current pick index.
type DraftPackDetail struct {
	// PackCards is the list of Arena grpIds available in this pack.
	PackCards []int `json:"PackCards"`
	// SelfPick is the 1-based pick number within the pack.
	SelfPick int `json:"SelfPick"`
}

// DraftPickPayload is the parsed/emitted payload for a "draft.pick" event.
//
// It is the common output shape both draft formats normalise into: Premier
// (ParsePremierDraftMakePick, #338) and BotDraft / QuickDraft
// (ParseBotDraftPick, #337). The JSON tags below describe the daemon-emit (BFF
// contract) serialisation, not the raw MTGA wire line.
//
// Emitted shape:
//
//	{"CourseName":"QuickDraft_SOS_20260526","pickedCards":[102704],"PackNumber":0,"PickNumber":0}
type DraftPickPayload struct {
	// CourseName is the event identifier, e.g. "PremierDraft_BLB".
	//
	// NOTE: CourseName is EMPTY for Premier drafts — EventPlayerDraftMakePick
	// carries no CourseName, only DraftId. See DraftPackPayload.CourseName.
	CourseName string `json:"CourseName"`
	// PickedCards is the list of grpIds the player picked (usually one card).
	PickedCards []int `json:"pickedCards"`
	// PackNumber is the 0-based pack number (0 = pack 1, 1 = pack 2, 2 = pack 3).
	PackNumber int `json:"PackNumber"`
	// PickNumber is the 0-based pick index within the pack.
	PickNumber int `json:"PickNumber"`
	// DraftID is the stable per-draft UUID. Premier drafts set it; BotDraft
	// (#337) leaves it empty. Additive, non-breaking BFF field.
	DraftID string `json:"draft_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Premier draft (#338) — the wire format MTGA actually emits for human-pod
// (Premier / Traditional) drafts in 2026.59.20. The old draftPack/pickedCards
// keys above belong exclusively to the BotDraft format (#337).
// ---------------------------------------------------------------------------

// PremierDraftNotifyPayload is the parsed Draft.Notify line (Premier draft pack).
//
// Wire format (line prefixed "[UnityCrossThreadLogger]Draft.Notify "):
//
//	{"draftId":"<uuid>","SelfPick":1,"SelfPack":1,"PackCards":"102614,102609,..."}
//
// PackCards is a COMMA-SEPARATED STRING of grpIds, NOT a JSON array. SelfPick
// is the 1-based WITHIN-PACK pick (resets to 1 on each new pack). SelfPack is
// the 1-based pack number (1, 2, or 3).
type PremierDraftNotifyPayload struct {
	DraftID   string `json:"draftId"`
	SelfPack  int    `json:"SelfPack"`
	SelfPick  int    `json:"SelfPick"`
	PackCards string `json:"PackCards"`
}

// PremierDraftMakePickRequest is the inner request JSON string nested inside an
// EventPlayerDraftMakePick line under the "request" key (double-unmarshal).
type PremierDraftMakePickRequest struct {
	DraftID string `json:"DraftId"`
	GrpIDs  []int  `json:"GrpIds"`
	Pack    int    `json:"Pack"`
	Pick    int    `json:"Pick"`
}

// parsePackCards splits the comma-separated PackCards string into a slice of
// grpIds. An empty string yields an empty slice (NOT [""]→error), guarding the
// 0-card edge case that strings.Split would otherwise return as []string{""}.
func parsePackCards(s string) ([]int, error) {
	if s == "" {
		return []int{}, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("parse PackCards grpId %q: %w", p, err)
		}
		out = append(out, id)
	}
	return out, nil
}

// ParsePremierDraftNotify parses a Premier Draft.Notify entry into the existing
// DraftPackPayload (zero BFF-contract change). The comma-string PackCards is
// split into a slice and the within-pack SelfPick is reconstructed into the
// cumulative 1-based index the draftstate Store expects:
//
//	cumulative_1based = (SelfPack-1)*15 + SelfPick
//
// CourseName is left empty — Draft.Notify carries none; DraftID is the session
// key (see draftstate.sessionKey).
func ParsePremierDraftNotify(entry *LogEntry) (*DraftPackPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	if _, ok := entry.JSON["draftId"]; !ok {
		return nil, fmt.Errorf("entry does not contain draftId key")
	}
	if _, ok := entry.JSON["PackCards"]; !ok {
		return nil, fmt.Errorf("entry does not contain PackCards key")
	}

	raw, err := json.Marshal(entry.JSON)
	if err != nil {
		return nil, fmt.Errorf("re-marshal entry JSON: %w", err)
	}

	var n PremierDraftNotifyPayload
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("unmarshal PremierDraftNotifyPayload: %w", err)
	}
	if n.DraftID == "" {
		return nil, fmt.Errorf("Draft.Notify missing draftId")
	}

	cards, err := parsePackCards(n.PackCards)
	if err != nil {
		return nil, err
	}

	cumulative := (n.SelfPack-1)*15 + n.SelfPick

	return &DraftPackPayload{
		CourseName: "",
		DraftID:    n.DraftID,
		DraftPack: DraftPackDetail{
			PackCards: cards,
			SelfPick:  cumulative,
		},
	}, nil
}

// ParsePremierDraftMakePick parses a Premier EventPlayerDraftMakePick request
// entry into the existing DraftPickPayload. The pick data is a JSON STRING
// nested under "request" (double-unmarshal). Pack/Pick are 1-based on the wire
// and converted to 0-based here. CourseName is left empty; DraftID is carried.
//
// The parser is strict: it requires a non-empty inner DraftId (per Ray's note,
// preferring DraftId != "" over strings.Contains). The classifier may keep a
// Contains shortcut, but the parser validates the decoded struct.
func ParsePremierDraftMakePick(entry *LogEntry) (*DraftPickPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	reqStr, ok := entry.JSON["request"].(string)
	if !ok || reqStr == "" {
		return nil, fmt.Errorf("entry does not contain request string")
	}

	var req PremierDraftMakePickRequest
	if err := json.Unmarshal([]byte(reqStr), &req); err != nil {
		return nil, fmt.Errorf("unmarshal EventPlayerDraftMakePick request: %w", err)
	}
	if req.DraftID == "" {
		return nil, fmt.Errorf("EventPlayerDraftMakePick request missing DraftId")
	}

	return &DraftPickPayload{
		CourseName:  "",
		DraftID:     req.DraftID,
		PickedCards: req.GrpIDs,
		PackNumber:  req.Pack - 1,
		PickNumber:  req.Pick - 1,
	}, nil
}
