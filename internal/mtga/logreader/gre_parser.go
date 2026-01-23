package logreader

import (
	"fmt"
	"time"
)

// GREGameStateMessage represents a parsed game state message from the GRE.
type GREGameStateMessage struct {
	MatchID       string
	GameNumber    int
	TurnInfo      *GRETurnInfo
	Players       []GREPlayerState
	GameObjects   []GREGameObject
	Zones         map[int]*GREZone     // Zone ID to zone info mapping
	PrevGameState *GREGameStateMessage // For comparing state changes
	Timestamp     time.Time
}

// GREZone represents a zone in the game.
type GREZone struct {
	ZoneID      int
	Type        string // e.g., "ZoneType_Hand", "ZoneType_Battlefield"
	Visibility  string
	OwnerSeatID int // 0 if shared zone (battlefield, stack, etc.)
}

// GRETurnInfo contains information about the current turn.
type GRETurnInfo struct {
	TurnNumber          int
	Phase               string // "Phase_Main1", "Phase_Combat", etc.
	Step                string // "Step_BeginCombat", "Step_DeclareAttackers", etc.
	ActivePlayer        int    // Seat ID of the active player
	PriorityPlayer      int    // Seat ID of player with priority
	DecisionPlayer      int    // Seat ID of player making a decision
	NextPhase           string
	NextStep            string
	StormCount          int
	ManaSpent           int
	PhasePaymentOptions []interface{}
}

// GREPlayerState represents a player's state in the game.
type GREPlayerState struct {
	SeatID          int
	LifeTotal       int
	TeamID          int
	MaxHandSize     int
	PendingMessages int
	TimerState      string
	TimeRemaining   int
	SystemSeatID    int // For identifying player vs opponent
}

// GREGameObject represents an object in the game (card, token, etc.).
type GREGameObject struct {
	InstanceID           int
	GRPId                int    // Arena card ID
	OwnerSeatID          int    // Who owns this object
	ControllerSeatID     int    // Who controls this object
	ZoneID               int    // Current zone
	ZoneName             string // Derived from ZoneID
	CardTypes            []string
	Subtypes             []string
	SuperTypes           []string
	Power                int
	Toughness            int
	IsTapped             bool
	IsAttacking          bool
	IsBlocking           bool
	HasSummoningSickness bool
	Counters             map[string]int
	Abilities            []int
}

// GamePlayEvent represents a detected game play/action.
type GamePlayEvent struct {
	MatchID        string
	GameNumber     int
	TurnNumber     int
	Phase          string
	Step           string
	PlayerType     string // "player" or "opponent"
	ActionType     string // "play_card", "attack", "block", "land_drop", "life_change", etc.
	CardID         int    // Arena card ID (GRPId)
	CardName       string // Will be populated later from card database
	ZoneFrom       string
	ZoneTo         string
	LifeFrom       int // Previous life total (for life_change events)
	LifeTo         int // New life total (for life_change events)
	Timestamp      time.Time
	SequenceNumber int
}

// GREConnection stores the player's connection info for seat identification.
type GREConnection struct {
	SeatID       int
	SystemSeatID int
	TeamID       int
}

// GetPlayerSeatID extracts the player's seat ID from connectResp messages.
// This is used to identify which objects belong to the player vs opponent.
// If playerScreenName is provided, it matches by name; otherwise falls back to connectResp.
func GetPlayerSeatID(entries []*LogEntry) *GREConnection {
	return GetPlayerSeatIDByName(entries, "")
}

