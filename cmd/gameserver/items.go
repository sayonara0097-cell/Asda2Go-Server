package main

import (
	"log"
	"math"

	"asda2/shared/types"
)

const (
	itemStatusFail byte = 0
	itemStatusOK   byte = 1
)

const (
	inventoryStatusNoSpace           byte = 0
	inventoryStatusOK                byte = 1
	inventoryStatusNotInfoAboutItem  byte = 2
	inventoryStatusItemIsNotForEquip byte = 3
	inventoryStatusFail              byte = 4
	inventoryStatusWeightExceeds     byte = 5
)

const (
	equipmentSlotLeftRing  int16 = 5
	equipmentSlotRightRing int16 = 6
	equipmentSlotShield    int16 = 8
	equipmentSlotAnyArmor  int16 = 20
	equipmentSlotAnyAvatar int16 = 21
	equipmentSlotJewelry   int16 = 22
)

func initItemRuntime() error {
	_, err := InitItemTemplateCache()
	return err
}

func inventoryItems(chr *Character, inv byte) []*ItemRow {
	if chr == nil {
		return nil
	}
	return itemsInInventory(chr.Items, inv)
}

func findItem(chr *Character, inv byte, slot int16) *ItemRow {
	if chr == nil {
		return nil
	}
	return itemInInventorySlot(chr.Items, inv, slot)
}

func findItemByID(chr *Character, inv byte, itemID int) *ItemRow {
	for _, item := range inventoryItems(chr, inv) {
		if item.ItemID == itemID {
			return item
		}
	}
	return nil
}

func inventorySlotCapacity(chr *Character, inv byte) int16 {
	switch inv {
	case types.InventoryShop:
		return types.DefaultShopInventorySlots
	case types.InventoryWarehouse:
		bags := byte(0)
		if chr != nil {
			bags = chr.PremiumWarehouseBagsCount
		}
		return premiumBagSlots(bags)
	case types.InventoryAvatarWarehouse:
		bags := byte(0)
		if chr != nil {
			bags = chr.PremiumAvatarWarehouseBagsCount
		}
		return premiumBagSlots(bags)
	default:
		return types.InventorySlotCount(inv)
	}
}

func premiumBagSlots(extraBags byte) int16 {
	slots := int16(extraBags+1) * types.DefaultWarehouseBagSlots
	if slots > types.WarehouseInventorySlots {
		return types.WarehouseInventorySlots
	}
	return slots
}

func validCharacterInventorySlot(chr *Character, inv byte, slot int16) bool {
	return slot >= 0 && slot < inventorySlotCapacity(chr, inv)
}

func freeInventorySlot(chr *Character, inv byte) (int16, bool) {
	max := inventorySlotCapacity(chr, inv)
	start := int16(0)
	if inv == types.InventoryRegular {
		start = types.ReservedRegularInventorySlot + 1
	}
	for slot := start; slot < max; slot++ {
		if findItem(chr, inv, slot) == nil {
			return slot, true
		}
	}
	return -1, false
}

func freeInventorySlotCount(chr *Character, inv byte) int {
	count := 0
	max := inventorySlotCapacity(chr, inv)
	start := int16(0)
	if inv == types.InventoryRegular {
		start = types.ReservedRegularInventorySlot + 1
	}
	for slot := start; slot < max; slot++ {
		if findItem(chr, inv, slot) == nil {
			count++
		}
	}
	return count
}

func targetInventoryForTemplate(templ ItemTemplate) byte {
	if templ.InventoryType == 0 || templ.InventoryType == types.InventoryEquipment {
		return types.InventoryRegular
	}
	return templ.InventoryType
}

func createCharacterItem(chr *Character, itemID int, amount int, inv byte, slot int16) (*ItemRow, error) {
	item, status, err := createCharacterItemDetailed(chr, itemID, amount, inv, slot, nil, 0)
	if status != inventoryStatusOK {
		return nil, err
	}
	return item, err
}

