package stats

import (
	"math"

	"asda2/shared/types"
)

const (
	charFormulaMaxToTotalMultiplier        = 0.7
	charFormulaDamagePerIntellect          = 0.28
	charFormulaDamagePerAgility            = 0.17
	charFormulaDamagePerStrength           = 0.15
	charFormulaMagicDefencePerSpirit       = 1.0
	charFormulaHealthPerStrength           = 0.6
	charFormulaHealthPerStamina            = 14.5
	charFormulaManaPerSpirit               = 2.54
	charFormulaItemsDefenceMultiplier      = 2.5
	charFormulaItemsMagicDefenceMultiplier = 4.5
)

const (
	equipmentSlotWeapon int16 = 9

	defaultWeaponMinDamage int32 = 1
	defaultWeaponMaxDamage int32 = 3
)

const (
	charItemBonusMaxAttack              int16 = 1
	charItemBonusMaxMagicAttack         int16 = 2
	charItemBonusMaxDefence             int16 = 3
	charItemBonusMaxHP                  int16 = 5
	charItemBonusMaxMP                  int16 = 6
	charItemBonusMinAttack              int16 = 64
	charItemBonusMinMagicAttack         int16 = 65
	charItemBonusMinDefence             int16 = 66
	charItemBonusAttack                 int16 = 67
	charItemBonusMagicAttack            int16 = 68
	charItemBonusDefence                int16 = 69
	charItemBonusMinBlockRatePercent    int16 = 71
	charItemBonusMaxBlockRatePercent    int16 = 72
	charItemBonusBlockRatePercent       int16 = 73
	charItemBonusBlockedDamageReduction int16 = 74
	charItemBonusPvPDefence             int16 = 94
	charItemBonusPvPPenetration         int16 = 95
	charItemBonusMagicDefence           int16 = 115
)

const (
	sowelBonusNone        = 0
	sowelBonusWeaponAtk   = 1
	sowelBonusWeaponMAtk  = 2
	sowelBonusDefence     = 3
	sowelBonusStrength    = 4
	sowelBonusAgility     = 5
	sowelBonusStamina     = 6
	sowelBonusSpirit      = 7
	sowelBonusIntellect   = 8
	sowelBonusLuck        = 9
	sowelBonusStrengthPct = 10
	sowelBonusStaminaPct  = 11
	sowelBonusIntPct      = 12
	sowelBonusSpiritPct   = 13
	sowelBonusLuckPct     = 14
)

const (
	ItemBonusMaxAttack              = charItemBonusMaxAttack
	ItemBonusMaxMagicAttack         = charItemBonusMaxMagicAttack
	ItemBonusMaxDefence             = charItemBonusMaxDefence
	ItemBonusMaxHP                  = charItemBonusMaxHP
	ItemBonusMaxMP                  = charItemBonusMaxMP
	ItemBonusMinAttack              = charItemBonusMinAttack
	ItemBonusMinMagicAttack         = charItemBonusMinMagicAttack
	ItemBonusMinDefence             = charItemBonusMinDefence
	ItemBonusAttack                 = charItemBonusAttack
	ItemBonusMagicAttack            = charItemBonusMagicAttack
	ItemBonusDefence                = charItemBonusDefence
	ItemBonusMinBlockRatePercent    = charItemBonusMinBlockRatePercent
	ItemBonusMaxBlockRatePercent    = charItemBonusMaxBlockRatePercent
	ItemBonusBlockRatePercent       = charItemBonusBlockRatePercent
	ItemBonusBlockedDamageReduction = charItemBonusBlockedDamageReduction
	ItemBonusPvPDefence             = charItemBonusPvPDefence
	ItemBonusPvPPenetration         = charItemBonusPvPPenetration
	ItemBonusMagicDefence           = charItemBonusMagicDefence

	SowelBonusNone        = sowelBonusNone
	SowelBonusWeaponAtk   = sowelBonusWeaponAtk
	SowelBonusWeaponMAtk  = sowelBonusWeaponMAtk
	SowelBonusDefence     = sowelBonusDefence
	SowelBonusStrength    = sowelBonusStrength
	SowelBonusAgility     = sowelBonusAgility
	SowelBonusStamina     = sowelBonusStamina
	SowelBonusSpirit      = sowelBonusSpirit
	SowelBonusIntellect   = sowelBonusIntellect
	SowelBonusLuck        = sowelBonusLuck
	SowelBonusStrengthPct = sowelBonusStrengthPct
	SowelBonusStaminaPct  = sowelBonusStaminaPct
	SowelBonusIntPct      = sowelBonusIntPct
	SowelBonusSpiritPct   = sowelBonusSpiritPct
	SowelBonusLuckPct     = sowelBonusLuckPct
)

