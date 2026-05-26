package main

import (
	"time"

	"asda2/shared/types"
)

const (
	firstSoulGuardSkillID       int16 = 2228
	legacyFirstSoulGuardSkillID int16 = 1228
	soulGuardSkillCount               = 27
	soulGuardChargeCost         byte  = 5
	soulGuardMaxChargeCount     byte  = 10
	soulGuardActivationCount    byte  = 8
)

var soulGuardVisualByID = map[int16]int16{
	0: 237, 1: 228, 2: 220,
	3: 226, 4: 224, 5: 220,
	6: 233, 7: 219, 8: 220,
	9: 244, 10: 250, 11: 238,
	12: 241, 13: 242, 14: 238,
	15: 237, 16: 219, 17: 238,
	18: 252, 19: 254, 20: 270,
	21: 255, 22: 300, 23: 299,
	24: 264, 25: 270, 26: 270,
}

func normalizeSoulGuardSkillID(skillID int16) int16 {
	if skillID >= legacyFirstSoulGuardSkillID && skillID < legacyFirstSoulGuardSkillID+soulGuardSkillCount {
		return skillID + (firstSoulGuardSkillID - legacyFirstSoulGuardSkillID)
	}
	return skillID
}

func soulGuardUseStatus(c *Client, requestedSkillID int16, skill SkillTemplate) skillUseResult {
	if c == nil || c.Char == nil {
		return skillUseCannotApply
	}
	level := soulGuardLevelForSkill(skill)
	if level < 1 || level > 3 {
		return skillUseItIsNotAnActiveSkill
	}
	if !soulGuardSkillMatchesCharacter(c.Char, normalizeSoulGuardSkillID(requestedSkillID), level) {
		return skillUseWrongJob
	}
	if !hasSoulGuardCharges(c.Char, level) {
		return skillUseCannotApply
	}
	return skillUseOK
}

func soulGuardLevelForSkill(skill SkillTemplate) byte {
	if skill.SoulGuardLevel > 0 {
		return skill.SoulGuardLevel
	}
	if skill.ID >= firstSoulGuardSkillID && skill.ID < firstSoulGuardSkillID+soulGuardSkillCount {
		return byte((skill.ID-firstSoulGuardSkillID)%3 + 1)
	}
	return 0
}

func soulGuardSkillMatchesCharacter(chr *Character, skillID int16, level byte) bool {
	first := firstSoulGuardSkillForClass(chr.Class)
	return first > 0 && skillID == first+int16(level)-1
}

func firstSoulGuardSkillForClass(classID byte) int16 {
	switch types.Asda2Class(classID) {
	case types.Asda2ClassOHS:
		return 2228
	case types.Asda2ClassSpear:
		return 2231
	case types.Asda2ClassTHS:
		return 2234
	case types.Asda2ClassCrossbow:
		return 2237
	case types.Asda2ClassBow:
		return 2240
	case types.Asda2ClassBalista:
		return 2243
	case types.Asda2ClassHealMage:
		return 2246
	case types.Asda2ClassAttackMage:
		return 2249
	case types.Asda2ClassSupportMage:
		return 2252
	default:
		return 0
	}
}

func soulGuardBaseIDForClass(classID byte) int16 {
	switch types.Asda2Class(classID) {
	case types.Asda2ClassOHS:
		return 0
	case types.Asda2ClassSpear:
		return 3
	case types.Asda2ClassTHS:
		return 6
	case types.Asda2ClassCrossbow:
		return 9
	case types.Asda2ClassBow:
		return 12
	case types.Asda2ClassBalista:
		return 15
	case types.Asda2ClassHealMage:
		return 18
	case types.Asda2ClassSupportMage:
		return 21
	case types.Asda2ClassAttackMage:
		return 24
	default:
		return -1
	}
}

func hasSoulGuardCharges(chr *Character, level byte) bool {
	switch level {
	case 1:
		return chr.GreenCharges >= soulGuardChargeCost
	case 2:
		return chr.BlueCharges >= soulGuardChargeCost
	case 3:
		return chr.RedCharges >= soulGuardChargeCost
	default:
		return false
	}
}

