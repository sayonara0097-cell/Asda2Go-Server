package main

import (
	"encoding/binary"
	"log"

	"asda2/shared/types"
)

// ---- Warehouse ----

const (
	warehouseStatusCantFindItem       byte = 0
	warehouseStatusOK                 byte = 1
	warehouseStatusNotEnoughSlots     byte = 2
	warehouseStatusOnlyForSoulmate    byte = 3
	warehouseStatusItemNotFound       byte = 4
	warehouseStatusNotEnoughSlotsInWh byte = 5
	warehouseStatusNotEnoughGold      byte = 6
	warehouseStatusWeightLimit        byte = 7
	warehouseStatusAlreadyUsing       byte = 8
)

const warehouseTakeCommissionPerItem = int64(30)

type warehouseItemStub struct {
	slot   int16
	inv    byte
	amount int
	weight int16
}

func handlePushToWarehouse(c *Client, p *PacketIn) {
	moveRegularWarehouseItems(c, p, false)
}

func handleTakeItemFromWarehouse(c *Client, p *PacketIn) {
	moveRegularWarehouseItems(c, p, true)
}

func handleShowWarehouse(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	page, ok := warehousePageFromRequest(c.Char, p.Data, false)
	if !ok {
		c.Send(NewPacket(ShowWarehouseEnd))
		return
	}
	sendWarehouseItems(c, types.InventoryWarehouse, ShowWarehouseItems, ShowWarehouseEnd, page)
}

func handleShowAvatarWhItems(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	page, ok := warehousePageFromRequest(c.Char, p.Data, true)
	if !ok {
		c.Send(NewPacket(AvatarWhItemsListEnded))
		return
	}
	sendWarehouseItems(c, types.InventoryAvatarWarehouse, AvatarWhItemsList, AvatarWhItemsListEnded, page)
}

func handleStoreAvatarItems(c *Client, p *PacketIn) {
	moveAvatarWarehouseItems(c, p, false)
}

func handleRetriveItemsFromAvatarWh(c *Client, p *PacketIn) {
	moveAvatarWarehouseItems(c, p, true)
}

func moveRegularWarehouseItems(c *Client, p *PacketIn, fromWarehouse bool) {
	moveWarehouseItems(c, p, fromWarehouse, types.InventoryWarehouse)
}

func moveAvatarWarehouseItems(c *Client, p *PacketIn, fromWarehouse bool) {
	moveWarehouseItems(c, p, fromWarehouse, types.InventoryAvatarWarehouse)
}

func moveWarehouseItems(c *Client, p *PacketIn, fromWarehouse bool, warehouseInv byte) {
	if c.Char == nil {
		return
	}
	stubs := readWarehouseItemStubs(p.Data)
	if len(stubs) == 0 {
		sendWarehouseTransfer(c, fromWarehouse, warehouseStatusItemNotFound, nil, nil, warehouseInv)
		return
	}
	if fromWarehouse {
		status, source, dest := takeItemsFromWarehouse(c.Char, stubs, warehouseInv)
		sendWarehouseTransfer(c, true, status, source, dest, warehouseInv)
		return
	}
	status, source, dest := pushItemsToWarehouse(c.Char, stubs, warehouseInv)
	sendWarehouseTransfer(c, false, status, source, dest, warehouseInv)
}

func warehousePageFromRequest(chr *Character, data []byte, avatar bool) (byte, bool) {
	if len(data) == 0 {
		return 0xFF, true
	}
	requested := data[0]
	if requested == 0 {
		return 0, false
	}
	maxExtraBags := chr.PremiumWarehouseBagsCount
	if avatar {
		maxExtraBags = chr.PremiumAvatarWarehouseBagsCount
	}
	page := requested - 1
	return page, page <= maxExtraBags
}

