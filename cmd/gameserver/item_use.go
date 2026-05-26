package main

import "asda2/shared/types"

const (
	useItemStatusFail            byte = 0
	useItemStatusOK              byte = 1
	useItemStatusCooldown        byte = 3
	useItemStatusLevel           byte = 9
	useItemStatusNoActivePet     byte = 13
	useItemStatusPetIsMature     byte = 14
	maxPremiumWarehouseBagsCount byte = 8
)

type itemUseOutcome struct {
	status            byte
	consumed          bool
	added             *ItemRow
	healthChanged     bool
	powerChanged      bool
	warehouseExpanded bool
	equipmentRepaired bool
	characterChanged  bool
	skillsChanged     bool
	functionalUsed    bool
}

func applyInventoryItemUse(chr *Character, item *ItemRow, params ...uint32) itemUseOutcome {
	if chr == nil || item == nil || item.ItemID == int(goldLootItemID) || item.Amount <= 0 {
		return itemUseOutcome{status: useItemStatusFail}
	}
	param := uint32(0)
	if len(params) > 0 {
		param = params[0]
	}
	templ := itemTemplateByID(item.ItemID)
	if templ.RequiredLevel > 0 && templ.RequiredLevel > chr.Level {
		return itemUseOutcome{status: useItemStatusLevel}
	}

	out := itemUseOutcome{status: useItemStatusOK}
	switch templ.UseEffect() {
	case types.ItemUseRecoverHP:
		out.healthChanged = recoverHealthFromItem(chr, templ)
		if templ.Category == types.ItemCategoryInstantRecoverHPMP {
			out.powerChanged = recoverPower(chr, powerRecoverAmount(chr, templ))
		}
	case types.ItemUseRecoverMP:
		out.powerChanged = recoverPower(chr, powerRecoverAmount(chr, templ))
	case types.ItemUseContainer, types.ItemUseBooster:
		resultID := templ.ValueOnUse
		if resultID <= 0 {
			resultID = item.ItemID + 1
		}
		added, status, err := createCharacterItemDetailed(chr, resultID, 1, targetInventoryForTemplate(itemTemplateByID(resultID)), -1, nil, 0)
		if err != nil || status != inventoryStatusOK || added == nil {
			return itemUseOutcome{status: useItemStatusFail}
		}
		out.added = added
	case types.ItemUseExpandWarehouse:
		if chr.PremiumWarehouseBagsCount >= maxPremiumWarehouseBagsCount {
			return itemUseOutcome{status: useItemStatusFail}
		}
		chr.PremiumWarehouseBagsCount++
		out.warehouseExpanded = true
	case types.ItemUseOpenWarehouse:
		// Timed warehouse-opening premium items are treated as a successful
		// activation in the current runtime; the warehouse UI itself is opened
		// by the client through ShowWarehouse.
	case types.ItemUseChangeGender:
		toggleCharacterGender(chr)
		out.characterChanged = true
	case types.ItemUseReviveSelf:
		if chr.HP > 0 {
			return itemUseOutcome{status: useItemStatusFail}
		}
		chr.HP = chr.MaxHP
		chr.MP = chr.MaxMP
		out.healthChanged = true
		out.powerChanged = true
	case types.ItemUseRepairEquipment:
		if !repairCharacterEquipment(chr) {
			return itemUseOutcome{status: useItemStatusFail}
		}
		out.equipmentRepaired = true
	case types.ItemUseResetAllSkills:
		if !resetAllCharacterSkills(chr) {
			return itemUseOutcome{status: useItemStatusFail}
		}
		out.skillsChanged = true
	case types.ItemUseResetOneSkill:
		if param == 0 || param > 32767 || !resetOneCharacterSkill(chr, int16(param)) {
			return itemUseOutcome{status: useItemStatusFail}
		}
		out.skillsChanged = true
	case types.ItemUsePet:
		return itemUseOutcome{status: useItemStatusNoActivePet}
	case types.ItemUseFunctionalBuff:
		activateFunctionalBuff(chr, item)
		out.functionalUsed = true
	case types.ItemUseRecipe, types.ItemUseTeleport, types.ItemUseUnsupported:
		return itemUseOutcome{status: useItemStatusFail}
	default:
		return itemUseOutcome{status: useItemStatusFail}
	}

	if err := removeCharacterItem(chr, item, 1); err != nil {
		return itemUseOutcome{status: useItemStatusFail}
	}
	out.consumed = true
	if out.healthChanged || out.powerChanged || out.warehouseExpanded || out.characterChanged {
		_ = SaveCharacter(chr)
	}
	return out
}

