package main

import (
	"log"
	"time"

	"asda2/shared/types"
)

const (
	spellEffectSchoolDamage        int16 = 2
	spellEffectApplyAura           int16 = 6
	spellEffectHeal                int16 = 10
	spellEffectWeaponDamage        int16 = 58
	spellEffectAttack              int16 = 78
	spellEffectRestoreHealthPct    int16 = 136
	spellEffectRestoreManaPct      int16 = 137
	spellEffectDamageFromPrcAttack int16 = 200
	spellEffectCastAnotherSpell    int16 = 201

	spellTargetFlagUnit uint32 = 2
)

type skillEffectKind byte

const (
	skillEffectNone skillEffectKind = iota
	skillEffectDamage
	skillEffectHeal
	skillEffectBuff
)

type runtimeSkillTarget struct {
	monster *Monster
	player  *Client
	self    bool
}

type runtimeSkillOptions struct {
	soulGuard bool
}

func useRuntimeSkill(c *Client, skillID int16, targetType byte, targetID uint16, opts runtimeSkillOptions) skillUseResult {
	if c == nil || c.Char == nil {
		return skillUseCannotApply
	}
	if c.Char.HP <= 0 {
		return skillUsePlayerIsDead
	}
	if c.Char.IsMoving {
		return skillUseMoving
	}

	requestedSkillID := skillID
	if opts.soulGuard {
		skillID = normalizeSoulGuardSkillID(skillID)
	}
	skill, ok := skillTemplates[skillID]
	if !ok {
		return skillUseItIsNotAnActiveSkill
	}
	if opts.soulGuard || skill.SoulGuardLevel > 0 {
		if result := soulGuardUseStatus(c, requestedSkillID, skill); result != skillUseOK {
			return result
		}
	} else if !skillAvailableForCharacter(c.Char, skill) {
		return skillUseWrongJob
	}
	if skill.IsPassive {
		return skillUseItIsNotAnActiveSkill
	}
	if !opts.soulGuard && !skillLearned(c, skill.ID) {
		return skillUseItIsNotAnActiveSkill
	}
	if remaining := skillCooldownRemaining(c, skill.ID); remaining > 0 {
		return skillUseCooldown
	}
	if c.Char.MP < int32(skill.PowerCost) {
		return skillUseLowMP
	}

	effectKind := skill.PrimaryEffectKind()
	target, result := resolveRuntimeSkillTarget(c, skill, effectKind, targetType, targetID)
	if result != skillUseOK {
		return result
	}
	if result := validateRuntimeSkillRange(c, skill, target); result != skillUseOK {
		return result
	}

	c.Char.MP -= int32(skill.PowerCost)
	if c.Char.MP < 0 {
		c.Char.MP = 0
	}
	sendCharacterMPUpdate(c)
	sendUseSkillResult(c, skill.ID, skillUseOK)
	sendSetSkillCooldown(c, skill)
	setSkillCooldown(c, skill)
	scheduleSkillReady(c, skill)

	result = applyRuntimeSkillEffect(c, skill, target, effectKind)
	if result != skillUseOK {
		return result
	}

	if opts.soulGuard || skill.SoulGuardLevel > 0 {
		consumeSoulGuardForSkill(c, skill)
	} else {
		addSoulGuardChargesForSkill(c, skill)
	}
	return skillUseOK
}

func (skill SkillTemplate) PrimaryEffectKind() skillEffectKind {
	switch {
	case skill.Effect0Type == spellEffectHeal || skill.Effect1Type == spellEffectHeal:
		return skillEffectHeal
	case skill.Effect0Type == spellEffectRestoreHealthPct || skill.Effect1Type == spellEffectRestoreHealthPct:
		return skillEffectHeal
	case skill.Effect0Type == spellEffectApplyAura || skill.Effect1Type == spellEffectApplyAura:
		return skillEffectBuff
	case skill.Effect0Type == spellEffectSchoolDamage ||
		skill.Effect0Type == spellEffectWeaponDamage ||
		skill.Effect0Type == spellEffectAttack ||
		skill.Effect0Type == spellEffectDamageFromPrcAttack ||
		skill.Effect1Type == spellEffectSchoolDamage ||
		skill.Effect1Type == spellEffectWeaponDamage ||
		skill.Effect1Type == spellEffectAttack ||
		skill.Effect1Type == spellEffectDamageFromPrcAttack:
		return skillEffectDamage
	case skill.Effect0Type == spellEffectCastAnotherSpell || skill.Effect1Type == spellEffectCastAnotherSpell:
		return skillEffectBuff
	case skill.Damage > 0:
		return skillEffectDamage
	default:
		return skillEffectNone
	}
}