type AttributeSet struct {
	Strength  int32
	Agility   int32
	Stamina   int32
	Spirit    int32
	Intellect int32
	Luck      int32
}

type Modifiers struct {
	Attributes AttributeSet

	StrengthPct  float64
	AgilityPct   float64
	StaminaPct   float64
	SpiritPct    float64
	IntellectPct float64
	LuckPct      float64

	Health       int32
	Power        int32
	Damage       int32
	MagicDamage  int32
	Defence      int32
	MagicDefence int32
	BlockChance  int32
	BlockValue   int32

	DamagePct       float64
	MagicDamagePct  float64
	DefencePct      float64
	MagicDefencePct float64

	MinDefenceParam int32
	MaxDefenceParam int32
}

type Snapshot struct {
	Base  AttributeSet
	Bonus AttributeSet
	Total AttributeSet
	Mods  Modifiers

	MaxHP int32
	MaxMP int32

	MinDamage      int32
	MaxDamage      int32
	MinMagicDamage int32
	MaxMagicDamage int32
	MagicDefence   int32
	DefenceMin     int32
	DefenceMax     int32
	BlockChance    int32
	BlockValue     int32
	AttackMagical  bool
}

func CalculateCharacterStats(chr *types.Character) Snapshot {
	base := characterBaseAttributes(chr)
	mods := collectModifiers(chr)
	total := AttributeSet{
		Strength:  applyIntMulti(base.Strength+mods.Attributes.Strength, mods.StrengthPct),
		Agility:   applyIntMulti(base.Agility+mods.Attributes.Agility, mods.AgilityPct),
		Stamina:   applyIntMulti(base.Stamina+mods.Attributes.Stamina, mods.StaminaPct),
		Spirit:    applyIntMulti(base.Spirit+mods.Attributes.Spirit, mods.SpiritPct),
		Intellect: applyIntMulti(base.Intellect+mods.Attributes.Intellect, mods.IntellectPct),
		Luck:      applyIntMulti(base.Luck+mods.Attributes.Luck, mods.LuckPct),
	}
	stats := Snapshot{
		Base:  base,
		Bonus: total.sub(base),
		Total: total,
		Mods:  mods,
	}
	stats.MaxHP = calculateMaxHealth(chr, total, mods)
	stats.MaxMP = calculateMaxPower(chr, total, mods)
	stats.fillCombatFields(chr)
	return stats
}

func characterBaseAttributes(chr *types.Character) AttributeSet {
	if chr == nil {
		return AttributeSet{}
	}
	return AttributeSet{
		Strength:  int32(chr.BaseStrength),
		Agility:   int32(chr.BaseAgility),
		Stamina:   int32(chr.BaseStamina),
		Spirit:    int32(chr.BaseSpirit),
		Intellect: int32(chr.BaseIntellect),
		Luck:      int32(chr.BaseLuck),
	}
}

func (a AttributeSet) sub(b AttributeSet) AttributeSet {
	return AttributeSet{
		Strength:  a.Strength - b.Strength,
		Agility:   a.Agility - b.Agility,
		Stamina:   a.Stamina - b.Stamina,
		Spirit:    a.Spirit - b.Spirit,
		Intellect: a.Intellect - b.Intellect,
		Luck:      a.Luck - b.Luck,
	}
}

func equippedWeapon(chr *types.Character) *types.ItemRow {
	if chr == nil {
		return nil
	}
	return types.ItemInInventorySlot(chr.Items, types.InventoryEquipment, equipmentSlotWeapon)
}

func itemCategory(item *types.ItemRow) int {
	if item == nil {
		return 0
	}
	return types.ItemTemplateByID(item.ItemID).Category
}