func recoverHealthFromItem(chr *Character, templ ItemTemplate) bool {
	amount := templ.ValueOnUse
	if amount <= 0 || templ.Category == types.ItemCategoryInstantRecoverHP || templ.Category == types.ItemCategoryInstantRecoverHPMP {
		amount = int(chr.MaxHP)
	}
	before := chr.HP
	chr.HP += int32(amount)
	if chr.HP > chr.MaxHP {
		chr.HP = chr.MaxHP
	}
	if chr.HP < 1 {
		chr.HP = 1
	}
	return chr.HP != before
}

func powerRecoverAmount(chr *Character, templ ItemTemplate) int32 {
	if templ.ValueOnUse <= 0 || templ.Category == types.ItemCategoryInstantRecoverHPMP {
		return chr.MaxMP
	}
	return int32(templ.ValueOnUse)
}

func recoverPower(chr *Character, amount int32) bool {
	if chr == nil || amount <= 0 {
		return false
	}
	before := chr.MP
	chr.MP += amount
	if chr.MP > chr.MaxMP {
		chr.MP = chr.MaxMP
	}
	if chr.MP < 0 {
		chr.MP = 0
	}
	return chr.MP != before
}

func removableInventory(inv byte) bool {
	return inv == types.InventoryRegular || inv == types.InventoryShop
}

func toggleCharacterGender(chr *Character) {
	if chr == nil {
		return
	}
	if chr.Gender == 1 {
		chr.Gender = 2
		return
	}
	chr.Gender = 1
}

func repairCharacterEquipment(chr *Character) bool {
	if chr == nil {
		return false
	}
	repaired := false
	for _, item := range chr.Items {
		if item == nil {
			continue
		}
		templ := itemTemplateByID(item.ItemID)
		if templ.Kind != types.ItemKindWeapon && templ.Kind != types.ItemKindArmor &&
			templ.Kind != types.ItemKindAvatar && templ.Kind != types.ItemKindAccessory {
			continue
		}
		maxDurability := templ.MaxDurability
		if maxDurability == 0 {
			maxDurability = 100
		}
		if item.Durability >= maxDurability {
			continue
		}
		item.Durability = maxDurability
		_ = SaveItem(item)
		repaired = true
	}
	return repaired
}

func resetAllCharacterSkills(chr *Character) bool {
	if chr == nil {
		return false
	}
	remove := make([]int16, 0, len(chr.LearnedSkills))
	for skillID := range chr.LearnedSkills {
		remove = append(remove, skillID)
	}
	if len(remove) > 0 {
		if err := DeleteCharacterSkills(chr.GUID, remove); err != nil {
			return false
		}
	}
	chr.LearnedSkills = make(map[int16]byte, len(defaultSkillIDs))
	for _, skillID := range defaultSkillIDs {
		chr.LearnedSkills[skillID] = 1
		if err := SaveCharacterSkill(chr.GUID, skillID, 1); err != nil {
			return false
		}
	}
	return true
}

func resetOneCharacterSkill(chr *Character, skillID int16) bool {
	if chr == nil || skillID <= 0 || chr.LearnedSkills == nil || chr.LearnedSkills[skillID] == 0 {
		return false
	}
	if isDefaultSkillID(skillID) {
		return false
	}
	if err := DeleteCharacterSkills(chr.GUID, []int16{skillID}); err != nil {
		return false
	}
	delete(chr.LearnedSkills, skillID)
	return true
}

func isDefaultSkillID(skillID int16) bool {
	for _, defaultID := range defaultSkillIDs {
		if defaultID == skillID {
			return true
		}
	}
	return false
}

func activateFunctionalBuff(chr *Character, item *ItemRow) {
	if chr == nil || item == nil || item.ItemID <= 0 {
		return
	}
	itemID := int32(item.ItemID)
	for i, existing := range chr.Buffs {
		if existing == 0 || existing == itemID {
			chr.Buffs[i] = itemID
			return
		}
	}
	chr.Buffs[0] = itemID
}
