package logreader

// botdraft.go — BotDraft (QuickDraft / bot-draft) wire format parsers (#337).
//
// QuickDraft (and other bot drafts) emit a different wire format than Premier
// (#338). Both shapes are DOUBLY-NESTED: a stringified inner JSON envelope with
// CAPITALIZED keys and STRINGIFIED grpIds, requiring a double-unmarshal.
//
// Pack (BotDraft status / pack offer):
//
//	{"CurrentModule":"BotDraft","Payload":"{\"Result\":\"Success\",
//	 \"EventName\":\"QuickDraft_SOS_20260526\",\"DraftStatus\":\"PickNext\",
//	 \"PackNumber\":0,\"PickNumber\":0,\"NumCardsToPick\":1,
//	 \"DraftPack\":[\"102470\",...],\"PackStyles\":[],
//	 \"PickedCards\":[],\"PickedStyles\":[]}"}
//
// Pick (BotDraftDraftPick request):
//
//	{"id":"<uuid>","request":"{\"EventName\":\"QuickDraft_SOS_20260526\",
//	 \"PickInfo\":{\"EventName\":\"QuickDraft_SOS_20260526\",
//	 \"CardIds\":[\"102704\"],\"PackNumber\":0,\"PickNumber\":0}}"}
//
// PackNumber/PickNumber are 0-based on the wire (no conversion). grpIds are
// STRINGS and are converted to ints. The parsers reuse the existing
// DraftPackPayload / DraftPickPayload types so HandlePack/HandlePick and the
// BFF contract are unchanged.

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// botDraftEnvelope is the outer wrapper for a BotDraft status/pack line. The
// inner Payload is a STRINGIFIED JSON object that must be unmarshalled again.
type botDraftEnvelope struct {
	CurrentModule string `json:"CurrentModule"`
	Payload       string `json:"Payload"`
}

// botDraftStatus is the decoded inner Payload of a BotDraft pack line. grpIds
// in DraftPack are strings on the wire.
type botDraftStatus struct {
	EventName  string   `json:"EventName"`
	PackNumber int      `json:"PackNumber"`
	PickNumber int      `json:"PickNumber"`
	DraftPack  []string `json:"DraftPack"`
}

// botDraftPickRequest is the decoded inner "request" string of a
// BotDraftDraftPick line. The presence of PickInfo distinguishes BotDraft from
// Premier (which carries DraftId/GrpIds/Pack/Pick instead).
type botDraftPickRequest struct {
	EventName string            `json:"EventName"`
	PickInfo  *botDraftPickInfo `json:"PickInfo"`
}

// botDraftPickInfo holds the actual pick data. CardIds are strings on the wire.
type botDraftPickInfo struct {
	EventName  string   `json:"EventName"`
	CardIds    []string `json:"CardIds"`
	PackNumber int      `json:"PackNumber"`
	PickNumber int      `json:"PickNumber"`
}

// parseStringGrpIDs converts a slice of stringified grpIds into ints. An empty
// slice yields an empty (non-nil) slice.
func parseStringGrpIDs(ids []string) ([]int, error) {
	out := make([]int, 0, len(ids))
	for _, s := range ids {
		id, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("parse grpId %q: %w", s, err)
		}
		out = append(out, id)
	}
	return out, nil
}

// ParseBotDraftStatusPack parses a BotDraft pack line (CurrentModule=BotDraft +
// stringified Payload) into the existing DraftPackPayload. EventName becomes
// CourseName (the draftstate session key). PackNumber/PickNumber are 0-based on
// the wire; the within-pack pick is reconstructed into the cumulative 1-based
// index the draftstate Store expects:
//
//	cumulative_1based = PackNumber*15 + PickNumber + 1
//
// (Consistent with the Premier formula (SelfPack-1)*15 + SelfPick: BotDraft
// pack=0/pick=0 → 1; Premier SelfPack=1/SelfPick=1 → 1.)
func ParseBotDraftStatusPack(entry *LogEntry) (*DraftPackPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}

	raw, err := json.Marshal(entry.JSON)
	if err != nil {
		return nil, fmt.Errorf("re-marshal entry JSON: %w", err)
	}

	var env botDraftEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("unmarshal botDraftEnvelope: %w", err)
	}
	if env.CurrentModule != "BotDraft" {
		return nil, fmt.Errorf("entry CurrentModule is %q, not BotDraft", env.CurrentModule)
	}
	if env.Payload == "" {
		return nil, fmt.Errorf("BotDraft envelope missing Payload")
	}

	var status botDraftStatus
	if err := json.Unmarshal([]byte(env.Payload), &status); err != nil {
		return nil, fmt.Errorf("unmarshal BotDraft Payload: %w", err)
	}

	cards, err := parseStringGrpIDs(status.DraftPack)
	if err != nil {
		return nil, err
	}

	cumulative := status.PackNumber*15 + status.PickNumber + 1

	return &DraftPackPayload{
		CourseName: status.EventName,
		DraftPack: DraftPackDetail{
			PackCards: cards,
			SelfPick:  cumulative,
		},
	}, nil
}

// ParseBotDraftPick parses a BotDraftDraftPick request line into the existing
// DraftPickPayload. The pick data is a JSON STRING nested under "request"
// (double-unmarshal) carrying PickInfo.CardIds. PackNumber/PickNumber are
// 0-based and passed through unchanged. EventName becomes CourseName.
//
// The parser is strict: a request without a PickInfo block is rejected — that
// is the Premier (DraftId/GrpIds/Pack/Pick) shape, not BotDraft.
func ParseBotDraftPick(entry *LogEntry) (*DraftPickPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}
	reqStr, ok := entry.JSON["request"].(string)
	if !ok || reqStr == "" {
		return nil, fmt.Errorf("entry does not contain request string")
	}

	var req botDraftPickRequest
	if err := json.Unmarshal([]byte(reqStr), &req); err != nil {
		return nil, fmt.Errorf("unmarshal BotDraftDraftPick request: %w", err)
	}
	if req.PickInfo == nil {
		return nil, fmt.Errorf("BotDraftDraftPick request missing PickInfo")
	}

	cards, err := parseStringGrpIDs(req.PickInfo.CardIds)
	if err != nil {
		return nil, err
	}

	courseName := req.PickInfo.EventName
	if courseName == "" {
		courseName = req.EventName
	}

	return &DraftPickPayload{
		CourseName:  courseName,
		PickedCards: cards,
		PackNumber:  req.PickInfo.PackNumber,
		PickNumber:  req.PickInfo.PickNumber,
	}, nil
}