func createCharacterItemDetailed(chr *Character, itemID int, amount int, inv byte, slot int16, copyFrom *ItemRow, carriedWeightCredit int) (*ItemRow, byte, error) {
	if chr == nil {
		return nil, inventoryStatusFail, nil
	}
	templ := itemTemplateByID(itemID)
	if inv == 0 {
		inv = templ.InventoryType
	}
	if inv == 0 {
		inv = types.InventoryRegular
	}
	if types.InventorySlotCount(inv) == 0 {
		return nil, inventoryStatusFail, nil
	}
	if amount <= 0 {
		amount = 1
	}
	if templ.IsStackable {
		if existing := findItemByID(chr, inv, itemID); existing != nil {
			if !canCarryAdditionalItem(chr, inv, itemUnitWeight(existing), amount, carriedWeightCredit) {
				return nil, inventoryStatusWeightExceeds, nil
			}
			if copyFrom != nil && copyFrom.IsSoulBound {
				existing.IsSoulBound = true
			}
			existing.Amount += amount
			if err := SaveItem(existing); err != nil {
				return nil, inventoryStatusFail, err
			}
			return existing, inventoryStatusOK, nil
		}
	} else {
		amount = 1
	}
	if slot < 0 {
		var ok bool
		slot, ok = freeInventorySlot(chr, inv)
		if !ok {
			return nil, inventoryStatusNoSpace, nil
		}
	} else if !validCharacterInventorySlot(chr, inv, slot) || (inv == types.InventoryRegular && slot == types.ReservedRegularInventorySlot) {
		return nil, inventoryStatusNoSpace, nil
	}
	if !canCarryAdditionalItem(chr, inv, itemTemplateUnitWeight(templ, copyFrom), amount, carriedWeightCredit) {
		return nil, inventoryStatusWeightExceeds, nil
	}
	guid, err := NextItemGUID()
	if err != nil {
		return nil, inventoryStatusFail, err
	}
	item := &ItemRow{
		Guid:          guid,
		OwnerID:       chr.GUID,
		ItemID:        itemID,
		InventoryType: inv,
		Slot:          slot,
		Durability:    templ.MaxDurability,
		IsStackable:   templ.IsStackable,
		Weight:        templ.Weight,
		Amount:        amount,
		OwnerName:     chr.Name,
	}
	if copyFrom != nil {
		copyMutableItemState(item, copyFrom)
		item.IsStackable = templ.IsStackable || effectiveItemStackable(copyFrom)
		item.InventoryType = inv
		item.Slot = slot
		item.Amount = amount
		item.OwnerID = chr.GUID
		item.OwnerName = chr.Name
		item.Guid = guid
	}
	if item.Durability == 0 && (templ.Kind == types.ItemKindWeapon || templ.Kind == types.ItemKindArmor) {
		item.Durability = 100
	}
	if err := SaveItem(item); err != nil {
		return nil, inventoryStatusFail, err
	}
	chr.Items = append(chr.Items, item)
	return item, inventoryStatusOK, nil
}

func copyMutableItemState(dst *ItemRow, src *ItemRow) {
	if dst == nil || src == nil {
		return
	}
	dst.CreatorID = src.CreatorID
	dst.Durability = src.Durability
	dst.Duration = src.Duration
	dst.IsSoulBound = src.IsSoulBound
	dst.Soul1ID = src.Soul1ID
	dst.Soul2ID = src.Soul2ID
	dst.Soul3ID = src.Soul3ID
	dst.Soul4ID = src.Soul4ID
	dst.Enchant = src.Enchant
	dst.EnchantResetCount = src.EnchantResetCount
	dst.Param1Type = src.Param1Type
	dst.Param1Value = src.Param1Value
	dst.Param2Type = src.Param2Type
	dst.Param2Value = src.Param2Value
	dst.Param3Type = src.Param3Type
	dst.Param3Value = src.Param3Value
	dst.Param4Type = src.Param4Type
	dst.Param4Value = src.Param4Value
	dst.Param5Type = src.Param5Type
	dst.Param5Value = src.Param5Value
	dst.IsStackable = src.IsStackable
	dst.CreatorEntityID = src.CreatorEntityID
	dst.Weight = src.Weight
	dst.SealCount = src.SealCount
	dst.AuctionPrice = src.AuctionPrice
	dst.AuctionEndTime = src.AuctionEndTime
	dst.IsCrafted = src.IsCrafted
}