// GetPlayerSeatIDByName extracts the player's seat ID, matching by screen name if provided.
// This ensures correct player identification even when player order varies in reservedPlayers.
func GetPlayerSeatIDByName(entries []*LogEntry, playerScreenName string) *GREConnection {
	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		// Look for connectResp - this is reliable as it's sent directly to the player
		if connectResp, ok := entry.JSON["connectResp"]; ok {
			connMap, ok := connectResp.(map[string]interface{})
			if !ok {
				continue
			}

			conn := &GREConnection{}

			// Get system seat IDs from connectResp - this is always the player's seat
			if seatIDs, ok := connMap["systemSeatIds"].([]interface{}); ok && len(seatIDs) > 0 {
				if seatID, ok := seatIDs[0].(float64); ok {
					conn.SystemSeatID = int(seatID)
					conn.SeatID = int(seatID)
				}
			}

			// Try to get teamId
			if teamID, ok := connMap["teamId"].(float64); ok {
				conn.TeamID = int(teamID)
			}

			if conn.SeatID != 0 || conn.SystemSeatID != 0 {
				return conn
			}
		}

		// Also look for matchGameRoomStateChangedEvent for seat info
		if matchEvent, ok := entry.JSON["matchGameRoomStateChangedEvent"]; ok {
			eventMap, ok := matchEvent.(map[string]interface{})
			if !ok {
				continue
			}

			gameRoomInfo, ok := eventMap["gameRoomInfo"].(map[string]interface{})
			if !ok {
				continue
			}

			gameRoomConfig, ok := gameRoomInfo["gameRoomConfig"].(map[string]interface{})
			if !ok {
				continue
			}

			reservedPlayers, ok := gameRoomConfig["reservedPlayers"].([]interface{})
			if !ok {
				continue
			}

			// Search through all players to find the matching one
			for _, p := range reservedPlayers {
				player, ok := p.(map[string]interface{})
				if !ok {
					continue
				}

				// If we have a screen name, match by playerName
				if playerScreenName != "" {
					pName, hasName := player["playerName"].(string)
					if !hasName || pName != playerScreenName {
						continue // Not a match, try next player
					}
				}

				// Extract connection info
				conn := &GREConnection{}
				if seatID, ok := player["systemSeatId"].(float64); ok {
					conn.SystemSeatID = int(seatID)
					conn.SeatID = int(seatID)
				}
				if teamID, ok := player["teamId"].(float64); ok {
					conn.TeamID = int(teamID)
				}

				// If matching by name, return this player's seat
				if playerScreenName != "" && (conn.SeatID != 0 || conn.SystemSeatID != 0) {
					return conn
				}

				// Without screen name, we can't reliably pick - skip matchGameRoomStateChangedEvent
				// and hope connectResp is found instead
				break
			}
		}
	}

	return nil
}

// ParseGREMessages extracts game state messages from log entries.
func ParseGREMessages(entries []*LogEntry) ([]*GREGameStateMessage, error) {
	var messages []*GREGameStateMessage

	for _, entry := range entries {
		if !entry.IsJSON {
			continue
		}

		// Parse entry timestamp
		entryTime := time.Now()
		if entry.Timestamp != "" {
			if t, err := parseLogTimestamp(entry.Timestamp); err == nil {
				entryTime = t
			}
		}

		// Look for greToClientEvent messages
		if greEvent, ok := entry.JSON["greToClientEvent"]; ok {
			eventMap, ok := greEvent.(map[string]interface{})
			if !ok {
				continue
			}

			greToClientMsgs, ok := eventMap["greToClientMessages"].([]interface{})
			if !ok {
				continue
			}

			for _, msgData := range greToClientMsgs {
				msgMap, ok := msgData.(map[string]interface{})
				if !ok {
					continue
				}

				// Check message type
				msgType, _ := msgMap["type"].(string)
				if msgType != "GREMessageType_GameStateMessage" {
					continue
				}

				msg := parseGameStateMessage(msgMap, entryTime)
				if msg != nil {
					messages = append(messages, msg)
				}
			}
		}
	}

	return messages, nil
}

