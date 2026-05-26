package main

import "log"

func handleVendorNpcInteraction(c *Client, npc *Npc, state npcInteractionState) {
	if c == nil || c.Char == nil || npc == nil {
		return
	}
	log.Printf("[NPC] %q selected vendor session=%d entry=%d kind=%s stock=%d fallback=%t",
		c.Char.Name, npc.SessionID, npc.EntryID, npcInteractionKindName(state.Kind),
		vendorStockCount(npc.EntryID), vendorStockUsesFallback())
}
