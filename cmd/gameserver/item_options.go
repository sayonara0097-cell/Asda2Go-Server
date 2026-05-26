package main

import (
	"math"
	"math/rand"

	"asda2/shared/types"
)

const (
	itemBonusNone   int16 = 0
	itemBonusMaxAtk int16 = 1
	itemBonusMaxDef int16 = 3
	itemBonusMaxHP  int16 = 5
	itemBonusMaxMP  int16 = 6
	itemBonusMinAtk int16 = 64
	itemBonusMinDef int16 = 66
	itemBonusAtk    int16 = 67
	itemBonusMAtk   int16 = 68
	itemBonusDef    int16 = 69
	itemBonusDodge  int16 = 70
	itemBonusBlock  int16 = 73
)

const optionStatStartsWithEnchantValue = 7

var itemOptionRandFloat = rand.Float64

func generateNewEquipmentOptions(item *ItemRow) {
	if item == nil || !isEquipmentTemplate(itemTemplateByID(item.ItemID)) {
		return
	}
	mult := itemOptionValueMultiplier(item)
	item.Param1Type = itemBonusMinDef
	item.Param1Value = randomOptionValue(item, 3, 8, mult)
	item.Param2Type = randomCommonOptionType(item)
	item.Param2Value = randomOptionValue(item, 4, 12, mult)
	if item.IsCrafted {
		generateCraftedEquipmentOption(item)
	}
	if item.Enchant >= optionStatStartsWithEnchantValue {
		generateUpgradeEquipmentOption(item)
	}
}

func generateCraftedEquipmentOption(item *ItemRow) {
	if item == nil {
		return
	}
	item.Param3Type = randomCraftOptionType(item)
	item.Param3Value = uint16(randomOptionValue(item, 5, 15, itemOptionValueMultiplier(item)))
}

func generateUpgradeEquipmentOption(item *ItemRow) {
	if item == nil {
		return
	}
	if item.Param4Type == itemBonusNone {
		item.Param4Type = randomEnchantOptionType(item)
	}
	value := float64(randomOptionValue(item, 6, 18, itemOptionValueMultiplier(item)))
	value *= calculateEnchantMultiplierNotDamageItemStats(item.Enchant)
	item.Param4Value = clampItemOptionInt16(int(math.Round(value)))
}

func generateAdvancedEquipmentOption(item *ItemRow) {
	if item == nil {
		return
	}
	item.Param5Type = randomAdvancedOptionType(item)
	item.Param5Value = randomOptionValue(item, 8, 22, itemOptionValueMultiplier(item))
}

func itemOptionValueMultiplier(item *ItemRow) float64 {
	if item == nil {
		return 1
	}
	templ := itemTemplateByID(item.ItemID)
	mult := 1.0
	if item.IsCrafted {
		mult += 0.5
	}
	switch templ.Quality {
	case types.ItemQualityYellow:
		mult += 0.1
	case types.ItemQualityPurple:
		mult += 0.2
	case types.ItemQualityGreen:
		mult += 0.35
	case types.ItemQualityOrange:
		mult += 0.5
	}
	return mult
}

func randomCommonOptionType(item *ItemRow) int16 {
	templ := itemTemplateByID(item.ItemID)
	if templ.Kind == types.ItemKindWeapon {
		if templ.Category == types.ItemCategoryStaff {
			return itemBonusMAtk
		}
		return itemBonusAtk
	}
	if templ.Kind == types.ItemKindArmor {
		return itemBonusDef
	}
	return oneOfOptionTypes([]int16{itemBonusMaxHP, itemBonusMaxMP, itemBonusDodge})
}

func randomCraftOptionType(item *ItemRow) int16 {
	templ := itemTemplateByID(item.ItemID)
	if templ.Kind == types.ItemKindWeapon {
		return oneOfOptionTypes([]int16{itemBonusMaxAtk, itemBonusAtk, itemBonusDodge})
	}
	if templ.Kind == types.ItemKindArmor {
		return oneOfOptionTypes([]int16{itemBonusMaxDef, itemBonusDef, itemBonusMaxHP})
	}
	return oneOfOptionTypes([]int16{itemBonusMaxHP, itemBonusMaxMP, itemBonusBlock})
}

func randomEnchantOptionType(item *ItemRow) int16 {
	templ := itemTemplateByID(item.ItemID)
	if templ.Kind == types.ItemKindWeapon {
		return oneOfOptionTypes([]int16{itemBonusMaxAtk, itemBonusMinAtk, itemBonusAtk})
	}
	return oneOfOptionTypes([]int16{itemBonusMaxDef, itemBonusMinDef, itemBonusDef})
}

func randomAdvancedOptionType(item *ItemRow) int16 {
	templ := itemTemplateByID(item.ItemID)
	if templ.Kind == types.ItemKindWeapon {
		return oneOfOptionTypes([]int16{itemBonusMaxAtk, itemBonusAtk, itemBonusDodge})
	}
	return oneOfOptionTypes([]int16{itemBonusMaxDef, itemBonusDef, itemBonusBlock})
}

func oneOfOptionTypes(values []int16) int16 {
	if len(values) == 0 {
		return itemBonusNone
	}
	idx := int(itemOptionRandFloat() * float64(len(values)))
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func randomOptionValue(item *ItemRow, min int, max int, mult float64) int16 {
	if max < min {
		max = min
	}
	templ := itemTemplateByID(item.ItemID)
	levelBoost := int(templ.RequiredLevel) / 10
	raw := min + levelBoost + int(itemOptionRandFloat()*float64(max-min+1))
	return clampItemOptionInt16(int(math.Round(float64(raw) * mult)))
}

func calculateEnchantMultiplierNotDamageItemStats(enchant byte) float64 {
	if enchant == 0 {
		return 1
	}
	return math.Pow(float64(enchant), 0.065)
}

func isEquipmentTemplate(templ ItemTemplate) bool {
	switch templ.Kind {
	case types.ItemKindWeapon, types.ItemKindArmor, types.ItemKindAvatar, types.ItemKindAccessory:
		return true
	default:
		return templ.EquipmentSlot >= 0
	}
}

func clampItemOptionInt16(v int) int16 {
	if v < -32768 {
		return -32768
	}
	if v > 32767 {
		return 32767
	}
	return int16(v)
}