func removeCharacterItem(chr *Character, item *ItemRow, amount int) error {
	if chr == nil || item == nil {
		return nil
	}
	if amount <= 0 || amount >= item.Amount || !effectiveItemStackable(item) {
		if err := DeleteItem(item.Guid); err != nil {
			return err
		}
		for i, it := range chr.Items {
			if it == item || it.Guid == item.Guid {
				chr.Items = append(chr.Items[:i], chr.Items[i+1:]...)
				break
			}
		}
		item.Amount = 0
		return nil
	}
	item.Amount -= amount
	return SaveItem(item)
}

func moveOrSwapItem(chr *Character, srcInv byte, srcSlot int16, destInv byte, destSlot int16) (byte, *ItemRow, *ItemRow) {
	src := findItem(chr, srcInv, srcSlot)
	if src == nil {
		return inventoryStatusNotInfoAboutItem, nil, nil
	}
	if srcInv == types.InventoryEquipment && isCarriedInventory(destInv) {
		resolvedSlot, status := resolveUnequipDestinationSlot(chr, destInv, destSlot)
		if status != inventoryStatusOK {
			return status, src, nil
		}
		destSlot = resolvedSlot
	}
	if status := validateMoveRequest(chr, src, srcInv, srcSlot, destInv, &destSlot); status != inventoryStatusOK {
		return status, src, nil
	}
	if status := validateEquipmentDestination(chr, src, destInv, destSlot); status != inventoryStatusOK {
		return status, src, nil
	}
	dest := findItem(chr, destInv, destSlot)
	if status := validateMoveWeight(chr, src, srcInv, dest, destInv); status != inventoryStatusOK {
		return status, src, dest
	}
	oldSrcInv, oldSrcSlot := src.InventoryType, src.Slot
	if dest != nil {
		dest.InventoryType = oldSrcInv
		dest.Slot = oldSrcSlot
		if err := SaveItem(dest); err != nil {
			log.Printf("[Items] save swapped item guid=%d: %v", dest.Guid, err)
			return inventoryStatusFail, src, dest
		}
	}
	src.InventoryType = destInv
	src.Slot = destSlot
	if err := SaveItem(src); err != nil {
		log.Printf("[Items] save moved item guid=%d: %v", src.Guid, err)
		return inventoryStatusFail, src, dest
	}
	return inventoryStatusOK, src, dest
}

func resolveUnequipDestinationSlot(chr *Character, inv byte, requestedSlot int16) (int16, byte) {
	if !isCarriedInventory(inv) {
		return requestedSlot, inventoryStatusNotInfoAboutItem
	}
	if availableInventorySlot(chr, inv, requestedSlot) {
		return requestedSlot, inventoryStatusOK
	}
	freeSlot, ok := freeInventorySlot(chr, inv)
	if !ok {
		return -1, inventoryStatusNoSpace
	}
	return freeSlot, inventoryStatusOK
}

func availableInventorySlot(chr *Character, inv byte, slot int16) bool {
	if !validCharacterInventorySlot(chr, inv, slot) {
		return false
	}
	if inv == types.InventoryRegular && slot == types.ReservedRegularInventorySlot {
		return false
	}
	return findItem(chr, inv, slot) == nil
}

