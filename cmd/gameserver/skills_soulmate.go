package main

import (
	"log"
	"time"
)

type soulmateSkillStatus byte

const (
	soulmateSkillFail            soulmateSkillStatus = 0
	soulmateSkillOK              soulmateSkillStatus = 1
	soulmateSkillNoFriend        soulmateSkillStatus = 2
	soulmateSkillFriendNotInGame soulmateSkillStatus = 3
	soulmateSkillTooFar          soulmateSkillStatus = 7
	soulmateSkillLowMana         soulmateSkillStatus = 8
	soulmateSkillCantUseMoving   soulmateSkillStatus = 12
	soulmateSkillYouAreDead      soulmateSkillStatus = 13
	soulmateSkillFriendDead      soulmateSkillStatus = 14
	soulmateSkillCooldown        soulmateSkillStatus = 16
)

type soulmateSkillTemplate struct {
	ID            int16
	RequiredLevel byte
	Cooldown      time.Duration
	Range         float64
	MPCost        int16
}

var soulmateSkillTemplates = map[int16]soulmateSkillTemplate{
	32: {ID: 32, RequiredLevel: 3, Cooldown: 20 * time.Minute, Range: 0, MPCost: 10},
	33: {ID: 33, RequiredLevel: 5, Cooldown: 60 * time.Second, Range: 40, MPCost: 15},
	34: {ID: 34, RequiredLevel: 5, Cooldown: 60 * time.Second, Range: 40, MPCost: 15},
	35: {ID: 35, RequiredLevel: 10, Cooldown: 20 * time.Minute, Range: 40, MPCost: 20},
	37: {ID: 37, RequiredLevel: 10, Cooldown: 60 * time.Second, Range: 40, MPCost: 20},
	39: {ID: 39, RequiredLevel: 18, Cooldown: 10 * time.Minute, Range: 40, MPCost: 25},
	40: {ID: 40, RequiredLevel: 20, Cooldown: 20 * time.Minute, Range: 0, MPCost: 25},
	42: {ID: 42, RequiredLevel: 20, Cooldown: 60 * time.Second, Range: 40, MPCost: 25},
	43: {ID: 43, RequiredLevel: 20, Cooldown: 60 * time.Second, Range: 40, MPCost: 25},
	44: {ID: 44, RequiredLevel: 5, Cooldown: 12 * time.Hour, Range: 40, MPCost: 30},
}

func useSoulmateSkill(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil || p.Remaining() < 2 {
		return
	}
	skillID := p.ReadInt16()
	targetSessionID := readSoulmateSkillTargetSession(p.Data)
	status := applySoulmateSkill(c, skillID, targetSessionID)
	if status != soulmateSkillOK {
		sendSoulmateSkillUsed(c, status, 0, 0)
	}
}

func readSoulmateSkillTargetSession(data []byte) int16 {
	if len(data) >= 116 {
		in := &PacketIn{Data: data[114:]}
		return in.ReadInt16()
	}
	for offset := len(data) - 2; offset >= 2; offset -= 2 {
		in := &PacketIn{Data: data[offset:]}
		value := in.ReadInt16()
		if value > 0 {
			return value
		}
	}
	return 0
}

func applySoulmateSkill(c *Client, skillID int16, targetSessionID int16) soulmateSkillStatus {
	template, ok := soulmateSkillTemplates[skillID]
	if !ok {
		return soulmateSkillFail
	}
	friend := activeSoulmate(c)
	if friend == nil {
		if c.Char.SoulmateGUID == 0 {
			return soulmateSkillNoFriend
		}
		return soulmateSkillFriendNotInGame
	}
	if c.Char.HP <= 0 {
		return soulmateSkillYouAreDead
	}
	if c.Char.IsMoving {
		return soulmateSkillCantUseMoving
	}
	if c.Char.MP < int32(template.MPCost) {
		return soulmateSkillLowMana
	}
	if effectiveSoulmateLevel(c.Char) < template.RequiredLevel {
		return soulmateSkillFail
	}
	if remaining := soulmateSkillCooldownRemaining(c, skillID); remaining > 0 {
		return soulmateSkillCooldown
	}
	if template.Range > 0 && characterDistance(c.Char, friend.Char) > template.Range {
		return soulmateSkillTooFar
	}

	switch skillID {
	case 33, 37, 42:
		if friend.Char.HP <= 0 {
			return soulmateSkillFriendDead
		}
		healCharacter(friend, soulmateHealAmount(friend, skillID), "soulmate")
	case 34, 43:
		applySoulmateEmpower(c, friend, skillID)
	case 35:
		applySoulmateSoulSave(friend)
	case 39:
		if friend.Char.HP > 0 {
			return soulmateSkillFail
		}
		healCharacter(friend, friend.Char.MaxHP/2, "soulmate-resurrect")
	case 32:
		sendSoulmateSummoningYou(c, friend)
	case 40:
		teleportClientToClient(c, friend)
	case 44:
		applySoulmateSong(c, friend, skillID)
	}

	c.Char.MP -= int32(template.MPCost)
	if c.Char.MP < 0 {
		c.Char.MP = 0
	}
	sendCharacterMPUpdate(c)
	setSoulmateSkillCooldown(c, skillID, template.Cooldown)
	sendSoulmateSkillCast(c, targetSessionID, skillID)
	sendSoulmateSkillUsed(c, soulmateSkillOK, skillID, targetSessionID)
	sendSoulmateSkillUsed(friend, soulmateSkillOK, skillID, targetSessionID)
	log.Printf("[SoulmateSkill] %q used skill=%d friend=%q", c.Char.Name, skillID, friend.Char.Name)
	return soulmateSkillOK
}