func resolveRuntimeSkillTarget(c *Client, skill SkillTemplate, kind skillEffectKind, targetType byte, targetID uint16) (runtimeSkillTarget, skillUseResult) {
	if targetID == 0 && skillCanTargetSelf(skill, kind) {
		return runtimeSkillTarget{player: c, self: true}, skillUseOK
	}
	if targetID == 0 {
		return runtimeSkillTarget{}, skillUseNoTarget
	}

	switch targetType {
	case 0:
		target := currentMapMonsterByClientTarget(c, targetID)
		if target == nil || target.State != MonsterStateOK || target.Health <= 0 {
			return runtimeSkillTarget{}, skillUseInvalidTarget
		}
		if kind == skillEffectHeal || kind == skillEffectBuff {
			return runtimeSkillTarget{}, skillUseCannotApply
		}
		return runtimeSkillTarget{monster: target}, skillUseOK
	case 1:
		target := getClientBySessionID(int16(targetID))
		if target == nil || target.Char == nil || c.Char.MapID != target.Char.MapID || c.Channel != target.Channel {
			return runtimeSkillTarget{}, skillUseInvalidTarget
		}
		if kind == skillEffectDamage {
			return runtimeSkillTarget{}, skillUseCannotUseAgainstPlayer
		}
		return runtimeSkillTarget{player: target, self: target == c}, skillUseOK
	default:
		return runtimeSkillTarget{}, skillUseInvalidTarget
	}
}

func skillCanTargetSelf(skill SkillTemplate, kind skillEffectKind) bool {
	if skill.TargetFlags != 0 && skill.TargetFlags&spellTargetFlagUnit != 0 {
		return false
	}
	return kind == skillEffectBuff || kind == skillEffectHeal || kind == skillEffectNone
}

func validateRuntimeSkillRange(c *Client, skill SkillTemplate, target runtimeSkillTarget) skillUseResult {
	if skill.Range <= 0 {
		return skillUseOK
	}
	if target.monster != nil && monsterDistanceToClient(target.monster, c) > skill.Range {
		return skillUseDistanceTooFar
	}
	if target.player != nil && !target.self && skillTargetDistance(c, target.player) > skill.Range {
		return skillUseDistanceTooFar
	}
	return skillUseOK
}

func skillTargetDistance(a *Client, b *Client) float64 {
	if a == nil || a.Char == nil || b == nil || b.Char == nil {
		return 0
	}
	return distance2D(float64(asda2X(a.Char.X, a.Char.MapID)), float64(asda2Y(a.Char.Y, a.Char.MapID)),
		float64(asda2X(b.Char.X, b.Char.MapID)), float64(asda2Y(b.Char.Y, b.Char.MapID)))
}

func applyRuntimeSkillEffect(c *Client, skill SkillTemplate, target runtimeSkillTarget, kind skillEffectKind) skillUseResult {
	switch kind {
	case skillEffectDamage:
		if target.monster == nil {
			return skillUseInvalidTarget
		}
		sendSetAttackStateGUI(c)
		damage := skillDamageForCharacter(c.Char, skill)
		sendAnimateSkillStrike(c, skill, target.monster, damage)
		gm := World.GetMap(c.Char.MapID)
		if gm == nil {
			return skillUseInvalidTarget
		}
		c.Char.TargetID = target.monster.SessionID
		c.Char.IsFighting = true
		killed := gm.DamageMonsterFromSkill(c, target.monster, damage)
		log.Printf("[Skill] %q used skill=%d target=%d entry=%d damage=%d hp=%d killed=%t",
			c.Char.Name, skill.ID, target.monster.SessionID, target.monster.EntryID, damage, target.monster.Health, killed)
		if killed {
			c.Char.TargetID = -1
			c.Char.IsFighting = false
		}
	case skillEffectHeal:
		targetClient := target.player
		if targetClient == nil {
			targetClient = c
		}
		amount := skillHealAmount(targetClient.Char, skill)
		healCharacter(targetClient, amount, "skill")
		sendCharacterBuffed(targetClient, skill, 2*time.Second)
	case skillEffectBuff:
		targetClient := target.player
		if targetClient == nil {
			targetClient = c
		}
		applySkillBuff(targetClient, skill)
	default:
		if target.player != nil {
			sendCharacterBuffed(target.player, skill, 2*time.Second)
		}
	}
	return skillUseOK
}

func skillDamageForCharacter(chr *Character, skill SkillTemplate) int32 {
	damage := skill.Damage
	if damage <= 0 {
		damage = firstPositiveInt32(skill.Effect0BasePoints, skill.Effect1BasePoints, 20)
	}
	if chr != nil {
		if types.Asda2ClassFamily(chr.Class) == types.Asda2ProfessionMage {
			damage = int32(float64(damage) * (1 + float64(chr.SkillMagicDamageBonusPct)))
		} else {
			damage = int32(float64(damage) * (1 + float64(chr.SkillDamageBonusPct)))
		}
	}
	if damage < 1 {
		damage = 1
	}
	return damage
}