func validateMoveRequest(chr *Character, src *ItemRow, srcInv byte, srcSlot int16, destInv byte, destSlot *int16) byte {
	if src == nil || src.InventoryType != srcInv || src.Slot != srcSlot {
		return inventoryStatusNotInfoAboutItem
	}
	if itemLockedForTrading(chr, src) {
		return inventoryStatusFail
	}
	if !validCharacterInventorySlot(chr, srcInv, srcSlot) {
		return inventoryStatusNotInfoAboutItem
	}
	if destSlot == nil {
		return inventoryStatusNotInfoAboutItem
	}
	if !validCharacterInventorySlot(chr, destInv, *destSlot) {
		return inventoryStatusNotInfoAboutItem
	}
	if (srcInv == types.InventoryRegular && srcSlot == types.ReservedRegularInventorySlot) ||
		(destInv == types.InventoryRegular && *destSlot == types.ReservedRegularInventorySlot) {
		return inventoryStatusFail
	}
	switch srcInv {
	case types.InventoryRegular:
		return inventoryStatusOK
	case types.InventoryShop:
		if destInv != types.InventoryShop && destInv != types.InventoryEquipment {
			return inventoryStatusNotInfoAboutItem
		}
	case types.InventoryEquipment:
		if destInv != types.InventoryShop && destInv != types.InventoryRegular {
			return inventoryStatusNotInfoAboutItem
		}
	case types.InventoryWarehouse:
		if destInv != types.InventoryWarehouse {
			return inventoryStatusNotInfoAboutItem
		}
	case types.InventoryAvatarWarehouse:
		if destInv != types.InventoryAvatarWarehouse {
			return inventoryStatusNotInfoAboutItem
		}
	default:
		return inventoryStatusNotInfoAboutItem
	}
	return inventoryStatusOK
}

func validateEquipmentDestination(chr *Character, item *ItemRow, destInv byte, destSlot int16) byte {
	if destInv != types.InventoryEquipment {
		return inventoryStatusOK
	}
	templ := itemTemplateByID(item.ItemID)
	if templ.EquipmentSlot < 0 {
		return inventoryStatusItemIsNotForEquip
	}
	if destSlot == equipmentSlotWeapon && chr != nil &&
		templ.Category != types.ItemCategoryOneHandedSword &&
		findItem(chr, types.InventoryEquipment, equipmentSlotShield) != nil {
		return inventoryStatusItemIsNotForEquip
	}
	if !equipmentSlotMatches(templ, destSlot) {
		return inventoryStatusItemIsNotForEquip
	}
	if templ.RequiredLevel > 0 && chr != nil && templ.RequiredLevel > chr.Level {
		return inventoryStatusFail
	}
	if templ.RequiredProfession > 0 && chr != nil && templ.RequiredProfession > chr.RealProfessionLevel() {
		return inventoryStatusFail
	}
	return inventoryStatusOK
}

func equipmentSlotMatches(templ ItemTemplate, destSlot int16) bool {
	switch templ.EquipmentSlot {
	case equipmentSlotLeftRing:
		return destSlot == equipmentSlotLeftRing || destSlot == equipmentSlotRightRing
	case equipmentSlotAnyArmor:
		return destSlot >= 0 && destSlot <= 4
	case equipmentSlotAnyAvatar:
		return destSlot >= 11 && destSlot <= 15
	case equipmentSlotJewelry:
		return destSlot >= 5 && destSlot <= 7
	default:
		return templ.EquipmentSlot == destSlot
	}
}

func validateMoveWeight(chr *Character, src *ItemRow, srcInv byte, dest *ItemRow, destInv byte) byte {
	if !isCarriedInventory(destInv) || isCarriedInventory(srcInv) {
		return inventoryStatusOK
	}
	credit := 0
	if dest != nil && isCarriedInventory(dest.InventoryType) {
		credit = itemTotalWeight(dest)
	}
	if !canCarryAdditionalItem(chr, destInv, itemUnitWeight(src), src.Amount, credit) {
		return inventoryStatusWeightExceeds
	}
	return inventoryStatusOK
}

