package main

import "log"

func damageCharacter(c *Client, damage int32, source string) bool {
	if c == nil || c.Char == nil || damage <= 0 {
		return false
	}
	if c.Char.HP <= 0 {
		return true
	}

	c.Char.HP -= damage
	if c.Char.HP < 0 {
		c.Char.HP = 0
	}
	sendCharacterHealthUpdate(c)

	if c.Char.HP == 0 {
		killCharacter(c, source)
		return true
	}
	log.Printf("[Combat] %q took %d damage from %s hp=%d/%d", c.Char.Name, damage, source, c.Char.HP, c.Char.MaxHP)
	return false
}

func healCharacter(c *Client, amount int32, source string) bool {
	if c == nil || c.Char == nil || amount <= 0 {
		return false
	}

	wasDead := c.Char.HP <= 0
	c.Char.HP += amount
	if c.Char.HP > c.Char.MaxHP {
		c.Char.HP = c.Char.MaxHP
	}
	if c.Char.HP < 1 {
		c.Char.HP = 1
	}

	if wasDead {
		prepareCharacterRespawnAtCurrent(c)
		c.Char.IsFighting = false
		c.Char.TargetID = -1
		sendResurrectResponse(c)
	} else {
		sendCharacterHealthUpdate(c)
	}
	log.Printf("[Combat] %q healed %d by %s hp=%d/%d", c.Char.Name, amount, source, c.Char.HP, c.Char.MaxHP)
	return wasDead
}

func killCharacter(c *Client, source string) {
	if c == nil || c.Char == nil {
		return
	}

	c.Char.HP = 0
	c.Char.IsFighting = false
	stopCharacterMovementAtCurrent(c)
	c.Char.TargetID = -1
	if gm := World.GetMap(c.Char.MapID); gm != nil {
		gm.ClearMonsterTargetsFor(c.Char.SessionID)
	}
	sendSelfDeathResponse(c)
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[DB] failed to save death for %q: %v", c.Char.Name, err)
	}
	log.Printf("[Combat] %q died from %s", c.Char.Name, source)
}

func sendSelfDeathResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SelfDeath)
	p.WriteInt16(c.Char.SessionID)
	c.SendToArea(p)
}

func sendPreResurrectResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(PreResurect)
	p.WriteUint8(1)
	c.Send(p)
}

func sendResurrectResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	sendPreResurrectResponse(c)
	if gm := World.GetMap(c.Char.MapID); gm != nil {
		sessionID := c.Char.SessionID
		gm.CallDelayed(50, func() {
			if c.Char == nil || c.Char.SessionID != sessionID || c.Char.HP <= 0 {
				return
			}
			sendResurrectStateResponse(c)
			scheduleRespawnFacingSync(c, sessionID)
		})
		return
	}

	sendResurrectStateResponse(c)
	scheduleRespawnFacingSync(c, c.Char.SessionID)
}

func sendResurrectStateResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(Resurect)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(int16(asda2X(c.Char.X, c.Char.MapID)))
	p.WriteInt16(int16(asda2Y(c.Char.Y, c.Char.MapID)))
	p.WriteInt32(c.Char.HP)
	p.WriteInt16(int16(c.Char.MP))
	p.WriteInt64(characterTotalXP(c.Char))
	p.WriteInt16(1)
	c.SendToArea(p)
}

func scheduleRespawnFacingSync(c *Client, sessionID int16) {
	if c == nil || c.Char == nil {
		return
	}
	if gm := World.GetMap(c.Char.MapID); gm != nil {
		gm.CallDelayed(100, func() {
			if c.Char != nil && c.Char.SessionID == sessionID && c.Char.HP > 0 {
				sendInstantMovementToArea(c, "respawn")
			}
		})
	}
}