func (s *Snapshot) fillCombatFields(chr *types.Character) {
	weapon := equippedWeapon(chr)
	category := itemCategory(weapon)
	magical := category == types.ItemCategoryStaff
	minWeapon, maxWeapon := weaponDamageRange(chr, weapon, category)

	if magical {
		bonus := calculateMagicDamageBonusForClass(chr, s.Total)
		pct := s.Mods.MagicDamagePct
		if chr != nil {
			pct += float64(chr.SkillMagicDamageBonusPct)
		}
		s.MinMagicDamage = clampDamageInt(applyFloatMulti(minWeapon+float64(s.Mods.MagicDamage)+bonus, pct))
		s.MaxMagicDamage = clampDamageInt(applyFloatMulti(maxWeapon+float64(s.Mods.MagicDamage)+bonus, pct))
		if s.MaxMagicDamage < s.MinMagicDamage {
			s.MaxMagicDamage = s.MinMagicDamage
		}
	} else {
		bonus := calculatePhysicalDamageBonusForClass(chr, s.Total)
		pct := s.Mods.DamagePct
		if chr != nil {
			pct += float64(chr.SkillDamageBonusPct)
		}
		s.MinDamage = clampDamageInt(applyFloatMulti(minWeapon+float64(s.Mods.Damage)+bonus, pct))
		s.MaxDamage = clampDamageInt(applyFloatMulti(maxWeapon+float64(s.Mods.Damage)+bonus, pct))
		if s.MaxDamage < s.MinDamage {
			s.MaxDamage = s.MinDamage
		}
	}

	defence := applyFloatMulti(float64(s.Mods.Defence)+calculateDefenceBonusForClass(chr, s.Total), s.Mods.DefencePct)
	if chr != nil {
		defence = applyFloatMulti(defence, float64(chr.SkillDefenseBonusPct))
	}
	defenceValue := int32(defence)
	s.DefenceMin = defenceValue - s.Mods.MinDefenceParam
	s.DefenceMax = defenceValue - s.Mods.MinDefenceParam + s.Mods.MaxDefenceParam
	if s.DefenceMin < 0 {
		s.DefenceMin = 0
	}
	if s.DefenceMax < s.DefenceMin {
		s.DefenceMax = s.DefenceMin
	}

	magicDefence := applyFloatMulti(float64(s.Mods.MagicDefence)+calculateMagicDefenceBonusForClass(chr, s.Total), s.Mods.MagicDefencePct)
	if chr != nil {
		magicDefence = applyFloatMulti(magicDefence, float64(chr.SkillMagicDefenseBonusPct))
	}
	if magicDefence < 0 {
		magicDefence = 0
	}
	s.MagicDefence = int32(magicDefence)
	s.BlockChance = s.Mods.BlockChance
	s.BlockValue = s.Mods.BlockValue
	s.AttackMagical = magical
}

func NormalAttackDamageRange(chr *types.Character) (int32, int32, bool) {
	stats := CalculateCharacterStats(chr)
	if stats.AttackMagical {
		return stats.MinMagicDamage, stats.MaxMagicDamage, true
	}
	return stats.MinDamage, stats.MaxDamage, false
}

func NormalAttackDisplayStats(chr *types.Character) (int32, int32, int32, int32) {
	stats := CalculateCharacterStats(chr)
	return stats.MinDamage, stats.MaxDamage, stats.MinMagicDamage, stats.MaxMagicDamage
}

func weaponDamageRange(chr *types.Character, weapon *types.ItemRow, category int) (float64, float64) {
	if weapon == nil {
		return float64(defaultWeaponMinDamage), float64(defaultWeaponMaxDamage)
	}
	soulTemplate := types.ItemTemplateByID(weapon.Soul1ID)
	if soulTemplate.SowelBonusValue <= 0 || !weaponSowelDamageApplies(category, soulTemplate) {
		return float64(defaultWeaponMinDamage), float64(defaultWeaponMaxDamage)
	}
	minDamage := float64(soulTemplate.SowelBonusValue) *
		calculateEnchantMultiplier(weapon.Enchant) *
		weaponTypeDamageMultiplier(category, chr)
	maxDamage := minDamage * 1.10000002384186
	if minDamage < 1 || maxDamage < minDamage {
		return float64(defaultWeaponMinDamage), float64(defaultWeaponMaxDamage)
	}
	return minDamage, maxDamage
}

func weaponSowelDamageApplies(category int, soulTemplate types.ItemTemplate) bool {
	if soulTemplate.SowelBonusType == sowelBonusNone {
		return true
	}
	if category == types.ItemCategoryStaff {
		return soulTemplate.SowelBonusType == sowelBonusWeaponMAtk
	}
	return soulTemplate.SowelBonusType == sowelBonusWeaponAtk
}

