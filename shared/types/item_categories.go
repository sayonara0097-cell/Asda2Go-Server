package types

type ItemUseEffect byte

const (
	ItemUseUnsupported ItemUseEffect = iota
	ItemUseRecoverHP
	ItemUseRecoverMP
	ItemUseContainer
	ItemUseBooster
	ItemUseExpandWarehouse
	ItemUseOpenWarehouse
	ItemUsePet
	ItemUseRecipe
	ItemUseFunctionalBuff
	ItemUseTeleport
	ItemUseChangeGender
	ItemUseReviveSelf
	ItemUseRepairEquipment
	ItemUseResetAllSkills
	ItemUseResetOneSkill
)

const (
	ItemCategoryOneHandedSword          = 1
	ItemCategoryTwoHandedSword          = 2
	ItemCategoryStaff                   = 3
	ItemCategoryCrossbow                = 4
	ItemCategoryBow                     = 5
	ItemCategoryBallista                = 6
	ItemCategorySpear                   = 7
	ItemCategoryShowel                  = 9
	ItemCategoryPetExp                  = 19
	ItemCategoryItemPackage             = 21
	ItemCategoryFish                    = 24
	ItemCategoryHealthPotion            = 29
	ItemCategoryManaPotion              = 30
	ItemCategoryRecipe                  = 32
	ItemCategoryEnchantStone            = 33
	ItemCategoryRingMaxAtack            = 44
	ItemCategoryRingMaxMAtack           = 45
	ItemCategoryRingMaxDef              = 46
	ItemCategoryNacklessCriticalChance  = 47
	ItemCategoryNacklessHealth          = 48
	ItemCategoryNacklessMana            = 49
	ItemCategoryHealthElixir            = 50
	ItemCategoryEnchantWeaponStoneD     = 51
	ItemCategoryRingMDef                = 52
	ItemCategoryNacklessMDef            = 53
	ItemCategoryEnchantWeaponStoneC     = 62
	ItemCategoryEnchantWeaponStoneB     = 65
	ItemCategoryEnchantWeaponStoneA     = 68
	ItemCategoryEnchantWeaponStoneS     = 69
	ItemCategoryEnchantArmorStoneD      = 70
	ItemCategoryEnchantArmorStoneC      = 71
	ItemCategoryEnchantArmorStoneB      = 72
	ItemCategoryEnchantArmorStoneA      = 73
	ItemCategoryEnchantArmorStoneS      = 74
	ItemCategoryEnchantArmorStoneE      = 79
	ItemCategoryEnchantUniversalStoneE  = 80
	ItemCategoryEnchantUniversalStoneD  = 81
	ItemCategoryEnchantUniversalStoneC  = 82
	ItemCategoryEnchantUniversalStoneB  = 83
	ItemCategoryBowAmmo                 = 61
	ItemCategoryCrossbowAmmo            = 63
	ItemCategoryManaElixir              = 66
	ItemCategoryEnchantUniversalStoneA  = 85
	ItemCategoryEnchantUniversalStoneS  = 87
	ItemCategoryEnchant100Stone         = 88
	ItemCategoryResurectScroll          = 84
	ItemCategoryReturnScroll            = 86
	ItemCategoryBooster                 = 94
	ItemCategoryIncPAtk                 = 108
	ItemCategoryIncMAtk                 = 109
	ItemCategoryIncPDef                 = 110
	ItemCategoryIncMdef                 = 111
	ItemCategoryIncHp                   = 112
	ItemCategoryIncMp                   = 113
	ItemCategoryIncStr                  = 114
	ItemCategoryIncSta                  = 115
	ItemCategoryIncInt                  = 116
	ItemCategoryIncSpi                  = 117
	ItemCategoryIncDex                  = 118
	ItemCategoryIncLuck                 = 119
	ItemCategoryReduceMpConsumption     = 120
	ItemCategoryIncMoveSpeed            = 121
	ItemCategoryIncExp                  = 122
	ItemCategoryIncDropChance           = 123
	ItemCategoryIncDigChance            = 124
	ItemCategoryIncExpStackable         = 125
	ItemCategoryPetNotEating            = 126
	ItemCategoryDoublePetExperience     = 127
	ItemCategoryIncAttackSpeed          = 128
	ItemCategoryRemoveDeathPenalties    = 129
	ItemCategoryExpandWarehouse         = 130
	ItemCategoryResetAllSkill           = 131
	ItemCategoryResetOneSkill           = 132
	ItemCategoryTeleportToCharacter     = 133
	ItemCategoryRepairEquipment         = 134
	ItemCategoryInstantRecoverHP        = 135
	ItemCategoryInstantRecoverHPMP      = 136
	ItemCategoryRecoverHPOverTime       = 140
	ItemCategoryShopBanner              = 145
	ItemCategoryChangeGender            = 146
	ItemCategoryReviveWithoutExpLoss    = 137
	ItemCategoryGlobalChat              = 138
	ItemCategoryIncreaseWeightCapacity  = 139
	ItemCategoryClassReset              = 141
	ItemCategoryGuildCrest              = 142
	ItemCategoryGuildCrestVehicleFlags  = 143
	ItemCategorySummonCharacterToYou    = 144
	ItemCategoryItemOptionExchange      = 147
	ItemCategorySealItem                = 148
	ItemCategoryIncreaseOptionTransfer  = 149
	ItemCategoryIncreaseUpgradeChance   = 150
	ItemCategorySuperiorUpgradeBoost    = 151
	ItemCategoryExpandPetBox            = 152
	ItemCategoryRecoverPetFood          = 153
	ItemCategoryUltimateIncubator       = 154
	ItemCategoryIncreasePetHatching     = 155
	ItemCategoryPremiumPetEgg           = 156
	ItemCategoryChangeCharacterName     = 157
	ItemCategoryOpenWarehouse           = 158
	ItemCategoryResetFaction            = 159
	ItemCategoryPremiumPotions          = 160
	ItemCategoryKeyForTreasureBox       = 161
	ItemCategoryStyleShopCoupon         = 162
	ItemCategoryPetSynthesisSupplement  = 163
	ItemCategoryPetLevelBreakCharm      = 164
	ItemCategoryPetLevelProtection      = 165
	ItemCategorySowelProtectionScroll   = 166
	ItemCategoryUpgradeProtectScroll    = 167
	ItemCategoryExpandInventory         = 168
	ItemCategoryPetNotEatingByDays      = 169
	ItemCategoryOpenWarehouseAndRune    = 170
	ItemCategoryAvatarSynthesisBoost    = 171
	ItemCategoryRemoveDeathPenaltyByDay = 172
)