// parseGameStateMessage parses a single game state message.
func parseGameStateMessage(msgMap map[string]interface{}, timestamp time.Time) *GREGameStateMessage {
	msg := &GREGameStateMessage{
		Timestamp: timestamp,
		Zones:     make(map[int]*GREZone),
	}

	// Get game state info
	gameStateMsg, ok := msgMap["gameStateMessage"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Parse zones first so we can use them when parsing game objects
	if zones, ok := gameStateMsg["zones"].([]interface{}); ok {
		for _, zoneData := range zones {
			zoneMap, ok := zoneData.(map[string]interface{})
			if !ok {
				continue
			}
			zone := parseZone(zoneMap)
			if zone != nil {
				msg.Zones[zone.ZoneID] = zone
			}
		}
	}

	// Parse turn info
	if turnInfo, ok := gameStateMsg["turnInfo"].(map[string]interface{}); ok {
		msg.TurnInfo = parseTurnInfo(turnInfo)
	}

	// Parse players
	if players, ok := gameStateMsg["players"].([]interface{}); ok {
		for _, playerData := range players {
			playerMap, ok := playerData.(map[string]interface{})
			if !ok {
				continue
			}
			player := parsePlayerState(playerMap)
			msg.Players = append(msg.Players, player)
		}
	}

	// Parse game objects (using zones map for name resolution)
	if gameObjects, ok := gameStateMsg["gameObjects"].([]interface{}); ok {
		for _, objData := range gameObjects {
			objMap, ok := objData.(map[string]interface{})
			if !ok {
				continue
			}
			obj := parseGameObjectWithZones(objMap, msg.Zones)
			msg.GameObjects = append(msg.GameObjects, obj)
		}
	}

	// Get game info if available
	if gameInfo, ok := gameStateMsg["gameInfo"].(map[string]interface{}); ok {
		if matchID, ok := gameInfo["matchID"].(string); ok {
			msg.MatchID = matchID
		}
		if gameNumber, ok := gameInfo["gameNumber"].(float64); ok {
			msg.GameNumber = int(gameNumber)
		}
	}

	return msg
}

// parseZone parses a zone from the game state.
func parseZone(zoneMap map[string]interface{}) *GREZone {
	zone := &GREZone{}

	if zoneID, ok := zoneMap["zoneId"].(float64); ok {
		zone.ZoneID = int(zoneID)
	} else {
		return nil // Zone ID is required
	}

	if zoneType, ok := zoneMap["type"].(string); ok {
		zone.Type = zoneType
	}
	if visibility, ok := zoneMap["visibility"].(string); ok {
		zone.Visibility = visibility
	}
	if ownerSeatID, ok := zoneMap["ownerSeatId"].(float64); ok {
		zone.OwnerSeatID = int(ownerSeatID)
	}

	return zone
}

// parseTurnInfo parses turn information from the game state.
func parseTurnInfo(turnInfo map[string]interface{}) *GRETurnInfo {
	ti := &GRETurnInfo{}

	if turnNumber, ok := turnInfo["turnNumber"].(float64); ok {
		ti.TurnNumber = int(turnNumber)
	}
	if phase, ok := turnInfo["phase"].(string); ok {
		ti.Phase = phase
	}
	if step, ok := turnInfo["step"].(string); ok {
		ti.Step = step
	}
	if activePlayer, ok := turnInfo["activePlayer"].(float64); ok {
		ti.ActivePlayer = int(activePlayer)
	}
	if priorityPlayer, ok := turnInfo["priorityPlayer"].(float64); ok {
		ti.PriorityPlayer = int(priorityPlayer)
	}
	if decisionPlayer, ok := turnInfo["decisionPlayer"].(float64); ok {
		ti.DecisionPlayer = int(decisionPlayer)
	}
	if nextPhase, ok := turnInfo["nextPhase"].(string); ok {
		ti.NextPhase = nextPhase
	}
	if nextStep, ok := turnInfo["nextStep"].(string); ok {
		ti.NextStep = nextStep
	}

	return ti
}

// parsePlayerState parses a player's state from the game state.
func parsePlayerState(playerMap map[string]interface{}) GREPlayerState {
	ps := GREPlayerState{}

	if seatID, ok := playerMap["seatId"].(float64); ok {
		ps.SeatID = int(seatID)
	}
	if lifeTotal, ok := playerMap["lifeTotal"].(float64); ok {
		ps.LifeTotal = int(lifeTotal)
	}
	if teamID, ok := playerMap["teamId"].(float64); ok {
		ps.TeamID = int(teamID)
	}
	if maxHandSize, ok := playerMap["maxHandSize"].(float64); ok {
		ps.MaxHandSize = int(maxHandSize)
	}
	if systemSeatID, ok := playerMap["systemSeatId"].(float64); ok {
		ps.SystemSeatID = int(systemSeatID)
	}
	if timerState, ok := playerMap["timerState"].(string); ok {
		ps.TimerState = timerState
	}
	if timeRemaining, ok := playerMap["timeRemaining"].(float64); ok {
		ps.TimeRemaining = int(timeRemaining)
	}

	return ps
}

// parseGameObject parses a game object from the game state (legacy, no zones map).
func parseGameObject(objMap map[string]interface{}) GREGameObject {
	return parseGameObjectWithZones(objMap, nil)
}

// parseGameObjectWithZones parses a game object using the zones map for name resolution.
func parseGameObjectWithZones(objMap map[string]interface{}, zones map[int]*GREZone) GREGameObject {
	obj := GREGameObject{
		Counters: make(map[string]int),
	}

	if instanceID, ok := objMap["instanceId"].(float64); ok {
		obj.InstanceID = int(instanceID)
	}
	if grpID, ok := objMap["grpId"].(float64); ok {
		obj.GRPId = int(grpID)
	}
	if ownerSeatID, ok := objMap["ownerSeatId"].(float64); ok {
		obj.OwnerSeatID = int(ownerSeatID)
	}
	if controllerSeatID, ok := objMap["controllerSeatId"].(float64); ok {
		obj.ControllerSeatID = int(controllerSeatID)
	}
	if zoneID, ok := objMap["zoneId"].(float64); ok {
		obj.ZoneID = int(zoneID)
		obj.ZoneName = zoneIDToNameWithMap(int(zoneID), zones)
	}

	// Parse card types
	if cardTypes, ok := objMap["cardTypes"].([]interface{}); ok {
		for _, ct := range cardTypes {
			if ctStr, ok := ct.(string); ok {
				obj.CardTypes = append(obj.CardTypes, ctStr)
			}
		}
	}

	// Parse other attributes
	if power, ok := objMap["power"].(map[string]interface{}); ok {
		if val, ok := power["value"].(float64); ok {
			obj.Power = int(val)
		}
	}
	if toughness, ok := objMap["toughness"].(map[string]interface{}); ok {
		if val, ok := toughness["value"].(float64); ok {
			obj.Toughness = int(val)
		}
	}
	if isTapped, ok := objMap["isTapped"].(bool); ok {
		obj.IsTapped = isTapped
	}
	if attacking, ok := objMap["attackState"].(string); ok {
		obj.IsAttacking = attacking == "AttackState_Attacking"
	}
	if blocking, ok := objMap["blockState"].(string); ok {
		obj.IsBlocking = blocking != "" && blocking != "BlockState_None"
	}

	// Parse counters
	if counters, ok := objMap["counters"].([]interface{}); ok {
		for _, counterData := range counters {
			counterMap, ok := counterData.(map[string]interface{})
			if !ok {
				continue
			}
			if counterType, ok := counterMap["type"].(string); ok {
				if count, ok := counterMap["count"].(float64); ok {
					obj.Counters[counterType] = int(count)
				}
			}
		}
	}

	return obj
}

// zoneIDToNameWithMap maps zone IDs to readable zone names using the zones map.
func zoneIDToNameWithMap(zoneID int, zones map[int]*GREZone) string {
	if zones != nil {
		if zone, ok := zones[zoneID]; ok {
			return zoneTypeToReadableName(zone.Type)
		}
	}
	// Fallback to legacy method if zones map is not available
	return zoneIDToName(zoneID)
}

// zoneTypeToReadableName converts MTGA zone type to readable name.
func zoneTypeToReadableName(zoneType string) string {
	switch zoneType {
	case "ZoneType_Hand":
		return "hand"
	case "ZoneType_Library":
		return "library"
	case "ZoneType_Battlefield":
		return "battlefield"
	case "ZoneType_Graveyard":
		return "graveyard"
	case "ZoneType_Exile":
		return "exile"
	case "ZoneType_Stack":
		return "stack"
	case "ZoneType_Command":
		return "command"
	case "ZoneType_Sideboard":
		return "sideboard"
	case "ZoneType_Revealed":
		return "revealed"
	case "ZoneType_Limbo":
		return "limbo"
	case "ZoneType_Pending":
		return "pending"
	case "ZoneType_Suppressed":
		return "suppressed"
	default:
		// Strip "ZoneType_" prefix if present
		if len(zoneType) > 9 && zoneType[:9] == "ZoneType_" {
			return zoneType[9:]
		}
		return zoneType
	}
}

// zoneIDToName maps zone IDs to readable zone names using legacy modulo method.
// This is a fallback when the zones array is not available.
func zoneIDToName(zoneID int) string {
	// This legacy method uses modulo patterns that don't work reliably
	// for all MTGA zone IDs. Use zoneIDToNameWithMap when zones are available.
	zoneType := zoneID % 10
	switch zoneType {
	case 1:
		return "hand"
	case 2:
		return "library"
	case 3:
		return "battlefield"
	case 4:
		return "graveyard"
	case 5:
		return "exile"
	case 6:
		return "stack"
	case 7:
		return "command"
	default:
		return fmt.Sprintf("zone_%d", zoneID)
	}
}

// ParseGamePlays extracts game plays by comparing consecutive game states.
func ParseGamePlays(entries []*LogEntry, playerConn *GREConnection) ([]*GamePlayEvent, error) {
	messages, err := ParseGREMessages(entries)
	if err != nil {
		return nil, err
	}

	if len(messages) < 2 {
		return nil, nil
	}

	// Build a cumulative zones map from all messages
	// The zones are typically defined in the first full game state message
	cumulativeZones := make(map[int]*GREZone)
	for _, msg := range messages {
		for zoneID, zone := range msg.Zones {
			cumulativeZones[zoneID] = zone
		}
	}

	// Track all game objects across all messages for proper zone transition detection
	allObjects := make(map[int]*trackedObject)

	// Track life totals for life change detection
	lifeTotals := make(map[int]int) // seatID -> life total

	var plays []*GamePlayEvent
	sequenceNum := 0

	// Track the current game number from messages that include it.
	// Many GRE messages don't include gameNumber in gameInfo, so we need to
	// track it ourselves and apply it to plays from messages missing the field.
	currentGameNumber := 1 // Default to game 1

	// Track the last valid turn number from messages that include it.
	// Some GRE messages (especially zone transfers for lands) report turnNumber=0
	// even though they happen during a valid turn. We carry forward the last valid
	// turn number to properly attribute these events.
	lastValidTurnNumber := 1 // Default to turn 1

	// Compare consecutive states to detect changes
	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		curr := messages[i]

		// Update tracked game number if this message has a valid one
		if curr.GameNumber > 0 {
			currentGameNumber = curr.GameNumber
		}

		// Update tracked turn number if this message has a valid one
		if curr.TurnInfo != nil && curr.TurnInfo.TurnNumber > 0 {
			lastValidTurnNumber = curr.TurnInfo.TurnNumber
		}

		// Detect life changes
		lifeChanges := detectLifeChanges(prev, curr, playerConn, lifeTotals)
		for _, change := range lifeChanges {
			change.SequenceNumber = sequenceNum
			sequenceNum++
			// Apply tracked game number if the message didn't have one
			if change.GameNumber == 0 {
				change.GameNumber = currentGameNumber
			}
			// Apply tracked turn number if the message had turnNumber=0
			if change.TurnNumber == 0 {
				change.TurnNumber = lastValidTurnNumber
			}
			plays = append(plays, change)
		}

		// Detect zone changes using cumulative zones map
		zoneChanges := detectZoneChangesWithZones(prev, curr, playerConn, cumulativeZones, allObjects)
		for _, change := range zoneChanges {
			change.SequenceNumber = sequenceNum
			sequenceNum++
			if change.GameNumber == 0 {
				change.GameNumber = currentGameNumber
			}
			// Apply tracked turn number if the message had turnNumber=0
			// This is especially important for land drops which often have turnNumber=0
			if change.TurnNumber == 0 {
				change.TurnNumber = lastValidTurnNumber
			}
			plays = append(plays, change)
		}

		// Detect attacks
		attacks := detectAttacks(prev, curr, playerConn)
		for _, attack := range attacks {
			attack.SequenceNumber = sequenceNum
			sequenceNum++
			if attack.GameNumber == 0 {
				attack.GameNumber = currentGameNumber
			}
			// Apply tracked turn number if the message had turnNumber=0
			if attack.TurnNumber == 0 {
				attack.TurnNumber = lastValidTurnNumber
			}
			plays = append(plays, attack)
		}

		// Detect blocks
		blocks := detectBlocks(prev, curr, playerConn)
		for _, block := range blocks {
			block.SequenceNumber = sequenceNum
			sequenceNum++
			if block.GameNumber == 0 {
				block.GameNumber = currentGameNumber
			}
			// Apply tracked turn number if the message had turnNumber=0
			if block.TurnNumber == 0 {
				block.TurnNumber = lastValidTurnNumber
			}
			plays = append(plays, block)
		}

		// Update tracked objects from current state
		for _, obj := range curr.GameObjects {
			allObjects[obj.InstanceID] = &trackedObject{
				instanceID:   obj.InstanceID,
				grpID:        obj.GRPId,
				zoneID:       obj.ZoneID,
				controllerID: obj.ControllerSeatID,
			}
		}

		// Update life totals from current state
		for _, player := range curr.Players {
			lifeTotals[player.SeatID] = player.LifeTotal
		}
	}

	return plays, nil
}

