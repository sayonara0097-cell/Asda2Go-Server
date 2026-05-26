package main

import "log"

// ---- Movement ----

func handleStartMove(c *Client, p *PacketIn) {
	// Mirrors Asda2MovmentHandler.StartMoveRequest
	// C#: game packet constructor lands at payload+28, then -= 24, += 16, += 5 → offset 25
	// Client sends LOCAL (Asda2) coords * 100. Server converts to world for storage.
	if c.Char == nil || c.Char.HP <= 0 {
		return
	}
	p.Seek(25)
	fromXRaw := p.ReadFloat32()
	fromYRaw := p.ReadFloat32()
	destXRaw := p.ReadFloat32()
	destYRaw := p.ReadFloat32()
	localFromX := normalizeLocalCoordinate(fromXRaw) // current local x
	localFromY := normalizeLocalCoordinate(fromYRaw) // current local y
	localDestX := normalizeLocalCoordinate(destXRaw) // destination local x
	localDestY := normalizeLocalCoordinate(destYRaw) // destination local y
	p.Skip(4)
	target := p.ReadInt16()
	rawTarget := target
	if !validLocalPosition(localFromX, localFromY) || !validLocalPosition(localDestX, localDestY) {
		log.Printf("[Move] invalid start %q from %.2f,%.2f to %.2f,%.2f", c.Char.Name, localFromX, localFromY, localDestX, localDestY)
		return
	}

	target = movementAreaTargetID(c, target)
	if c.Char.RunSpeed <= 0 {
		c.Char.RunSpeed = defaultCharacterRunSpeed
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localFromX + offset
	c.Char.Y = localFromY + offset
	c.Char.MoveDestX = localDestX
	c.Char.MoveDestY = localDestY
	c.Char.TargetID = target
	c.Char.IsMoving = true

	log.Printf("[Move] start %q from %.2f,%.2f to %.2f,%.2f target=%d", c.Char.Name, localFromX, localFromY, localDestX, localDestY, target)

	World.RefreshCharacterVisibility(c)
	if npcServerClient != nil {
		World.RefreshNpcServerVisibility(c, false)
	} else {
		World.RefreshNpcVisibility(c)
		World.RefreshMonsterVisibility(c)
	}
	World.RefreshPortalVisibility(c)
	if World.CheckPortalTriggers(c) {
		return
	}

	// Mirrors CreateStartComonMovePacket: status OK, current position, target position, run speed.
	resp := NewPacket(StartMoveCommon)
	resp.WriteUint8(1)
	resp.WriteInt16(c.Char.SessionID)
	resp.WriteUint32(c.Char.AccID)
	resp.WriteInt16(2)
	resp.WriteFloat32(fromXRaw)
	resp.WriteFloat32(fromYRaw)
	resp.WriteFloat32(destXRaw)
	resp.WriteFloat32(destYRaw)
	resp.WriteFloat32(c.Char.RunSpeed)
	resp.WriteInt16(target)
	sendMovementToArea(c, resp, "start", rawTarget, target)
}

func handleEndMove(c *Client, p *PacketIn) {
	// Mirrors Asda2MovmentHandler.EndMoveRequest
	// C#: game packet constructor lands at payload+28, then -= 3 → offset 25
	// Client sends LOCAL (Asda2) coords * 100.
	if c.Char == nil || c.Char.HP <= 0 {
		return
	}
	p.Seek(25)
	// All four floats are LOCAL coords * 100 (current start pos, then end pos)
	cx := p.ReadFloat32() // current x * 100 (local)
	cy := p.ReadFloat32() // current y * 100 (local)
	ex := p.ReadFloat32() // end x * 100 (local)
	ey := p.ReadFloat32() // end y * 100 (local)
	localCurrentX := normalizeLocalCoordinate(cx)
	localCurrentY := normalizeLocalCoordinate(cy)
	localEndX := normalizeLocalCoordinate(ex)
	localEndY := normalizeLocalCoordinate(ey)
	p.Skip(4)
	target := p.ReadInt16()
	rawTarget := target
	if !validLocalPosition(localCurrentX, localCurrentY) || !validLocalPosition(localEndX, localEndY) {
		log.Printf("[Move] invalid end %q current %.2f,%.2f end %.2f,%.2f", c.Char.Name, localCurrentX, localCurrentY, localEndX, localEndY)
		return
	}

	log.Printf("[Move] end %q at %.2f,%.2f target=%d", c.Char.Name, localEndX, localEndY, target)

	target = movementAreaTargetID(c, target)
	offset := mapOffset(c.Char.MapID)
	c.Char.X = localEndX + offset
	c.Char.Y = localEndY + offset
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.TargetID = target
	c.Char.IsMoving = false
	World.RefreshCharacterVisibility(c)
	if npcServerClient != nil {
		World.RefreshNpcServerVisibility(c, false)
	} else {
		World.RefreshNpcVisibility(c)
		World.RefreshMonsterVisibility(c)
	}
	World.RefreshPortalVisibility(c)
	if World.CheckPortalTriggers(c) {
		return
	}

	// Echo back exactly what client sent — mirrors EndMoveCommon
	resp := NewPacket(EndMoveCommon)
	resp.WriteUint8(0)
	resp.WriteInt16(c.Char.SessionID)
	resp.WriteUint32(c.Char.AccID)
	resp.WriteInt16(0)
	resp.WriteFloat32(cx)
	resp.WriteFloat32(cy)
	resp.WriteFloat32(ex)
	resp.WriteFloat32(ey)
	resp.WriteInt32(0)
	resp.WriteInt16(target)
	sendMovementToArea(c, resp, "end", rawTarget, target)
}

func sendMovementToArea(c *Client, p *PacketOut, phase string, rawTarget int16, sentTarget int16) {
	recipients := World.AreaRecipients(c, true)
	debugVisibilityf("move %s source=%s rawTarget=%d sentTarget=%d recipients=%s",
		phase, clientDebugLabel(c), rawTarget, sentTarget, clientListDebugLabel(recipients))
	for _, other := range recipients {
		other.Send(p)
	}
}

func movementAreaTargetID(c *Client, target int16) int16 {
	if c == nil || c.Char == nil || target <= 0 {
		return -1
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		debugVisibilityf("movement target ignored source=%s rawTarget=%d reason=no-map", clientDebugLabel(c), target)
		return -1
	}
	if _, ok := gm.FindMonsterByClientTargetID(uint16(target)); ok {
		return target
	}
	if _, ok := gm.FindNpcByClientTargetID(uint16(target)); ok {
		return target
	}
	debugVisibilityf("movement target ignored source=%s rawTarget=%d reason=no-monster-or-npc", clientDebugLabel(c), target)
	return -1
}