func calculatePhysicalDamageBonus(chr *types.Character) float64 {
	return calculatePhysicalDamageBonusForClass(chr, characterBaseAttributes(chr))
}

func calculatePhysicalDamageBonusForClass(chr *types.Character, attrs AttributeSet) float64 {
	if chr == nil {
		return 0
	}
	var base float64
	switch types.Asda2ClassFamily(chr.Class) {
	case types.Asda2ProfessionWarrior:
		base = float64(attrs.Strength) * charFormulaDamagePerStrength
	case types.Asda2ProfessionArcher:
		base = float64(attrs.Agility) * charFormulaDamagePerAgility
	default:
		base = float64(attrs.Agility)*charFormulaDamagePerAgility +
			float64(attrs.Strength)*charFormulaDamagePerStrength
	}
	return base * physicalClassDamageMultiplier(chr.Class)
}

func calculateMagicDamageBonus(chr *types.Character) float64 {
	return calculateMagicDamageBonusForClass(chr, characterBaseAttributes(chr))
}

func calculateMagicDamageBonusForClass(chr *types.Character, attrs AttributeSet) float64 {
	if chr == nil {
		return 0
	}
	base := float64(attrs.Intellect) * charFormulaDamagePerIntellect
	switch types.Asda2Class(chr.Class) {
	case types.Asda2ClassAttackMage:
		return base
	case types.Asda2ClassSupportMage:
		return base * 0.699999988079071
	case types.Asda2ClassHealMage:
		return base * 0.800000011920929
	default:
		return 0
	}
}

func calculateDefenceBonusForClass(_ *types.Character, _ AttributeSet) float64 {
	return 0
}

func calculateMagicDefenceBonusForClass(chr *types.Character, attrs AttributeSet) float64 {
	if chr == nil {
		return 0
	}
	base := float64(attrs.Spirit) * charFormulaMagicDefencePerSpirit
	switch types.Asda2Class(chr.Class) {
	case types.Asda2ClassOHS:
		return base * 2.20000004768372
	case types.Asda2ClassSupportMage, types.Asda2ClassHealMage:
		return base * 1.29999995231628
	default:
		return base
	}
}

func calculateMaxHealth(chr *types.Character, attrs AttributeSet, mods Modifiers) int32 {
	if chr == nil {
		return 0
	}
	base := float64(chr.MaxHP + mods.Health)
	value := int32(applyFloatMulti(base+calculateHealthBonusForClass(chr, attrs), 0))
	if value < 0 {
		return 0
	}
	return value
}

func calculateHealthBonusForClass(chr *types.Character, attrs AttributeSet) float64 {
	if chr == nil {
		return 0
	}
	stamina := float64(attrs.Stamina) * charFormulaHealthPerStamina
	strength := float64(attrs.Strength) * charFormulaHealthPerStrength
	switch types.Asda2Class(chr.Class) {
	case types.Asda2ClassOHS:
		return stamina*1.60000002384186 + strength
	case types.Asda2ClassSpear:
		return stamina*2.40000009536743 + strength
	case types.Asda2ClassTHS:
		return stamina*2.09999990463257 + strength*1.10000002384186
	case types.Asda2ClassCrossbow:
		return stamina*1.70000004768372 + strength*0.800000011920929
	case types.Asda2ClassBow, types.Asda2ClassBalista:
		return stamina*1.89999997615814 + strength*0.899999976158142
	case types.Asda2ClassAttackMage:
		return stamina * 1.25
	case types.Asda2ClassSupportMage, types.Asda2ClassHealMage:
		return stamina * 1.45000004768372
	default:
		return stamina + strength
	}
}

func calculateMaxPower(chr *types.Character, attrs AttributeSet, mods Modifiers) int32 {
	if chr == nil {
		return 0
	}
	value := int32(chr.MaxMP+mods.Power) + calculateManaBonusForClass(chr, attrs)
	if value < 0 {
		return 0
	}
	return value
}

func calculateManaBonusForClass(chr *types.Character, attrs AttributeSet) int32 {
	if chr == nil {
		return 0
	}
	multiplier := 1.0
	switch types.Asda2Class(chr.Class) {
	case types.Asda2ClassAttackMage, types.Asda2ClassSupportMage, types.Asda2ClassHealMage:
		multiplier = 2
	}
	return int32(float64(attrs.Spirit) * charFormulaManaPerSpirit * multiplier)
}

