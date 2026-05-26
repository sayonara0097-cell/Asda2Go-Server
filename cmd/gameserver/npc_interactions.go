package main

import (
	"sync"
	"time"

	"asda2/shared/types"
)

const (
	// WCell.RealmServer/NPCs/NPCMgr.cs DefaultInteractionDistance = 15.
	npcInteractionDistance = 15.0
	npcInteractionTTL      = 2 * time.Minute
)

type npcInteractionState struct {
	ClientID   uint32
	TargetID   uint16
	SessionID  int16
	SpawnID    uint32
	EntryID    uint16
	MapID      uint16
	ClassGroup byte
	Kind       types.NpcInteractionKind
	StartedAt  time.Time
	ExpiresAt  time.Time
}

var npcInteractions = struct {
	sync.Mutex
	byClient map[uint32]npcInteractionState
}{
	byClient: make(map[uint32]npcInteractionState),
}

func dispatchNpcInteraction(c *Client, npc *Npc, targetID uint16) {
	if c == nil || c.Char == nil || npc == nil {
		return
	}
	if !clientCanInteractWithNpc(c, npc) {
		clearNpcInteraction(c)
		debugNpcInteractionf("rejected char=%q target=%d entry=%d reason=range-or-channel", c.Char.Name, targetID, npc.EntryID)
		return
	}

	kind := npcInteractionKind(npc)
	state := rememberNpcInteraction(c, npc, targetID, kind)
	switch kind {
	case types.NpcInteractionTrainer:
		handleTrainerNpcInteraction(c, npc, state)
	case types.NpcInteractionVendor:
		handleVendorNpcInteraction(c, npc, state)
	case types.NpcInteractionQuest:
		handleQuestNpcInteraction(c, npc, state)
	default:
		handleDialogueNpcInteraction(c, npc, state)
	}
}

func rememberNpcInteraction(c *Client, npc *Npc, targetID uint16, kind types.NpcInteractionKind) npcInteractionState {
	now := time.Now()
	state := npcInteractionState{
		ClientID:   c.ID,
		TargetID:   targetID,
		SessionID:  npc.SessionID,
		SpawnID:    npc.SpawnID,
		EntryID:    npc.EntryID,
		MapID:      npc.MapID,
		ClassGroup: npc.ClassGroup,
		Kind:       kind,
		StartedAt:  now,
		ExpiresAt:  now.Add(npcInteractionTTL),
	}
	npcInteractions.Lock()
	npcInteractions.byClient[c.ID] = state
	npcInteractions.Unlock()
	return state
}

func currentNpcInteraction(c *Client) (npcInteractionState, bool) {
	if c == nil || c.Char == nil {
		return npcInteractionState{}, false
	}
	npcInteractions.Lock()
	state, ok := npcInteractions.byClient[c.ID]
	if !ok || time.Now().After(state.ExpiresAt) || state.MapID != c.Char.MapID {
		if ok {
			delete(npcInteractions.byClient, c.ID)
		}
		npcInteractions.Unlock()
		return npcInteractionState{}, false
	}
	npcInteractions.Unlock()
	return state, true
}

func clearNpcInteraction(c *Client) {
	if c == nil {
		return
	}
	npcInteractions.Lock()
	delete(npcInteractions.byClient, c.ID)
	npcInteractions.Unlock()
}

func resolveNpcInteractionNpc(c *Client, state npcInteractionState) *Npc {
	if c == nil || c.Char == nil {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}
	candidates := []uint16{uint16(state.SessionID), state.TargetID, state.EntryID, uint16(state.SpawnID)}
	for _, targetID := range candidates {
		if targetID == 0 {
			continue
		}
		if npc, ok := gm.FindNpcByClientTargetID(targetID); ok {
			return npc
		}
	}
	return nil
}

func npcInteractionKind(npc *Npc) types.NpcInteractionKind {
	if npc == nil {
		return types.NpcInteractionNone
	}
	if npc.InteractionKind != types.NpcInteractionNone {
		return npc.InteractionKind
	}
	template := types.NormalizeNpcTemplate(types.NpcTemplateRow{
		EntryID:    npc.EntryID,
		Name:       npc.Name,
		Kind:       npc.Kind,
		ClassGroup: npc.ClassGroup,
		IsTrainer:  npc.IsTrainer,
	})
	return template.InteractionKind
}

func clientCanInteractWithNpc(c *Client, npc *Npc) bool {
	if c == nil || c.Char == nil || npc == nil {
		return false
	}
	if c.Char.MapID != npc.MapID {
		return false
	}
	if npc.Channel >= 0 && npc.Channel != int16(c.Channel) {
		return false
	}
	return distance2D(
		float64(npc.LocalX),
		float64(npc.LocalY),
		float64(asda2X(c.Char.X, c.Char.MapID)),
		float64(asda2Y(c.Char.Y, c.Char.MapID)),
	) <= npcInteractionDistance
}

func npcInteractionKindName(kind types.NpcInteractionKind) string {
	switch kind {
	case types.NpcInteractionTrainer:
		return "trainer"
	case types.NpcInteractionVendor:
		return "vendor"
	case types.NpcInteractionQuest:
		return "quest"
	case types.NpcInteractionDialogue:
		return "dialogue"
	default:
		return "none"
	}
}

func debugNpcInteractionf(format string, args ...any) {
	if !visibilityDebugEnabled {
		return
	}
	debugNpcSpawnf(format, args...)
}