// detectLifeChanges finds changes in player life totals.
func detectLifeChanges(prev, curr *GREGameStateMessage, playerConn *GREConnection, lifeTotals map[int]int) []*GamePlayEvent {
	var events []*GamePlayEvent

	// Get turn number safely (may be nil for some game states)
	turnNumber := 0
	if curr.TurnInfo != nil {
		turnNumber = curr.TurnInfo.TurnNumber
	}

	for _, player := range curr.Players {
		prevLife, existed := lifeTotals[player.SeatID]
		if !existed {
			// First time seeing this player, check prev message
			for _, prevPlayer := range prev.Players {
				if prevPlayer.SeatID == player.SeatID {
					prevLife = prevPlayer.LifeTotal
					existed = true
					break
				}
			}
		}

		if existed && prevLife != player.LifeTotal {
			// Skip mulligan-related life changes (turn 0 or very early)
			// The caller will apply lastValidTurnNumber to turn 0 events
			if turnNumber < 1 && curr.TurnInfo != nil {
				continue
			}

			playerType := "opponent"
			if playerConn != nil && player.SeatID == playerConn.SeatID {
				playerType = "player"
			}

			// Extract phase/step safely (TurnInfo may be nil)
			phase := ""
			step := ""
			if curr.TurnInfo != nil {
				phase = normalizePhase(curr.TurnInfo.Phase)
				step = normalizeStep(curr.TurnInfo.Step)
			}

			event := &GamePlayEvent{
				MatchID:    curr.MatchID,
				GameNumber: curr.GameNumber,
				TurnNumber: turnNumber,
				Phase:      phase,
				Step:       step,
				PlayerType: playerType,
				ActionType: "life_change",
				LifeFrom:   prevLife,
				LifeTo:     player.LifeTotal,
				Timestamp:  curr.Timestamp,
			}
			events = append(events, event)
		}
	}

	return events
}