func activeSoulmate(c *Client) *Client {
	if c == nil || c.Char == nil || c.Char.SoulmateGUID == 0 {
		return nil
	}
	friend := getClientByGUID(c.Char.SoulmateGUID)
	if friend == nil || friend.Char == nil || friend.Char.SoulmateGUID != c.Char.GUID {
		return nil
	}
	return friend
}

func effectiveSoulmateLevel(chr *Character) byte {
	if chr == nil || chr.SoulmateGUID == 0 {
		return 0
	}
	if chr.SoulmateLevel == 0 {
		return 255
	}
	return chr.SoulmateLevel
}

func soulmateSkillCooldownRemaining(c *Client, skillID int16) time.Duration {
	if c == nil || c.Char == nil || c.Char.SoulmateSkillCooldowns == nil {
		return 0
	}
	return time.Until(c.Char.SoulmateSkillCooldowns[skillID])
}

func setSoulmateSkillCooldown(c *Client, skillID int16, cooldown time.Duration) {
	if c == nil || c.Char == nil || cooldown <= 0 {
		return
	}
	if c.Char.SoulmateSkillCooldowns == nil {
		c.Char.SoulmateSkillCooldowns = make(map[int16]time.Time)
	}
	c.Char.SoulmateSkillCooldowns[skillID] = time.Now().Add(cooldown)
}

func soulmateHealAmount(friend *Client, skillID int16) int32 {
	if friend == nil || friend.Char == nil {
		return 0
	}
	switch skillID {
	case 42:
		return friend.Char.MaxHP * 75 / 100
	case 37:
		return friend.Char.MaxHP * 60 / 100
	default:
		return friend.Char.MaxHP * 50 / 100
	}
}

func applySoulmateEmpower(c *Client, friend *Client, skillID int16) {
	duration := time.Minute
	value := int32(10)
	if skillID == 43 {
		value = 15
	}
	for _, target := range []*Client{c, friend} {
		skill := SkillTemplate{
			ID:                skillID,
			Level:             1,
			Duration:          duration,
			Effect0Type:       spellEffectApplyAura,
			Effect0BasePoints: value,
			Effect0Misc:       skillID,
		}
		applySkillBuff(target, skill)
	}
}

func applySoulmateSoulSave(friend *Client) {
	if friend == nil || friend.Char == nil {
		return
	}
	skill := SkillTemplate{
		ID:                35,
		Level:             1,
		Duration:          10 * time.Minute,
		Effect0Type:       spellEffectApplyAura,
		Effect0BasePoints: 10,
		Effect0Misc:       35,
	}
	applySkillBuff(friend, skill)
}

func applySoulmateSong(c *Client, friend *Client, skillID int16) {
	for _, target := range []*Client{c, friend} {
		skill := SkillTemplate{
			ID:                skillID,
			Level:             1,
			Duration:          12 * time.Hour,
			Effect0Type:       spellEffectApplyAura,
			Effect0BasePoints: 5,
			Effect0Misc:       skillID,
		}
		applySkillBuff(target, skill)
	}
}

func sendSoulmateSkillCast(c *Client, targetSessionID int16, skillID int16) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SoulmateSkillCast)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skillID)
	p.WriteBytes(make([]byte, 112))
	p.WriteInt16(targetSessionID)
	p.WriteInt16(targetSessionID)
	p.WriteBytes(make([]byte, 58))
	p.WriteUint8(byte(c.Char.MapID))
	p.WriteBytes(make([]byte, 17))
	c.SendToArea(p)
}

func sendSoulmateSkillUsed(c *Client, status soulmateSkillStatus, skillID int16, targetSessionID int16) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SoulmateSkillUsed)
	p.WriteUint8(byte(status))
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skillID)
	p.WriteBytes(make([]byte, 112))
	p.WriteInt16(targetSessionID)
	p.WriteInt16(targetSessionID)
	p.WriteBytes(make([]byte, 58))
	p.WriteUint8(byte(c.Char.MapID))
	p.WriteBytes(make([]byte, 17))
	c.Send(p)
}

func sendSoulmateSummoningYou(c *Client, friend *Client) {
	if c == nil || c.Char == nil || friend == nil || friend.Char == nil {
		return
	}
	friend.Char.CanTeleportToFriend = true
	friend.Char.TargetSummonMap = c.Char.MapID
	friend.Char.TargetSummonX = c.Char.X
	friend.Char.TargetSummonY = c.Char.Y

	p := NewPacket(SoulmateSummoningYou)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint8(0)
	p.WriteInt16(int16(c.Char.MapID))
	p.WriteUint8(1)
	p.WriteAsdaString(gamePublicIP, 16)
	p.WriteInt16(int16(gamePublicPort))
	p.WriteInt16(int16(asda2X(c.Char.X, c.Char.MapID)))
	p.WriteInt16(int16(asda2Y(c.Char.Y, c.Char.MapID)))
	p.WriteUint32(friend.Char.AccID)
	p.WriteInt16(friend.Char.SessionID)
	p.WriteUint8(0)
	p.WriteInt16(int16(c.Char.MapID))
	p.WriteUint8(1)
	friend.Send(p)
}

func teleportClientToClient(c *Client, target *Client) {
	if c == nil || c.Char == nil || target == nil || target.Char == nil {
		return
	}
	teleportClientToWorld(c, target.Char.MapID, target.Char.X, target.Char.Y)
}
