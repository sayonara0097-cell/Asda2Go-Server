package main

import charstats "asda2/shared/stats"

const (
	charItemBonusMaxAttack              = charstats.ItemBonusMaxAttack
	charItemBonusMaxMagicAttack         = charstats.ItemBonusMaxMagicAttack
	charItemBonusMaxDefence             = charstats.ItemBonusMaxDefence
	charItemBonusMaxHP                  = charstats.ItemBonusMaxHP
	charItemBonusMaxMP                  = charstats.ItemBonusMaxMP
	charItemBonusMinAttack              = charstats.ItemBonusMinAttack
	charItemBonusMinMagicAttack         = charstats.ItemBonusMinMagicAttack
	charItemBonusMinDefence             = charstats.ItemBonusMinDefence
	charItemBonusAttack                 = charstats.ItemBonusAttack
	charItemBonusMagicAttack            = charstats.ItemBonusMagicAttack
	charItemBonusDefence                = charstats.ItemBonusDefence
	charItemBonusMinBlockRatePercent    = charstats.ItemBonusMinBlockRatePercent
	charItemBonusMaxBlockRatePercent    = charstats.ItemBonusMaxBlockRatePercent
	charItemBonusBlockRatePercent       = charstats.ItemBonusBlockRatePercent
	charItemBonusBlockedDamageReduction = charstats.ItemBonusBlockedDamageReduction
	charItemBonusPvPDefence             = charstats.ItemBonusPvPDefence
	charItemBonusPvPPenetration         = charstats.ItemBonusPvPPenetration
	charItemBonusMagicDefence           = charstats.ItemBonusMagicDefence

	sowelBonusNone        = charstats.SowelBonusNone
	sowelBonusWeaponAtk   = charstats.SowelBonusWeaponAtk
	sowelBonusWeaponMAtk  = charstats.SowelBonusWeaponMAtk
	sowelBonusDefence     = charstats.SowelBonusDefence
	sowelBonusStrength    = charstats.SowelBonusStrength
	sowelBonusAgility     = charstats.SowelBonusAgility
	sowelBonusStamina     = charstats.SowelBonusStamina
	sowelBonusSpirit      = charstats.SowelBonusSpirit
	sowelBonusIntellect   = charstats.SowelBonusIntellect
	sowelBonusLuck        = charstats.SowelBonusLuck
	sowelBonusStrengthPct = charstats.SowelBonusStrengthPct
	sowelBonusStaminaPct  = charstats.SowelBonusStaminaPct
	sowelBonusIntPct      = charstats.SowelBonusIntPct
	sowelBonusSpiritPct   = charstats.SowelBonusSpiritPct
	sowelBonusLuckPct     = charstats.SowelBonusLuckPct
)

type characterAttributeSet = charstats.AttributeSet
type characterStatsSnapshot = charstats.Snapshot

func calculateCharacterStats(chr *Character) characterStatsSnapshot {
	return charstats.CalculateCharacterStats(chr)
}

func normalAttackDamageRange(chr *Character) (int32, int32, bool) {
	return charstats.NormalAttackDamageRange(chr)
}

func normalAttackDisplayStats(chr *Character) (int32, int32, int32, int32) {
	return charstats.NormalAttackDisplayStats(chr)
}

func writeCharacterAttributes(p *PacketOut, attrs characterAttributeSet) {
	p.WriteInt16(clampInt16(attrs.Strength))
	p.WriteInt16(clampInt16(attrs.Agility))
	p.WriteInt16(clampInt16(attrs.Stamina))
	p.WriteInt16(clampInt16(attrs.Spirit))
	p.WriteInt16(clampInt16(attrs.Intellect))
	p.WriteInt16(clampInt16(attrs.Luck))
}

func writeZeroCharacterAttributes(p *PacketOut) {
	for i := 0; i < 6; i++ {
		p.WriteInt16(0)
	}
}
