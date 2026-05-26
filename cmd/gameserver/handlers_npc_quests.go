package main

import "log"

func handleQuestNpcInteraction(c *Client, npc *Npc, state npcInteractionState) {
	if c == nil || c.Char == nil || npc == nil {
		return
	}
	log.Printf("[NPC] %q selected quest npc session=%d entry=%d kind=%s",
		c.Char.Name, npc.SessionID, npc.EntryID, npcInteractionKindName(state.Kind))
}