func consumeSoulGuardForSkill(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	switch soulGuardLevelForSkill(skill) {
	case 1:
		c.Char.GreenCharges = spendSoulGuardCharge(c.Char.GreenCharges)
	case 2:
		c.Char.BlueCharges = spendSoulGuardCharge(c.Char.BlueCharges)
	case 3:
		c.Char.RedCharges = spendSoulGuardCharge(c.Char.RedCharges)
	}
	sendSetSkillPowersStats(c, true, skill.ID)
	sessionID := c.Char.SessionID
	if gm := World.GetMap(c.Char.MapID); gm != nil {
		gm.CallDelayed(4000, func() {
			current := getClientBySessionID(sessionID)
			refreshSoulGuard(current, false, 0)
		})
		return
	}
	time.AfterFunc(4*time.Second, func() {
		current := getClientBySessionID(sessionID)
		refreshSoulGuard(current, false, 0)
	})
}

func spendSoulGuardCharge(charges byte) byte {
	if charges < soulGuardChargeCost {
		return charges
	}
	return charges - soulGuardChargeCost
}

func addSoulGuardChargesForSkill(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	addTimedSoulGuardCharge(c, 0)
	if skill.RequiredLevel >= 30 {
		addTimedSoulGuardCharge(c, 1)
	}
	if skill.RequiredLevel >= 50 {
		addTimedSoulGuardCharge(c, 2)
	}
	refreshSoulGuard(c, true, skill.ID)
}

func addTimedSoulGuardCharge(c *Client, tier int) {
	if c == nil || c.Char == nil {
		return
	}
	switch tier {
	case 0:
		if c.Char.GreenCharges < soulGuardMaxChargeCount {
			c.Char.GreenCharges++
		}
	case 1:
		if c.Char.BlueCharges < soulGuardMaxChargeCount {
			c.Char.BlueCharges++
		}
	case 2:
		if c.Char.RedCharges < soulGuardMaxChargeCount {
			c.Char.RedCharges++
		}
	default:
		return
	}
	sessionID := c.Char.SessionID
	time.AfterFunc(10*time.Second, func() {
		current := getClientBySessionID(sessionID)
		expireSoulGuardCharge(current, tier)
	})
}

func expireSoulGuardCharge(c *Client, tier int) {
	if c == nil || c.Char == nil {
		return
	}
	switch tier {
	case 0:
		if c.Char.GreenCharges > 0 {
			c.Char.GreenCharges--
		}
	case 1:
		if c.Char.BlueCharges > 0 {
			c.Char.BlueCharges--
		}
	case 2:
		if c.Char.RedCharges > 0 {
			c.Char.RedCharges--
		}
	}
	refreshSoulGuard(c, false, 0)
}

func refreshSoulGuard(c *Client, animate bool, skillID int16) {
	if c == nil || c.Char == nil {
		return
	}
	base := soulGuardBaseIDForClass(c.Char.Class)
	if base < 0 || c.Char.GreenCharges < soulGuardActivationCount {
		clearSoulGuard(c)
		sendSetSkillPowersStats(c, animate, skillID)
		return
	}

	setSoulGuardTier(c, base, 0, c.Char.GreenCharges >= soulGuardActivationCount)
	setSoulGuardTier(c, base, 1, c.Char.GreenCharges >= soulGuardActivationCount && c.Char.BlueCharges >= soulGuardActivationCount)
	setSoulGuardTier(c, base, 2, c.Char.GreenCharges >= soulGuardActivationCount && c.Char.BlueCharges >= soulGuardActivationCount && c.Char.RedCharges >= soulGuardActivationCount)
	showSoulGuardIfChanged(c, highestActiveSoulGuardID(c.Char, base))
	sendSetSkillPowersStats(c, animate, skillID)
}

func highestActiveSoulGuardID(chr *Character, base int16) int16 {
	if chr.SoulBuffed3 {
		return base + 2
	}
	if chr.SoulBuffed2 {
		return base + 1
	}
	if chr.SoulBuffed1 {
		return base
	}
	return -1
}

func clearSoulGuard(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	base := soulGuardBaseIDForClass(c.Char.Class)
	for tier := 0; tier < 3; tier++ {
		setSoulGuardTier(c, base, tier, false)
	}
	showSoulGuardIfChanged(c, -1)
}

func setSoulGuardTier(c *Client, base int16, tier int, active bool) {
	if c == nil || c.Char == nil || base < 0 {
		return
	}
	was := soulGuardTierActive(c.Char, tier)
	if was == active {
		return
	}
	soulID := base + int16(tier)
	applySoulGuardStatEffect(c.Char, soulID, active)
	setSoulGuardTierActive(c.Char, tier, active)
	if active {
		sendSoulGuardBuff(c, soulID, -1)
	} else {
		sendSoulGuardBuffEnded(c, soulID)
	}
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
}