func skillHealAmount(chr *Character, skill SkillTemplate) int32 {
	if chr == nil {
		return firstPositiveInt32(skill.Effect0BasePoints, skill.Effect1BasePoints, 20)
	}
	if skill.Effect0Type == spellEffectRestoreHealthPct || skill.Effect1Type == spellEffectRestoreHealthPct {
		pct := firstPositiveInt32(skill.Effect0BasePoints, skill.Effect1BasePoints, 10)
		return chr.MaxHP * pct / 100
	}
	amount := firstPositiveInt32(skill.Effect0BasePoints, skill.Effect1BasePoints, skill.Damage, chr.MaxHP/5)
	if chr.SkillHealingDonePct != 0 {
		amount = int32(float64(amount) * (1 + float64(chr.SkillHealingDonePct)))
	}
	if amount < 1 {
		amount = 1
	}
	return amount
}

func firstPositiveInt32(values ...int32) int32 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func applySkillBuff(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	if _, exists := skillTemplates[skill.ID]; !exists {
		skillTemplates[skill.ID] = skill
	}
	duration := skill.Duration
	if duration <= 0 {
		duration = 30 * time.Second
	}
	if c.Char.SkillBuffExpires == nil {
		c.Char.SkillBuffExpires = make(map[int16]time.Time)
	}
	removeSkillBuff(c, skill.ID, false)
	applySkillBuffModifier(c.Char, skill, true)
	c.Char.SkillBuffExpires[skill.ID] = time.Now().Add(duration)
	rememberCharacterBuff(c.Char, skill.ID)
	sendCharacterBuffed(c, skill, duration)
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
	sessionID := c.Char.SessionID
	time.AfterFunc(duration, func() {
		current := getClientBySessionID(sessionID)
		if current == nil || current.Char == nil {
			return
		}
		expiresAt, ok := current.Char.SkillBuffExpires[skill.ID]
		if !ok || time.Now().Before(expiresAt) {
			return
		}
		removeSkillBuff(current, skill.ID, true)
	})
}

func removeSkillBuff(c *Client, skillID int16, sendEnded bool) bool {
	if c == nil || c.Char == nil {
		return false
	}
	if _, ok := c.Char.SkillBuffExpires[skillID]; !ok {
		return false
	}
	delete(c.Char.SkillBuffExpires, skillID)
	skill := skillTemplates[skillID]
	applySkillBuffModifier(c.Char, skill, false)
	for i, buffID := range c.Char.Buffs {
		if int16(buffID) == skillID {
			c.Char.Buffs[i] = 0
			break
		}
	}
	if sendEnded {
		sendBuffEndedForCharacter(c, skillID)
		sendUpdateStats(c)
		sendUpdateStatsOne(c)
	}
	return true
}

func rememberCharacterBuff(chr *Character, skillID int16) {
	if chr == nil {
		return
	}
	for i, buffID := range chr.Buffs {
		if buffID == int32(skillID) {
			return
		}
		if buffID == 0 {
			chr.Buffs[i] = int32(skillID)
			return
		}
	}
}

func applySkillBuffModifier(chr *Character, skill SkillTemplate, apply bool) {
	if chr == nil {
		return
	}
	sign := float32(1)
	if !apply {
		sign = -1
	}
	value := float32(firstPositiveInt32(skill.Effect0BasePoints, skill.Effect1BasePoints, 5)) / 100
	switch types.Asda2ClassFamily(chr.Class) {
	case types.Asda2ProfessionMage:
		chr.SkillMagicDamageBonusPct += value * sign
		chr.SkillHealingDonePct += value * sign
	case types.Asda2ProfessionArcher:
		chr.SkillDamageBonusPct += value * sign
		chr.SkillSpeedBonusPct += value * sign / 2
	default:
		chr.SkillDamageBonusPct += value * sign
		chr.SkillDefenseBonusPct += value * sign / 2
	}
}

func sendCharacterBuffed(c *Client, skill SkillTemplate, duration time.Duration) {
	if c == nil || c.Char == nil {
		return
	}
	seconds := int16((duration + time.Second - time.Nanosecond) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	iconID := skill.Effect0Misc
	if iconID == 0 {
		iconID = skill.ID
	}
	p := NewPacket(CharacterBuffed)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skill.ID)
	p.WriteInt16(iconID)
	p.WriteInt16(skill.ID)
	p.WriteInt16(1)
	p.WriteUint8(2)
	p.WriteInt16(seconds)
	p.WriteUint8(2)
	p.WriteBytes([]byte{0, 0, 198, 112, 211, 37, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0})
	c.SendToArea(p)
}

func sendBuffEndedForCharacter(c *Client, buffID int16) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(BuffEnded)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(buffID)
	c.SendToArea(p)
}