func collectModifiers(chr *types.Character) Modifiers {
	if chr == nil {
		return Modifiers{}
	}
	var mods Modifiers
	for _, item := range types.ItemsInInventory(chr.Items, types.InventoryEquipment) {
		applyEquipmentTemplateStatModifier(&mods, item)
		applyEquipmentSoulStatModifiers(&mods, item)
		applyEquipmentParamStatModifiers(&mods, item)
	}
	return mods
}

func applyEquipmentTemplateStatModifier(mods *Modifiers, item *types.ItemRow) {
	if mods == nil || item == nil {
		return
	}
	templ := types.ItemTemplateByID(item.ItemID)
	switch templ.Category {
	case types.ItemCategoryRingMDef, types.ItemCategoryNacklessMDef:
		mods.MagicDefence += int32(float64(templ.ValueOnUse) * charFormulaItemsMagicDefenceMultiplier)
	case types.ItemCategoryRingMaxDef:
		mods.Defence += int32(templ.ValueOnUse)
	case types.ItemCategoryRingMaxMAtack:
		mods.MagicDamage += int32(templ.ValueOnUse)
	case types.ItemCategoryRingMaxAtack:
		mods.Damage += int32(templ.ValueOnUse)
	case types.ItemCategoryNacklessHealth:
		mods.Health += int32(templ.ValueOnUse)
	case types.ItemCategoryNacklessMana:
		mods.Power += int32(templ.ValueOnUse)
	}
}

func applyEquipmentSoulStatModifiers(mods *Modifiers, item *types.ItemRow) {
	if mods == nil || item == nil {
		return
	}
	itemTemplate := types.ItemTemplateByID(item.ItemID)
	for _, soulID := range []int{item.Soul1ID, item.Soul2ID, item.Soul3ID, item.Soul4ID} {
		if soulID <= 0 {
			continue
		}
		soulTemplate := types.ItemTemplateByID(soulID)
		value := int32(float64(soulTemplate.SowelBonusValue) * calculateEnchantMultiplier(item.Enchant))
		if value == 0 {
			continue
		}
		switch soulTemplate.SowelBonusType {
		case sowelBonusDefence:
			mods.Defence += getSowelDefence(value, itemTemplate.RequiredProfession)
		case sowelBonusStrength:
			mods.Attributes.Strength += value
		case sowelBonusAgility:
			mods.Attributes.Agility += int32(float64(value) * (77.0 / 64.0))
		case sowelBonusStamina:
			mods.Attributes.Stamina += int32(float64(value) * 1.5)
		case sowelBonusSpirit:
			mods.Attributes.Spirit += int32(float64(value) * 1.5)
		case sowelBonusIntellect:
			mods.Attributes.Intellect += value
		case sowelBonusLuck:
			mods.Attributes.Luck += int32(float64(value) * 2.625)
		case sowelBonusStrengthPct:
			mods.StrengthPct += float64(value) / 100
		case sowelBonusStaminaPct:
			mods.StaminaPct += float64(value) / 100
		case sowelBonusIntPct:
			mods.IntellectPct += float64(value) / 100
		case sowelBonusSpiritPct:
			mods.SpiritPct += float64(value) / 100
		case sowelBonusLuckPct:
			mods.LuckPct += float64(value) / 100
		}
	}
}

