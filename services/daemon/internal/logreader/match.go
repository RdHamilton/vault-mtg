package logreader

import (
	"fmt"
	"log/slog"

	"github.com/RdHamilton/vault-mtg/services/contract"
)

// IsMatchCompletedEntry reports whether the log entry is a
// matchGameRoomStateChangedEvent with stateType
// "MatchGameRoomStateType_MatchCompleted".
//
// This is a stateless classifier — no GRE session buffering is required.
// Arena emits a single log line containing the full match result when the
// match ends.
func IsMatchCompletedEntry(entry *LogEntry) bool {
	if entry == nil || !entry.IsJSON {
		return false
	}
	event, ok := entry.JSON["matchGameRoomStateChangedEvent"].(map[string]interface{})
	if !ok {
		return false
	}
	gameRoomInfo, ok := event["gameRoomInfo"].(map[string]interface{})
	if !ok {
		return false
	}
	stateType, _ := gameRoomInfo["stateType"].(string)
	return stateType == "MatchGameRoomStateType_MatchCompleted"
}

// ParseMatchCompletedEntry parses a matchGameRoomStateChangedEvent log entry
// into a contract.MatchCompletedPayload.
//
// playerUserID is the local player's MTGA userId (from daemon config). It is
// used to identify the opponent in reservedPlayers. When empty the opponent
// name is omitted.
//
// Returns an error if the entry is not a valid match-completed event.
func ParseMatchCompletedEntry(entry *LogEntry, playerUserID string) (*contract.MatchCompletedPayload, error) {
	if entry == nil || !entry.IsJSON {
		return nil, fmt.Errorf("entry is not JSON")
	}

	event, ok := entry.JSON["matchGameRoomStateChangedEvent"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("entry does not contain matchGameRoomStateChangedEvent")
	}

	gameRoomInfo, ok := event["gameRoomInfo"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("matchGameRoomStateChangedEvent has no gameRoomInfo")
	}

	stateType, _ := gameRoomInfo["stateType"].(string)
	if stateType != "MatchGameRoomStateType_MatchCompleted" {
		return nil, fmt.Errorf("stateType is %q, expected MatchGameRoomStateType_MatchCompleted", stateType)
	}

	p := &contract.MatchCompletedPayload{
		ResultList: []contract.MatchResult{},
	}

	// --- finalMatchResult ---
	if fmr, ok := gameRoomInfo["finalMatchResult"].(map[string]interface{}); ok {
		if matchID, ok := fmr["matchId"].(string); ok {
			p.MatchID = matchID
		}

		if rl, ok := fmr["resultList"].([]interface{}); ok {
			for _, item := range rl {
				rm, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				r := contract.MatchResult{}
				if v, ok := rm["scope"].(string); ok {
					r.Scope = v
				}
				if v, ok := rm["result"].(string); ok {
					r.Result = v
				}
				if v, ok := rm["winningTeamId"].(float64); ok {
					r.WinningTeamID = int(v)
				}
				if v, ok := rm["reason"].(string); ok {
					r.Reason = v
				}
				p.ResultList = append(p.ResultList, r)

				// Capture top-level winner from the match-scope result.
				if r.Scope == "MatchScope_Match" {
					p.WinningTeamID = r.WinningTeamID
				}
			}
		}
	}

	// --- gameRoomConfig (format + opponent name + player team) ---
	//
	// Real MTGA wire format: gameRoomConfig carries "reservedPlayers" and
	// "matchId" at the top level only.  eventId lives inside each
	// reservedPlayers[] entry — there is no top-level eventId key.
	if cfg, ok := gameRoomInfo["gameRoomConfig"].(map[string]interface{}); ok {
		// Iterate reservedPlayers to:
		//   1. Extract eventId (format) from the first player entry that carries it.
		//   2. Identify the local player's teamId.
		//   3. Identify the opponent's display name.
		if players, ok := cfg["reservedPlayers"].([]interface{}); ok {
			for _, pl := range players {
				pm, ok := pl.(map[string]interface{})
				if !ok {
					continue
				}
				uid, _ := pm["userId"].(string)
				name, _ := pm["playerName"].(string)
				teamID := int(0)
				if v, ok := pm["teamId"].(float64); ok {
					teamID = int(v)
				}

				// Pick up eventId from the first player entry that has it.
				if p.Format == "" {
					if eid, ok := pm["eventId"].(string); ok && eid != "" {
						p.Format = eid
					}
				}

				if playerUserID != "" && uid == playerUserID {
					// This entry is the local player.
					p.PlayerTeamID = teamID
				} else if uid != "" && name != "" && p.OpponentName == "" {
					// First non-local player with a name is the opponent.
					p.OpponentName = name
				}
			}
		}

		// Warn after exhausting all reservedPlayers entries if no eventId was
		// found.  The daemon emits an empty Format; the BFF owns the display
		// default.
		if p.Format == "" {
			slog.Warn("match.go: no reservedPlayers entry carries eventId; format will be empty", "matchID", p.MatchID)
		}
	}

	// Derive Result, PlayerWins, OpponentWins when we identified the player.
	if p.PlayerTeamID > 0 {
		if p.WinningTeamID == p.PlayerTeamID {
			p.Result = "win"
		} else if p.WinningTeamID > 0 {
			p.Result = "loss"
		}

		for _, r := range p.ResultList {
			if r.Scope != "MatchScope_Game" {
				continue
			}
			if r.WinningTeamID == p.PlayerTeamID {
				p.PlayerWins++
			} else if r.WinningTeamID > 0 {
				p.OpponentWins++
			}
		}
	}

	return p, nil
}