// trackedObject tracks an object's state across game state messages.
type trackedObject struct {
	instanceID   int
	grpID        int
	zoneID       int
	controllerID int
}

// detectZoneChangesWithZones finds objects that moved between zones using the cumulative zones map.
func detectZoneChangesWithZones(prev, curr *GREGameStateMessage, playerConn *GREConnection, zones map[int]*GREZone, trackedObjs map[int]*trackedObject) []*GamePlayEvent {
	var events []*GamePlayEvent

	// Note: We no longer return early when TurnInfo is nil.
	// Zone changes (especially land drops) often occur in game states without TurnInfo.
	// The caller (ParseGamePlays) will apply the last valid turn number to events with TurnNumber=0.

	// Build map of previous objects from both the prev message and our tracked state
	prevObjects := make(map[int]*trackedObject)

	// First, use previously tracked objects
	for id, obj := range trackedObjs {
		prevObjects[id] = obj
	}

	// Then overlay with objects from the previous message (more recent state)
	for _, obj := range prev.GameObjects {
		prevObjects[obj.InstanceID] = &trackedObject{
			instanceID:   obj.InstanceID,
			grpID:        obj.GRPId,
			zoneID:       obj.ZoneID,
			controllerID: obj.ControllerSeatID,
		}
	}

	// Check current objects for zone changes
	for _, currObj := range curr.GameObjects {
		prevObj, existed := prevObjects[currObj.InstanceID]

		// Get zone names using the cumulative zones map
		currZoneName := getZoneName(currObj.ZoneID, zones)
		prevZoneName := ""
		if existed {
			prevZoneName = getZoneName(prevObj.zoneID, zones)
		}

		// Skip if zone hasn't changed
		if existed && prevObj.zoneID == currObj.ZoneID {
			continue
		}

		// Determine player type
		playerType := "opponent"
		if playerConn != nil && currObj.ControllerSeatID == playerConn.SeatID {
			playerType = "player"
		}

		// Determine action type based on zone transition
		actionType := determineActionType(prevZoneName, currZoneName, currObj.CardTypes)

		// Skip draw events (library to hand) - not interesting plays
		if actionType == "draw" {
			continue
		}

		// Skip mulligan/game start events
		if prevZoneName == "" && currZoneName == "hand" {
			continue
		}

		// Extract turn info safely (may be nil for some game states)
		turnNumber := 0
		phase := ""
		step := ""
		if curr.TurnInfo != nil {
			turnNumber = curr.TurnInfo.TurnNumber
			phase = normalizePhase(curr.TurnInfo.Phase)
			step = normalizeStep(curr.TurnInfo.Step)
		}

		event := &GamePlayEvent{
			MatchID:    curr.MatchID,
			GameNumber: curr.GameNumber,
			TurnNumber: turnNumber,
			Phase:      phase,
			Step:       step,
			PlayerType: playerType,
			ActionType: actionType,
			CardID:     currObj.GRPId,
			ZoneFrom:   prevZoneName,
			ZoneTo:     currZoneName,
			Timestamp:  curr.Timestamp,
		}
		events = append(events, event)
	}

	return events
}