func sendWarehouseItems(c *Client, inv byte, itemOp Opcode, endOp Opcode, page byte) {
	if c == nil || c.Char == nil {
		return
	}
	items := itemsInInventory(c.Char.Items, inv)
	if page != 0xFF {
		start := int16(page) * types.DefaultWarehouseBagSlots
		end := start + types.DefaultWarehouseBagSlots
		filtered := items[:0]
		for _, item := range items {
			if item != nil && item.Slot >= start && item.Slot < end {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	for _, batch := range chunkItems(items, 10) {
		p := NewPacket(itemOp)
		for _, item := range batch {
			writeItemInfoToPacket(p, item, c.Char, false)
		}
		c.Send(p)
	}
	c.Send(NewPacket(endOp))
}

func readWarehouseItemStubs(data []byte) []warehouseItemStub {
	stubs := make([]warehouseItemStub, 0, 5)
	for offset := 0; offset+10 < len(data) && len(stubs) < 5; offset += 11 {
		slot := int16(binary.LittleEndian.Uint16(data[offset:]))
		if slot == -1 {
			continue
		}
		amount := int(binary.LittleEndian.Uint32(data[offset+5:]))
		if amount <= 0 {
			amount = 1
		}
		stubs = append(stubs, warehouseItemStub{
			slot:   slot,
			inv:    data[offset+4],
			amount: amount,
			weight: int16(binary.LittleEndian.Uint16(data[offset+9:])),
		})
	}
	if len(stubs) == 0 && len(data) >= 5 {
		slot := int16(binary.LittleEndian.Uint16(data[1:]))
		stubs = append(stubs, warehouseItemStub{slot: slot, inv: data[0], amount: 1})
	}
	return stubs
}

func pushItemsToWarehouse(chr *Character, stubs []warehouseItemStub, warehouseInv byte) (byte, []warehouseItemStub, []warehouseItemStub) {
	if chr == nil || len(stubs) == 0 {
		return warehouseStatusItemNotFound, nil, nil
	}
	for _, stub := range stubs {
		item := findItem(chr, stub.inv, stub.slot)
		if item == nil || item.ItemID == int(goldLootItemID) || stub.amount <= 0 || item.Amount < stub.amount {
			return warehouseStatusItemNotFound, nil, nil
		}
		if itemLockedForTrading(chr, item) {
			return warehouseStatusAlreadyUsing, nil, nil
		}
	}
	if freeInventorySlotCount(chr, warehouseInv) < requiredWarehouseSlots(chr, stubs, warehouseInv) {
		return warehouseStatusNotEnoughSlotsInWh, nil, nil
	}
	sourceStubs := make([]warehouseItemStub, 0, len(stubs))
	destStubs := make([]warehouseItemStub, 0, len(stubs))
	for _, stub := range stubs {
		item := findItem(chr, stub.inv, stub.slot)
		oldSlot := item.Slot
		oldInv := item.InventoryType
		existingTarget := findItemByID(chr, warehouseInv, item.ItemID)
		added, status, err := createCharacterItemDetailed(chr, item.ItemID, stub.amount, warehouseInv, -1, item, itemTotalWeight(item))
		if err != nil || status != inventoryStatusOK || added == nil {
			log.Printf("[Warehouse] push failed char=%q item=%d status=%d err=%v", chr.Name, item.ItemID, status, err)
			return warehouseStatusItemNotFound, sourceStubs, destStubs
		}
		if err := removeCharacterItem(chr, item, stub.amount); err != nil {
			rollbackWarehouseAdd(chr, added, existingTarget, stub.amount)
			log.Printf("[Warehouse] remove source failed char=%q item=%d: %v", chr.Name, item.ItemID, err)
			return warehouseStatusItemNotFound, sourceStubs, destStubs
		}
		sourceStubs = append(sourceStubs, warehouseItemStub{slot: oldSlot, inv: oldInv, amount: item.Amount, weight: int16(itemUnitWeight(item))})
		destStubs = append(destStubs, stubFromItem(added))
	}
	return warehouseStatusOK, sourceStubs, destStubs
}

func requiredWarehouseSlots(chr *Character, stubs []warehouseItemStub, warehouseInv byte) int {
	required := 0
	seenNewStacks := map[int]bool{}
	for _, stub := range stubs {
		item := findItem(chr, stub.inv, stub.slot)
		if item == nil {
			required++
			continue
		}
		if effectiveItemStackable(item) {
			if findItemByID(chr, warehouseInv, item.ItemID) != nil || seenNewStacks[item.ItemID] {
				continue
			}
			seenNewStacks[item.ItemID] = true
		}
		required++
	}
	return required
}

func takeItemsFromWarehouse(chr *Character, stubs []warehouseItemStub, warehouseInv byte) (byte, []warehouseItemStub, []warehouseItemStub) {
	if chr == nil || len(stubs) == 0 {
		return warehouseStatusItemNotFound, nil, nil
	}
	for _, stub := range stubs {
		item := findItem(chr, warehouseInv, stub.slot)
		if item == nil || stub.amount <= 0 || item.Amount < stub.amount {
			return warehouseStatusItemNotFound, nil, nil
		}
	}
	requiredSlots := map[byte]int{}
	addedWeight := 0
	seenStacks := map[byte]map[int]bool{}
	for _, stub := range stubs {
		item := findItem(chr, warehouseInv, stub.slot)
		targetInv := targetInventoryForTemplate(itemTemplateByID(item.ItemID))
		if effectiveItemStackable(item) {
			if seenStacks[targetInv] == nil {
				seenStacks[targetInv] = map[int]bool{}
			}
			if findItemByID(chr, targetInv, item.ItemID) == nil && !seenStacks[targetInv][item.ItemID] {
				requiredSlots[targetInv]++
				seenStacks[targetInv][item.ItemID] = true
			}
		} else {
			requiredSlots[targetInv] += stub.amount
		}
		if isCarriedInventory(targetInv) {
			addedWeight += itemUnitWeight(item) * stub.amount
		}
	}
	for inv, count := range requiredSlots {
		if freeInventorySlotCount(chr, inv) < count {
			return warehouseStatusNotEnoughSlots, nil, nil
		}
	}
	if carriedWeight(chr)+addedWeight > maxWeight(chr) {
		return warehouseStatusWeightLimit, nil, nil
	}
	if c := warehouseTakeCommissionPerItem * int64(len(stubs)); c > chr.Gold {
		return warehouseStatusNotEnoughGold, nil, nil
	}
	sourceStubs := make([]warehouseItemStub, 0, len(stubs))
	destStubs := make([]warehouseItemStub, 0, len(stubs))
	for _, stub := range stubs {
		item := findItem(chr, warehouseInv, stub.slot)
		targetInv := targetInventoryForTemplate(itemTemplateByID(item.ItemID))
		existingTarget := findItemByID(chr, targetInv, item.ItemID)
		added, status, err := createCharacterItemDetailed(chr, item.ItemID, stub.amount, targetInv, -1, item, 0)
		if status == inventoryStatusWeightExceeds {
			return warehouseStatusWeightLimit, sourceStubs, destStubs
		}
		if status == inventoryStatusNoSpace {
			return warehouseStatusNotEnoughSlots, sourceStubs, destStubs
		}
		if err != nil || status != inventoryStatusOK || added == nil {
			log.Printf("[Warehouse] take failed char=%q item=%d status=%d err=%v", chr.Name, item.ItemID, status, err)
			return warehouseStatusItemNotFound, sourceStubs, destStubs
		}
		oldSlot := item.Slot
		oldInv := item.InventoryType
		if err := removeCharacterItem(chr, item, stub.amount); err != nil {
			rollbackWarehouseAdd(chr, added, existingTarget, stub.amount)
			log.Printf("[Warehouse] remove warehouse item failed char=%q item=%d: %v", chr.Name, item.ItemID, err)
			return warehouseStatusItemNotFound, sourceStubs, destStubs
		}
		sourceStubs = append(sourceStubs, warehouseItemStub{slot: oldSlot, inv: oldInv, amount: item.Amount, weight: int16(itemUnitWeight(item))})
		destStubs = append(destStubs, stubFromItem(added))
	}
	chr.Gold -= warehouseTakeCommissionPerItem * int64(len(stubs))
	_ = SaveCharacter(chr)
	return warehouseStatusOK, sourceStubs, destStubs
}

func rollbackWarehouseAdd(chr *Character, added *ItemRow, existingTarget *ItemRow, amount int) {
	if added == nil {
		return
	}
	if existingTarget != nil && added.Guid == existingTarget.Guid {
		existingTarget.Amount -= amount
		if existingTarget.Amount <= 0 {
			discardCreatedWarehouseItem(chr, existingTarget)
			return
		}
		_ = SaveItem(existingTarget)
		return
	}
	discardCreatedWarehouseItem(chr, added)
}

func discardCreatedWarehouseItem(chr *Character, item *ItemRow) {
	if chr == nil || item == nil {
		return
	}
	_ = DeleteItem(item.Guid)
	for i, it := range chr.Items {
		if it == item || it.Guid == item.Guid {
			chr.Items = append(chr.Items[:i], chr.Items[i+1:]...)
			return
		}
	}
}

func sendWarehouseTransfer(c *Client, fromWarehouse bool, status byte, source []warehouseItemStub, dest []warehouseItemStub, warehouseInv byte) {
	op := ItemsPushedToWarehouse
	if warehouseInv == types.InventoryAvatarWarehouse {
		op = AvatarItemsStored
	}
	if fromWarehouse {
		op = ItemFormWarehouseTaked
		if warehouseInv == types.InventoryAvatarWarehouse {
			op = ItemsFromAvatarWhRetrived
		}
	}
	p := NewPacket(op)
	p.WriteUint8(status)
	if status == warehouseStatusOK {
		if fromWarehouse {
			writeWarehouseStubs(p, dest)
			writeWarehouseStubs(p, source)
			p.WriteInt32(clampInt32(c.Char.Gold))
		} else {
			writeWarehouseStubs(p, source)
			writeWarehouseStubs(p, dest)
		}
		p.WriteInt16(itemWeight(c.Char))
		p.WriteInt16(int16(len(itemsInInventory(c.Char.Items, warehouseInv))))
	}
	c.Send(p)
}

func writeWarehouseStubs(p *PacketOut, stubs []warehouseItemStub) {
	for i := 0; i < 5; i++ {
		if i >= len(stubs) {
			p.WriteInt32(-1)
			p.WriteUint8(0)
			p.WriteInt32(-1)
			p.WriteInt16(0)
			continue
		}
		stub := stubs[i]
		p.WriteInt32(int32(stub.slot))
		p.WriteUint8(stub.inv)
		p.WriteInt32(int32(stub.amount))
		p.WriteInt16(stub.weight)
	}
}

func stubFromItem(item *ItemRow) warehouseItemStub {
	if item == nil {
		return warehouseItemStub{slot: -1, amount: -1}
	}
	return warehouseItemStub{
		slot:   item.Slot,
		inv:    item.InventoryType,
		amount: item.Amount,
		weight: int16(itemUnitWeight(item)),
	}
}

func sendWarehouseSlotsExpanded(c *Client, avatar bool) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(WarehouseSlotsExpanded)
	if avatar {
		p.WriteUint8(3)
		p.WriteInt16(inventorySlotCapacity(c.Char, types.InventoryAvatarWarehouse))
	} else {
		p.WriteUint8(2)
		p.WriteInt16(inventorySlotCapacity(c.Char, types.InventoryWarehouse))
	}
	c.Send(p)
}
