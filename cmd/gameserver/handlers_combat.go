package main

import "log"

func handleStartAtack(c *Client, p *PacketIn) {
	// Mirrors Asda2CombatHandler.StartAtackRequest
	if c.Char == nil || c.Char.HP <= 0 {
		return
	}
	targetID, target := resolveStartAttackTarget(c, p)
	status := normalAttackStartStatus(c, target)
	if status == normalAttackStatusOK {
		c.Char.TargetID = target.SessionID
		c.Char.IsFighting = true
		if gm := World.GetMap(c.Char.MapID); gm != nil {
			gm.AggroMonster(target, c)
		}
	} else {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
	}

	if target == nil {
		log.Printf("[Combat] %q start attack failed: target=%d payload=% X map=%d", c.Char.Name, targetID, p.Data, c.Char.MapID)
	} else {
		log.Printf("[Combat] %q start attack monster target=%d session=%d world=%d entry=%d status=%d",
			c.Char.Name, targetID, target.SessionID, target.WorldEntityID, target.EntryID, status)
	}
	sendStartAttackResponse(c, target, status, int16(targetID))
	if status == normalAttackStatusOK {
		sendSetAttackStateGUI(c)
	}
}

func handleContinueAtack(c *Client, p *PacketIn) {
	if c.Char == nil || c.Char.HP <= 0 {
		return
	}
	target := currentMapMonster(c, c.Char.TargetID)
	if target == nil && p.Remaining() >= 2 {
		target = currentMapMonsterByClientTarget(c, p.ReadUint16())
	}
	if target == nil || target.State != MonsterStateOK || target.Health <= 0 {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
		return
	}
	c.Char.TargetID = target.SessionID
	_, killed, status := applyNormalMonsterAttackPulse(c, target, "continue")
	if status != normalAttackStatusOK {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
		sendStartAttackResponse(c, target, status, target.SessionID)
		return
	}
	if killed {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
	}
}

func handleStartAtackCharacter(c *Client, p *PacketIn) {
	// TODO: PvP attack initiation
}

func handleAtackCharacter(c *Client, p *PacketIn) {
	if c.Char == nil || c.Char.HP <= 0 || p.Remaining() < 2 {
		return
	}
	target := currentMapMonsterByClientTarget(c, p.ReadUint16())
	if target == nil || target.State != MonsterStateOK || target.Health <= 0 {
		return
	}
	c.Char.TargetID = target.SessionID
	c.Char.IsFighting = true
	_, killed, status := applyNormalMonsterAttackPulse(c, target, "attack_character")
	if status != normalAttackStatusOK {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
		sendStartAttackResponse(c, target, status, target.SessionID)
		return
	}
	if killed {
		c.Char.TargetID = -1
		c.Char.IsFighting = false
	}
}

func currentMapMonster(c *Client, sessionID int16) *Monster {
	if c == nil || c.Char == nil || sessionID <= 0 {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}
	monster, ok := gm.FindMonsterBySessionID(sessionID)
	if !ok {
		return nil
	}
	if npcServerClient != nil && !gm.KnowsMonster(c.ID, monster.SessionID) {
		return nil
	}
	return monster
}

func currentMapMonsterByEntry(c *Client, entryID uint16) *Monster {
	if c == nil || c.Char == nil || entryID == 0 {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}
	monster, ok := gm.FindMonsterByEntryID(entryID)
	if !ok {
		return nil
	}
	if npcServerClient != nil && !gm.KnowsMonster(c.ID, monster.SessionID) {
		return nil
	}
	return monster
}

func currentMapMonsterByClientTarget(c *Client, targetID uint16) *Monster {
	if c == nil || c.Char == nil || targetID == 0 {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}
	monster, ok := gm.FindMonsterByClientTargetID(targetID)
	if !ok {
		return nil
	}
	if npcServerClient != nil && !gm.KnowsMonster(c.ID, monster.SessionID) {
		return nil
	}
	return monster
}

func sendStartAttackResponse(c *Client, target *Monster, status byte, requestedTarget int16) {
	if c == nil || c.Char == nil {
		return
	}
	targetSession := requestedTarget
	targetEntity := int32(requestedTarget)
	if target != nil {
		targetSession = target.SessionID
		targetEntity = target.WorldEntityID
	} else {
		targetSession = -1
		targetEntity = -1
	}
	resp := NewPacket(StartAtackResponse)
	resp.WriteUint8(status)
	resp.WriteInt16(c.Char.SessionID)
	resp.WriteInt16(targetSession)
	resp.WriteInt16(0)
	resp.WriteInt16(0)
	resp.WriteInt32(targetEntity)
	c.SendToArea(resp)
}

func sendSetAttackStateGUI(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SetAtackStateGui)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint32(c.Char.AccID)
	c.SendToArea(p)
}

func sendCharacterHealthUpdate(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(CharHpUpdate)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt32(c.Char.MaxHP)
	p.WriteInt32(c.Char.HP)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteInt32(0)
	c.SendToArea(p)
}