// getZoneName returns the readable zone name for a zone ID using the zones map.
// Falls back to legacy modulo-based mapping if zones map doesn't have the zone.
func getZoneName(zoneID int, zones map[int]*GREZone) string {
	if zones != nil {
		if zone, ok := zones[zoneID]; ok {
			return zoneTypeToReadableName(zone.Type)
		}
	}
	// Fallback to legacy method for test compatibility and edge cases
	return zoneIDToName(zoneID)
}

// determineActionType determines the action type based on zone transition.
func determineActionType(fromZone, toZone string, cardTypes []string) string {
	// Check if it's a land
	isLand := false
	for _, ct := range cardTypes {
		if ct == "CardType_Land" {
			isLand = true
			break
		}
	}

	switch {
	case fromZone == "hand" && toZone == "battlefield":
		if isLand {
			return "land_drop"
		}
		return "play_card"
	case fromZone == "hand" && toZone == "stack":
		return "cast_spell"
	case fromZone == "stack" && toZone == "battlefield":
		return "resolve_spell"
	case fromZone == "library" && toZone == "hand":
		return "draw"
	case toZone == "graveyard":
		return "to_graveyard"
	case toZone == "exile":
		return "exile"
	case toZone == "battlefield":
		return "enter_battlefield"
	case toZone == "stack":
		return "cast_spell"
	default:
		return "zone_change"
	}
}