func (t ItemTemplate) UseEffect() ItemUseEffect {
	switch t.Category {
	case ItemCategoryHealthPotion, ItemCategoryHealthElixir, ItemCategoryInstantRecoverHP, ItemCategoryRecoverHPOverTime:
		return ItemUseRecoverHP
	case ItemCategoryManaPotion, ItemCategoryManaElixir, ItemCategoryFish:
		return ItemUseRecoverMP
	case ItemCategoryInstantRecoverHPMP:
		return ItemUseRecoverHP
	case ItemCategoryItemPackage:
		return ItemUseContainer
	case ItemCategoryBooster:
		return ItemUseBooster
	case ItemCategoryExpandWarehouse:
		return ItemUseExpandWarehouse
	case ItemCategoryOpenWarehouse, ItemCategoryOpenWarehouseAndRune:
		return ItemUseOpenWarehouse
	case ItemCategoryPetExp:
		return ItemUsePet
	case ItemCategoryRecipe:
		return ItemUseRecipe
	case ItemCategoryResurectScroll, ItemCategoryReturnScroll, ItemCategoryTeleportToCharacter:
		return ItemUseTeleport
	case ItemCategoryChangeGender:
		return ItemUseChangeGender
	case ItemCategoryReviveWithoutExpLoss:
		return ItemUseReviveSelf
	case ItemCategoryRepairEquipment:
		return ItemUseRepairEquipment
	case ItemCategoryResetAllSkill:
		return ItemUseResetAllSkills
	case ItemCategoryResetOneSkill:
		return ItemUseResetOneSkill
	case ItemCategoryIncPAtk, ItemCategoryIncMAtk, ItemCategoryIncPDef, ItemCategoryIncMdef,
		ItemCategoryIncHp, ItemCategoryIncMp, ItemCategoryIncStr, ItemCategoryIncSta,
		ItemCategoryIncInt, ItemCategoryIncSpi, ItemCategoryIncDex, ItemCategoryIncLuck,
		ItemCategoryIncMoveSpeed, ItemCategoryIncExp, ItemCategoryIncDropChance,
		ItemCategoryIncDigChance, ItemCategoryIncExpStackable, ItemCategoryIncAttackSpeed,
		ItemCategoryShopBanner,
		ItemCategoryPremiumPotions,
		ItemCategoryExpandInventory, ItemCategoryPetNotEatingByDays, ItemCategoryRemoveDeathPenaltyByDay:
		return ItemUseFunctionalBuff
	default:
		if t.Kind == ItemKindConsumable && t.ValueOnUse > 0 {
			return ItemUseRecoverHP
		}
		return ItemUseUnsupported
	}
}