func soulGuardTierActive(chr *Character, tier int) bool {
	switch tier {
	case 0:
		return chr.SoulBuffed1
	case 1:
		return chr.SoulBuffed2
	case 2:
		return chr.SoulBuffed3
	default:
		return false
	}
}

func setSoulGuardTierActive(chr *Character, tier int, active bool) {
	switch tier {
	case 0:
		chr.SoulBuffed1 = active
	case 1:
		chr.SoulBuffed2 = active
	case 2:
		chr.SoulBuffed3 = active
	}
}

func applySoulGuardStatEffect(chr *Character, soulID int16, apply bool) {
	sign := float32(1)
	if !apply {
		sign = -1
	}
	switch soulID {
	case 0:
		chr.SkillDefenseBonusPct += 0.05 * sign
	case 1:
		chr.SkillDamageBonusPct += 0.10 * sign
	case 2:
		chr.SkillDefenseBonusPct += 0.15 * sign
	case 3, 6, 9, 15:
		chr.SkillDamageBonusPct += 0.05 * sign
	case 4:
		chr.SkillDamageBonusPct += 0.10 * sign
	case 5, 14:
		chr.SkillSpeedBonusPct += 0.10 * sign
	case 7:
		chr.SkillDamageBonusPct += 0.10 * sign
	case 8, 11, 17:
		chr.SkillDamageBonusPct += 0.15 * sign
	case 10:
		chr.SkillDamageBonusPct += 0.10 * sign
	case 12:
		chr.SkillDamageBonusPct += 0.05 * sign
	case 13:
		chr.SkillDamageBonusPct += 0.10 * sign
	case 16:
		chr.SkillDefenseBonusPct += 0.10 * sign
	case 18:
		chr.SkillMagicDamageBonusPct += 0.05 * sign
		chr.SkillHealingDonePct += 0.05 * sign
	case 19:
		chr.SkillMagicDamageBonusPct += 0.10 * sign
		chr.SkillHealingDonePct += 0.10 * sign
	case 20:
		chr.SkillHealingDonePct += 0.15 * sign
	case 21:
		chr.SkillMagicDamageBonusPct += 0.05 * sign
	case 22:
		chr.SkillDefenseBonusPct += 0.10 * sign
	case 23:
		chr.SkillMagicDefenseBonusPct += 0.15 * sign
	case 24:
		chr.SkillMagicDamageBonusPct += 0.05 * sign
	case 25:
		chr.SkillMagicDamageBonusPct += 0.10 * sign
	case 26:
		chr.SkillMagicDamageBonusPct += 0.15 * sign
	}
}

func showSoulGuardIfChanged(c *Client, soulID int16) {
	if c == nil || c.Char == nil || c.Char.CurrentSoulGuardID == soulID {
		return
	}
	c.Char.CurrentSoulGuardID = soulID
	p := NewPacket(ShowSoulGuardResponse)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(displaySoulGuardID(soulID))
	c.SendToArea(p)
}

func displaySoulGuardID(soulID int16) int16 {
	if soulID >= 18 && soulID <= 23 {
		return soulID + 3
	}
	if soulID >= 24 && soulID <= 26 {
		return soulID - 6
	}
	return soulID
}

func sendSoulGuardBuff(c *Client, soulID int16, durationSeconds int32) {
	auraID, ok := soulGuardVisualByID[soulID]
	if !ok {
		return
	}
	p := NewPacket(BuffsOnCharacterInfo)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(0)
	p.WriteInt32(int32(auraID))
	p.WriteInt32(durationSeconds)
	p.WriteInt16(0)
	c.SendToArea(p)
}

func sendSoulGuardBuffEnded(c *Client, soulID int16) {
	auraID, ok := soulGuardVisualByID[soulID]
	if !ok {
		return
	}
	sendBuffEndedForCharacter(c, auraID)
}

func sendSetSkillPowersStats(c *Client, animate bool, skillID int16) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SetSkiillPowersStats)
	p.WriteInt32(355335)
	if animate {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteUint8(c.Char.Class)
	p.WriteInt16(skillID)
	p.WriteUint8(c.Char.GreenCharges)
	p.WriteUint8(c.Char.BlueCharges)
	p.WriteUint8(c.Char.RedCharges)
	c.Send(p)
}