// detectAttacks finds creatures that started attacking.
func detectAttacks(prev, curr *GREGameStateMessage, playerConn *GREConnection) []*GamePlayEvent {
	var events []*GamePlayEvent

	// Note: We no longer return early when TurnInfo is nil.
	// The caller (ParseGamePlays) will apply the last valid turn number to events with TurnNumber=0.

	// Extract turn info safely (may be nil for some game states)
	turnNumber := 0
	phase := ""
	step := ""
	if curr.TurnInfo != nil {
		turnNumber = curr.TurnInfo.TurnNumber
		phase = normalizePhase(curr.TurnInfo.Phase)
		step = normalizeStep(curr.TurnInfo.Step)
	}

	// Build map of previous attacking state
	prevAttacking := make(map[int]bool)
	for _, obj := range prev.GameObjects {
		prevAttacking[obj.InstanceID] = obj.IsAttacking
	}

	// Check for new attackers
	for _, obj := range curr.GameObjects {
		wasAttacking := prevAttacking[obj.InstanceID]
		if obj.IsAttacking && !wasAttacking {
			playerType := "opponent"
			if playerConn != nil && obj.ControllerSeatID == playerConn.SeatID {
				playerType = "player"
			}

			event := &GamePlayEvent{
				MatchID:    curr.MatchID,
				GameNumber: curr.GameNumber,
				TurnNumber: turnNumber,
				Phase:      phase,
				Step:       step,
				PlayerType: playerType,
				ActionType: "attack",
				CardID:     obj.GRPId,
				ZoneFrom:   "battlefield",
				ZoneTo:     "battlefield",
				Timestamp:  curr.Timestamp,
			}
			events = append(events, event)
		}
	}

	return events
}

// detectBlocks finds creatures that started blocking.
func detectBlocks(prev, curr *GREGameStateMessage, playerConn *GREConnection) []*GamePlayEvent {
	var events []*GamePlayEvent

	// Note: We no longer return early when TurnInfo is nil.
	// The caller (ParseGamePlays) will apply the last valid turn number to events with TurnNumber=0.

	// Extract turn info safely (may be nil for some game states)
	turnNumber := 0
	phase := ""
	step := ""
	if curr.TurnInfo != nil {
		turnNumber = curr.TurnInfo.TurnNumber
		phase = normalizePhase(curr.TurnInfo.Phase)
		step = normalizeStep(curr.TurnInfo.Step)
	}

	// Build map of previous blocking state
	prevBlocking := make(map[int]bool)
	for _, obj := range prev.GameObjects {
		prevBlocking[obj.InstanceID] = obj.IsBlocking
	}

	// Check for new blockers
	for _, obj := range curr.GameObjects {
		wasBlocking := prevBlocking[obj.InstanceID]
		if obj.IsBlocking && !wasBlocking {
			playerType := "opponent"
			if playerConn != nil && obj.ControllerSeatID == playerConn.SeatID {
				playerType = "player"
			}

			event := &GamePlayEvent{
				MatchID:    curr.MatchID,
				GameNumber: curr.GameNumber,
				TurnNumber: turnNumber,
				Phase:      phase,
				Step:       step,
				PlayerType: playerType,
				ActionType: "block",
				CardID:     obj.GRPId,
				ZoneFrom:   "battlefield",
				ZoneTo:     "battlefield",
				Timestamp:  curr.Timestamp,
			}
			events = append(events, event)
		}
	}

	return events
}

// normalizePhase converts MTGA phase names to readable names.
func normalizePhase(phase string) string {
	switch phase {
	case "Phase_Beginning":
		return "Beginning"
	case "Phase_Main1":
		return "Main1"
	case "Phase_Combat":
		return "Combat"
	case "Phase_Main2":
		return "Main2"
	case "Phase_Ending":
		return "Ending"
	default:
		return phase
	}
}

// normalizeStep converts MTGA step names to readable names.
func normalizeStep(step string) string {
	switch step {
	case "Step_Upkeep":
		return "Upkeep"
	case "Step_Draw":
		return "Draw"
	case "Step_BeginCombat":
		return "BeginCombat"
	case "Step_DeclareAttack":
		return "DeclareAttackers"
	case "Step_DeclareBlock":
		return "DeclareBlockers"
	case "Step_CombatDamage":
		return "CombatDamage"
	case "Step_FirstStrikeDamage":
		return "FirstStrikeDamage"
	case "Step_EndCombat":
		return "EndCombat"
	case "Step_End":
		return "EndStep"
	case "Step_Cleanup":
		return "Cleanup"
	default:
		return step
	}
}

