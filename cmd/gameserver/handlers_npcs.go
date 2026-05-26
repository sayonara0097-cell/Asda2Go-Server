package main

const (
	// WCell.RealmServer/Asda2Quests/Asda2QuestHandler.cs keeps the clicked
	// NPC target id at payload offset 20 and accepts either int32 or uint16.
	clientUiActionTargetIDOffset = 20
)

func handleClientUiActionRequest(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil || p == nil {
		return
	}
	targetID := readClientUiActionTargetID(p.Data)
	if targetID == 0 {
		clearNpcInteraction(c)
		return
	}
	npc := currentMapNpcByClientTarget(c, targetID)
	if npc == nil {
		clearNpcInteraction(c)
		return
	}
	dispatchNpcInteraction(c, npc, targetID)
}

func readClientUiActionTargetID(raw []byte) uint16 {
	if len(raw) < clientUiActionTargetIDOffset+2 {
		return 0
	}
	if len(raw) >= clientUiActionTargetIDOffset+4 {
		v := uint32(raw[clientUiActionTargetIDOffset]) |
			uint32(raw[clientUiActionTargetIDOffset+1])<<8 |
			uint32(raw[clientUiActionTargetIDOffset+2])<<16 |
			uint32(raw[clientUiActionTargetIDOffset+3])<<24
		if v > 0 && v <= 0xFFFF {
			return uint16(v)
		}
	}
	return uint16(raw[clientUiActionTargetIDOffset]) | uint16(raw[clientUiActionTargetIDOffset+1])<<8
}

func currentMapNpcByClientTarget(c *Client, targetID uint16) *Npc {
	if c == nil || c.Char == nil || targetID == 0 {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}
	npc, ok := gm.FindNpcByClientTargetID(targetID)
	if !ok {
		return nil
	}
	return npc
}