func effectiveItemStackable(item *ItemRow) bool {
	return types.ItemIsStackable(item)
}

func itemWeight(chr *Character) int16 {
	total := carriedWeight(chr)
	if total > math.MaxInt16 {
		return math.MaxInt16
	}
	return int16(total)
}

func carriedWeight(chr *Character) int {
	total := 0
	if chr == nil {
		return total
	}
	for _, item := range chr.Items {
		if item == nil || !isCarriedInventory(item.InventoryType) {
			continue
		}
		total += itemTotalWeight(item)
	}
	return total
}

func maxWeight(chr *Character) int {
	return types.DefaultCharacterMaxWeight
}

func isCarriedInventory(inv byte) bool {
	return inv == types.InventoryRegular || inv == types.InventoryShop
}

func itemTemplateUnitWeight(templ ItemTemplate, copyFrom *ItemRow) int {
	if copyFrom != nil {
		return itemUnitWeight(copyFrom)
	}
	if templ.Kind == types.ItemKindCurrency {
		return 0
	}
	if templ.Weight == 0 {
		return 1
	}
	return int(templ.Weight)
}

func itemUnitWeight(item *ItemRow) int {
	if item == nil || item.ItemID == int(goldLootItemID) {
		return 0
	}
	weight := int(item.Weight)
	if weight <= 0 {
		templ := itemTemplateByID(item.ItemID)
		weight = itemTemplateUnitWeight(templ, nil)
	}
	return weight
}

func itemTotalWeight(item *ItemRow) int {
	if item == nil {
		return 0
	}
	amount := item.Amount
	if amount <= 0 {
		amount = 1
	}
	return itemUnitWeight(item) * amount
}

func canCarryAdditionalItem(chr *Character, inv byte, unitWeight int, amount int, carriedWeightCredit int) bool {
	if !isCarriedInventory(inv) || unitWeight <= 0 {
		return true
	}
	if amount <= 0 {
		amount = 1
	}
	current := carriedWeight(chr) - carriedWeightCredit
	if current < 0 {
		current = 0
	}
	return current+unitWeight*amount <= maxWeight(chr)
}

func sendSingleInventoryUpdate(c *Client, item *ItemRow) {
	if c == nil || c.Char == nil || item == nil {
		return
	}
	op := RegularInventoryInfo
	if item.InventoryType == types.InventoryShop {
		op = ShopInventoryInfo
	}
	p := NewPacket(op)
	writeItemInfoToPacket(p, item, c.Char, false)
	c.Send(p)
}

func sendInventoryWeightUpdate(c *Client, changedItem *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UpdateShopItemInfo)
	p.WriteUint8(itemStatusOK)
	p.WriteInt16(c.Char.SessionID)
	writeItemInfoToPacket(p, changedItem, c.Char, true)
	p.WriteInt16(itemWeight(c.Char))
	p.WriteInt32(0)
	p.WriteInt16(int16(maxWeight(c.Char)))
	c.Send(p)
}

func broadcastEquipmentChange(c *Client, item *ItemRow, added bool) {
	if item == nil {
		return
	}
	broadcastEquipmentChangeAt(c, item, item.Slot, added)
}

func broadcastEquipmentChangeAt(c *Client, item *ItemRow, slot int16, added bool) {
	if c == nil || c.Char == nil || item == nil {
		return
	}
	op := CharacterAddEquipment
	if !added {
		op = CharacterRemoveEquipment
	}
	p := NewPacket(op)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt32(-1)
	p.WriteInt16(slot)
	p.WriteInt32(int32(item.ItemID))
	if added {
		p.WriteUint8(item.Enchant)
	} else {
		p.WriteInt32(0)
	}
	for _, viewer := range World.CharactersOnMap(c.Char.MapID) {
		if viewer != nil {
			viewer.Send(p)
		}
	}
}