// ExtractOpponentCards extracts all cards observed from the opponent.
func ExtractOpponentCards(entries []*LogEntry, playerConn *GREConnection) ([]OpponentCard, error) {
	messages, err := ParseGREMessages(entries)
	if err != nil {
		return nil, err
	}

	// Track seen cards with the turn they were first seen
	seenCards := make(map[int]*OpponentCard)

	for _, msg := range messages {
		turnNumber := 0
		if msg.TurnInfo != nil {
			turnNumber = msg.TurnInfo.TurnNumber
		}

		for _, obj := range msg.GameObjects {
			// Skip player's own cards
			if playerConn != nil && obj.ControllerSeatID == playerConn.SeatID {
				continue
			}

			// Skip cards without a valid GRPId
			if obj.GRPId == 0 {
				continue
			}

			// Track the card
			if existing, exists := seenCards[obj.GRPId]; exists {
				existing.TimesSeen++
				// Update zone if moved to a more revealing zone
				if obj.ZoneName == "battlefield" || obj.ZoneName == "hand" || obj.ZoneName == "graveyard" {
					existing.ZoneObserved = obj.ZoneName
				}
			} else {
				seenCards[obj.GRPId] = &OpponentCard{
					CardID:        obj.GRPId,
					ZoneObserved:  obj.ZoneName,
					TurnFirstSeen: turnNumber,
					TimesSeen:     1,
				}
			}
		}
	}

	// Convert map to slice
	var cards []OpponentCard
	for _, card := range seenCards {
		cards = append(cards, *card)
	}

	return cards, nil
}

// OpponentCard represents an opponent's card that was observed.
type OpponentCard struct {
	CardID        int
	CardName      string // Will be populated later from card database
	ZoneObserved  string
	TurnFirstSeen int
	TimesSeen     int
}

// ExtractGameSnapshots extracts turn-by-turn snapshots from game state messages.
func ExtractGameSnapshots(entries []*LogEntry, playerConn *GREConnection) ([]*GameSnapshot, error) {
	messages, err := ParseGREMessages(entries)
	if err != nil {
		return nil, err
	}

	// Group by turn number and take the last state for each turn
	turnSnapshots := make(map[int]*GREGameStateMessage)
	for _, msg := range messages {
		if msg.TurnInfo == nil {
			continue
		}
		turnSnapshots[msg.TurnInfo.TurnNumber] = msg
	}

	var snapshots []*GameSnapshot
	for turnNumber, msg := range turnSnapshots {
		snapshot := &GameSnapshot{
			MatchID:    msg.MatchID,
			GameNumber: msg.GameNumber,
			TurnNumber: turnNumber,
			Timestamp:  msg.Timestamp,
		}

		if msg.TurnInfo != nil {
			activePlayer := "opponent"
			if playerConn != nil && msg.TurnInfo.ActivePlayer == playerConn.SeatID {
				activePlayer = "player"
			}
			snapshot.ActivePlayer = activePlayer
		}

		// Extract player and opponent states
		for _, player := range msg.Players {
			if playerConn != nil && player.SeatID == playerConn.SeatID {
				snapshot.PlayerLife = player.LifeTotal
			} else {
				snapshot.OpponentLife = player.LifeTotal
			}
		}

		// Count cards and lands by controller
		playerCardsInHand := 0
		opponentCardsInHand := 0
		playerLands := 0
		opponentLands := 0

		for _, obj := range msg.GameObjects {
			isPlayer := playerConn != nil && obj.ControllerSeatID == playerConn.SeatID

			if obj.ZoneName == "hand" {
				if isPlayer {
					playerCardsInHand++
				} else {
					opponentCardsInHand++
				}
			}

			if obj.ZoneName == "battlefield" {
				for _, cardType := range obj.CardTypes {
					if cardType == "CardType_Land" {
						if isPlayer {
							playerLands++
						} else {
							opponentLands++
						}
						break
					}
				}
			}
		}

		snapshot.PlayerCardsInHand = playerCardsInHand
		snapshot.OpponentCardsInHand = opponentCardsInHand
		snapshot.PlayerLandsInPlay = playerLands
		snapshot.OpponentLandsInPlay = opponentLands

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// GameSnapshot represents the game state at a specific turn.
type GameSnapshot struct {
	MatchID             string
	GameNumber          int
	TurnNumber          int
	ActivePlayer        string
	PlayerLife          int
	OpponentLife        int
	PlayerCardsInHand   int
	OpponentCardsInHand int
	PlayerLandsInPlay   int
	OpponentLandsInPlay int
	BoardStateJSON      string
	Timestamp           time.Time
}

// parseLogTimestamp is defined in draft_picks.go