func applyEquipmentParamStatModifiers(mods *Modifiers, item *types.ItemRow) {
	if mods == nil || item == nil {
		return
	}
	for _, param := range itemParamEntries(item) {
		value := int32(float64(param.value) * calculateEnchantMultiplier(item.Enchant))
		weighted := int32(float64(value) * charFormulaMaxToTotalMultiplier)
		switch param.typ {
		case charItemBonusMaxAttack, charItemBonusMinAttack:
			mods.Damage += weighted
		case charItemBonusMaxMagicAttack, charItemBonusMinMagicAttack:
			mods.MagicDamage += weighted
		case charItemBonusMaxDefence, charItemBonusMinDefence:
			mods.Defence += weighted
		case charItemBonusAttack, charItemBonusPvPPenetration:
			mods.Damage += value
		case charItemBonusMagicAttack:
			mods.MagicDamage += value
		case charItemBonusDefence, charItemBonusPvPDefence:
			mods.Defence += value
		case charItemBonusMagicDefence:
			mods.MagicDefence += value
		case charItemBonusMaxHP:
			mods.Health += value
		case charItemBonusMaxMP:
			mods.Power += value
		case charItemBonusMinBlockRatePercent, charItemBonusMaxBlockRatePercent:
			mods.BlockChance += weighted
		case charItemBonusBlockRatePercent:
			mods.BlockChance += value
		case charItemBonusBlockedDamageReduction:
			mods.BlockValue += value
		}
		if param.slot <= 4 {
			switch param.typ {
			case charItemBonusMinDefence:
				mods.MinDefenceParam += int32(float64(param.value) * charFormulaMaxToTotalMultiplier)
			case charItemBonusMaxDefence:
				mods.MaxDefenceParam += int32(float64(param.value) * charFormulaMaxToTotalMultiplier)
			}
		}
	}
}

type itemParamEntry struct {
	slot  int
	typ   int16
	value int32
}

func itemParamEntries(item *types.ItemRow) []itemParamEntry {
	if item == nil {
		return nil
	}
	return []itemParamEntry{
		{slot: 1, typ: item.Param1Type, value: int32(item.Param1Value)},
		{slot: 2, typ: item.Param2Type, value: int32(item.Param2Value)},
		{slot: 3, typ: item.Param3Type, value: int32(item.Param3Value)},
		{slot: 4, typ: item.Param4Type, value: int32(item.Param4Value)},
		{slot: 5, typ: item.Param5Type, value: int32(item.Param5Value)},
	}
}

func equipmentFlatDamageBonus(chr *types.Character, magical bool) int32 {
	mods := collectModifiers(chr)
	if magical {
		return mods.MagicDamage
	}
	return mods.Damage
}

func getSowelDefence(value int32, requiredProfession byte) int32 {
	switch requiredProfession {
	case 2:
		return int32(float64(value) * 4.5 * charFormulaItemsDefenceMultiplier)
	case 3:
		return int32(float64(value) * 3.90000009536743 * charFormulaItemsDefenceMultiplier)
	default:
		return int32(float64(value) * 5 * charFormulaItemsDefenceMultiplier)
	}
}

func physicalClassDamageMultiplier(classID byte) float64 {
	switch types.Asda2Class(classID) {
	case types.Asda2ClassOHS:
		return 0.899999976158142
	case types.Asda2ClassSpear:
		return 1.10000002384186
	case types.Asda2ClassTHS:
		return 1.29999995231628
	case types.Asda2ClassCrossbow, types.Asda2ClassBow:
		return 1.04999995231628
	default:
		return 1
	}
}

func weaponTypeDamageMultiplier(category int, chr *types.Character) float64 {
	switch category {
	case types.ItemCategoryOneHandedSword:
		return 16.5
	case types.ItemCategoryTwoHandedSword:
		return 14.5
	case types.ItemCategoryStaff:
		switch {
		case chr != nil && chr.Class == byte(types.Asda2ClassAttackMage):
			return 15.2
		case chr != nil && (chr.Class == byte(types.Asda2ClassSupportMage) || chr.Class == byte(types.Asda2ClassHealMage)):
			return 13.45
		default:
			return 0
		}
	case types.ItemCategoryCrossbow:
		return 17.1
	case types.ItemCategoryBow:
		return 17.5
	case types.ItemCategorySpear:
		return 16.2
	default:
		return 1
	}
}

func calculateEnchantMultiplier(enchant byte) float64 {
	if enchant == 0 {
		return 1
	}
	value := math.Pow(float64(enchant), 0.3)
	switch enchant {
	case 16:
		value *= 1.2
	case 17:
		value *= 1.3
	case 18:
		value *= 1.4
	case 19:
		value *= 1.5
	case 20:
		value *= 2.5
	}
	return value
}

func applyIntMulti(value int32, pct float64) int32 {
	return int32(float64(value)*(1+pct) + 0.5)
}

func applyFloatMulti(value float64, pct float64) float64 {
	return value * (1 + pct)
}

func clampDamageInt(value float64) int32 {
	damage := int32(value)
	if damage < defaultWeaponMinDamage {
		return defaultWeaponMinDamage
	}
	return damage
}
