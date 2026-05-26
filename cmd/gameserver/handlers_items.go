package main

import (
	"encoding/binary"
	"log"

	"asda2/shared/types"
)

// ---- Items ----

const (
	buyStatusFail           byte = 0
	buyStatusOK             byte = 1
	buyStatusNotEnoughSpace byte = 2
	buyStatusWeightExceeds  byte = 3
	buyStatusNotEnoughGold  byte = 4
	buyStatusBadItemID      byte = 5
)

type itemPurchaseRequest struct {
	itemID int
	amount int
}

type itemSellRequest struct {
	slot   int16
	inv    byte
	amount int
}

type itemMoveRequest struct {
	srcSlot    int16
	srcInv     byte
	srcAmount  int32
	srcWeight  int16
	destSlot   int16
	destInv    byte
	destAmount int32
	destWeight int16
}

type itemUseRequest struct {
	inv  byte
	slot int16
}

type itemRemoveRequest struct {
	inv    byte
	slot   int16
	amount int
}

func handleReplaceItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendItemReplaced(c, inventoryStatusNotInfoAboutItem, 0, 0, 0, 0, 0, 0, 0, 0, false)
		return
	}
	req, ok := readMoveItemRequest(c.Char, p.Data)
	if !ok {
		log.Printf("[Items] replace rejected char=%q reason=no-valid-payload len=%d payload=% X", c.Char.Name, len(p.Data), p.Data)
		sendItemReplaced(c, inventoryStatusNotInfoAboutItem, 0, 0, 0, 0, 0, 0, 0, 0, false)
		return
	}
	srcSlot := req.srcSlot
	srcInv := req.srcInv
	srcAmount := req.srcAmount
	srcWeight := req.srcWeight
	destSlot := req.destSlot
	destInv := req.destInv
	destAmount := req.destAmount
	destWeight := req.destWeight
	if srcInv == 0 {
		srcInv = types.InventoryEquipment
	}
	status, moved, swapped := moveOrSwapItem(c.Char, srcInv, srcSlot, destInv, destSlot)
	if status == itemStatusOK {
		if types.IsEquipmentInventory(srcInv) {
			broadcastEquipmentChangeAt(c, moved, srcSlot, false)
			if swapped != nil {
				broadcastEquipmentChangeAt(c, swapped, srcSlot, true)
			}
		}
		if types.IsEquipmentInventory(destInv) {
			if swapped != nil {
				broadcastEquipmentChangeAt(c, swapped, destSlot, false)
			}
			broadcastEquipmentChangeAt(c, moved, destSlot, true)
		}
	}
	if status == inventoryStatusOK {
		if moved != nil {
			destSlot = moved.Slot
			destInv = moved.InventoryType
			destAmount = int32(moved.Amount)
			destWeight = int16(itemUnitWeight(moved))
		}
		if swapped != nil {
			srcAmount = int32(swapped.Amount)
			srcWeight = int16(itemUnitWeight(swapped))
		}
		log.Printf("[Items] replace moved char=%q %d:%d -> %d:%d swapped=%t", c.Char.Name, srcInv, srcSlot, destInv, destSlot, swapped != nil)
	} else {
		log.Printf("[Items] replace rejected char=%q status=%d %d:%d -> %d:%d", c.Char.Name, status, srcInv, srcSlot, destInv, destSlot)
	}
	sendItemReplaced(c, status, srcSlot, srcInv, srcAmount, srcWeight, destSlot, destInv, destAmount, destWeight, swapped == nil)
	if status == inventoryStatusOK && (isCarriedInventory(srcInv) != isCarriedInventory(destInv)) {
		sendInventoryWeightUpdate(c, moved)
	}
	if status == inventoryStatusOK && (types.IsEquipmentInventory(srcInv) || types.IsEquipmentInventory(destInv)) {
		sendUpdateStats(c)
	}
	if status == inventoryStatusOK && destInv == types.InventoryEquipment && moved != nil && moved.Slot == equipmentSlotWeapon {
		autoEquipRangedAmmo(c, moved)
	}
}

func autoEquipRangedAmmo(c *Client, weapon *ItemRow) {
	if c == nil || c.Char == nil || weapon == nil {
		return
	}
	ammoID, ok := defaultAmmoItemIDForWeapon(itemTemplateByID(weapon.ItemID).Category)
	if !ok {
		return
	}
	if equipped := findItem(c.Char, types.InventoryEquipment, equipmentSlotAmmo); equipped != nil && equipped.ItemID == ammoID {
		return
	}
	ammo := findItemByID(c.Char, types.InventoryRegular, ammoID)
	if ammo == nil {
		return
	}
	srcSlot := ammo.Slot
	srcInv := ammo.InventoryType
	status, moved, swapped := moveOrSwapItem(c.Char, srcInv, srcSlot, types.InventoryEquipment, equipmentSlotAmmo)
	if status != inventoryStatusOK || moved == nil {
		return
	}

	srcAmount := int32(moved.Amount)
	srcWeight := int16(itemUnitWeight(moved))
	if swapped != nil {
		srcAmount = int32(swapped.Amount)
		srcWeight = int16(itemUnitWeight(swapped))
		broadcastEquipmentChangeAt(c, swapped, equipmentSlotAmmo, false)
	}
	broadcastEquipmentChangeAt(c, moved, equipmentSlotAmmo, true)
	sendItemReplaced(c, status, srcSlot, srcInv, srcAmount, srcWeight, equipmentSlotAmmo, types.InventoryEquipment, int32(moved.Amount), int16(itemUnitWeight(moved)), swapped == nil)
	sendInventoryWeightUpdate(c, moved)
	sendUpdateStats(c)
}

func defaultAmmoItemIDForWeapon(category int) (int, bool) {
	switch category {
	case types.ItemCategoryBow:
		return 20565, true
	case types.ItemCategoryCrossbow:
		return 20568, true
	case types.ItemCategoryBallista:
		return 20571, true
	default:
		return 0, false
	}
}

func handleRemoveItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendItemRemoved(c, itemStatusFail, nil, 0)
		return
	}
	req, ok := readRemoveItemRequest(c.Char, p.Data)
	if !ok {
		log.Printf("[Items] remove rejected char=%q reason=no-valid-payload len=%d payload=% X", c.Char.Name, len(p.Data), p.Data)
		sendItemRemoved(c, itemStatusFail, nil, 0)
		return
	}
	inv := req.inv
	slot := req.slot
	amount := req.amount
	if amount == 0 {
		amount = 1
	}
	if !removableInventory(inv) {
		sendItemRemoved(c, itemStatusFail, nil, 0)
		return
	}
	item := findItem(c.Char, inv, slot)
	if item == nil || item.ItemID == int(goldLootItemID) {
		sendItemRemoved(c, itemStatusFail, nil, 0)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendItemRemoved(c, itemStatusFail, item, amount)
		return
	}
	if err := removeCharacterItem(c.Char, item, amount); err != nil {
		log.Printf("[Items] remove item failed char=%q inv=%d slot=%d: %v", c.Char.Name, inv, slot, err)
		sendItemRemoved(c, itemStatusFail, item, amount)
		return
	}
	if types.IsEquipmentInventory(inv) {
		broadcastEquipmentChange(c, item, false)
	}
	sendItemRemoved(c, itemStatusOK, item, amount)
}

func handleSellItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	sold := make([]*ItemRow, 0, 5)
	status := itemStatusOK
	requests := readSellRequests(c.Char, p.Data)
	for _, req := range requests {
		item := findItem(c.Char, req.inv, req.slot)
		if item == nil || item.ItemID == int(goldLootItemID) {
			continue
		}
		if itemLockedForTrading(c.Char, item) {
			status = itemStatusFail
			break
		}
		amount := req.amount
		if amount <= 0 || amount > item.Amount {
			amount = item.Amount
		}
		if amount <= 0 {
			continue
		}
		if !effectiveItemStackable(item) {
			amount = 1
		}
		templ := itemTemplateByID(item.ItemID)
		price := templ.SellPrice * int64(amount)
		if price < 0 {
			price = 0
		}
		if err := removeCharacterItem(c.Char, item, amount); err != nil {
			log.Printf("[Items] sell item failed char=%q item=%d: %v", c.Char.Name, item.ItemID, err)
			status = itemStatusFail
			break
		}
		c.Char.Gold += price
		sold = append(sold, item)
	}
	_ = SaveCharacter(c.Char)
	if status != itemStatusOK {
		logSellRejected(c, status, "sell-failed", p.Data)
	}
	sendSellItemResponse(c, status, sold)
}

func handleBuyItem(c *Client, p *PacketIn) {
	handleBuyItemWithVendor(c, p, true)
}

func handleBuyItemWithVendor(c *Client, p *PacketIn, requireVendor bool) {
	if c.Char == nil {
		return
	}
	requests := readPurchaseRequests(p.Data)
	bought := make([]*ItemRow, 0, len(requests))
	if requireVendor {
		if status := validateVendorPurchase(c, requests); status != buyStatusOK {
			sendBuyItemResponse(c, status, bought)
			return
		}
	}
	status := validatePurchase(c.Char, requests)
	if status != buyStatusOK {
		logBuyRejected(c, status, requests, p.Data)
		sendBuyItemResponse(c, status, bought)
		return
	}
	for _, req := range requests {
		itemID := req.itemID
		amount := req.amount
		if itemID <= 0 {
			continue
		}
		templ := itemTemplateByID(itemID)
		targetInv := targetInventoryForTemplate(templ)
		amount = normalizePurchaseAmount(templ, amount)
		price := templ.BuyPrice * int64(amount)
		if !templ.IsStackable && amount > 1 {
			for i := 0; i < amount; i++ {
				item, addStatus, err := createCharacterItemDetailed(c.Char, itemID, 1, targetInv, -1, nil, 0)
				if err != nil || addStatus != inventoryStatusOK || item == nil {
					log.Printf("[Items] buy item failed char=%q item=%d status=%d err=%v", c.Char.Name, itemID, addStatus, err)
					sendBuyItemResponse(c, buyStatusFail, bought)
					return
				}
				if i == amount-1 {
					bought = append(bought, item)
				}
			}
		} else {
			item, addStatus, err := createCharacterItemDetailed(c.Char, itemID, amount, targetInv, -1, nil, 0)
			if err != nil || addStatus != inventoryStatusOK || item == nil {
				log.Printf("[Items] buy item failed char=%q item=%d status=%d err=%v", c.Char.Name, itemID, addStatus, err)
				sendBuyItemResponse(c, buyStatusFail, bought)
				return
			}
			bought = append(bought, item)
		}
		c.Char.Gold -= price
	}
	_ = SaveCharacter(c.Char)
	sendBuyItemResponse(c, status, bought)
}

func readPurchaseRequests(data []byte) []itemPurchaseRequest {
	if len(data) <= 42 {
		requests, _ := readPurchaseRequestsAt(data, 0)
		return requests
	}
	bestRequests := []itemPurchaseRequest{}
	bestScore := -1 << 30
	maxStart := len(data)
	if maxStart > 36 {
		maxStart = 36
	}
	for start := 0; start <= maxStart; start++ {
		requests, score := readPurchaseRequestsAt(data, start)
		if len(requests) == 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestRequests = requests
		}
	}
	return bestRequests
}

func readPurchaseRequestsAt(data []byte, start int) ([]itemPurchaseRequest, int) {
	requests := make([]itemPurchaseRequest, 0, 7)
	score := 0
	for offset := start; offset+5 < len(data) && len(requests) < 7; offset += 6 {
		itemID := int(binary.LittleEndian.Uint16(data[offset:]))
		// WCell Asda2InventoryHandler.BuyItemRequest reads:
		// ushort itemId, ignored int16, signed short amount.
		middle := int(int16(binary.LittleEndian.Uint16(data[offset+2:])))
		amount := int(int16(binary.LittleEndian.Uint16(data[offset+4:])))
		if itemID <= 0 {
			continue
		}
		templ := itemTemplateByID(itemID)
		rawSane := purchaseAmountIsSane(templ, amount)
		middleSane := purchaseAmountIsSane(templ, middle)
		if preferMiddlePurchaseAmount(templ, amount, middle) {
			amount = middle
		}
		score += purchaseRequestScore(templ, rawSane, middleSane, amount)
		requests = append(requests, itemPurchaseRequest{itemID: itemID, amount: amount})
	}
	return requests, score
}

func validatePurchase(chr *Character, requests []itemPurchaseRequest) byte {
	if chr == nil {
		return buyStatusFail
	}
	requiredSlots := map[byte]int{}
	addedWeight := 0
	totalPrice := int64(0)
	for _, req := range requests {
		if req.itemID <= 0 {
			continue
		}
		templ := itemTemplateByID(req.itemID)
		if templ.ItemID <= 0 {
			return buyStatusBadItemID
		}
		amount := normalizePurchaseAmount(templ, req.amount)
		if !templ.IsStackable {
			requiredSlots[targetInventoryForTemplate(templ)] += amount
		} else if findItemByID(chr, targetInventoryForTemplate(templ), req.itemID) == nil {
			requiredSlots[targetInventoryForTemplate(templ)]++
		}
		if isCarriedInventory(targetInventoryForTemplate(templ)) {
			addedWeight += itemTemplateUnitWeight(templ, nil) * amount
		}
		price := templ.BuyPrice * int64(amount)
		if price < 0 {
			return buyStatusNotEnoughGold
		}
		totalPrice += price
		if totalPrice < 0 || totalPrice > 2147483647 {
			return buyStatusNotEnoughGold
		}
	}
	for inv, count := range requiredSlots {
		if freeInventorySlotCount(chr, inv) < count {
			return buyStatusNotEnoughSpace
		}
	}
	if carriedWeight(chr)+addedWeight > maxWeight(chr) {
		return buyStatusWeightExceeds
	}
	if totalPrice > chr.Gold {
		return buyStatusNotEnoughGold
	}
	return buyStatusOK
}

func normalizePurchaseAmount(templ ItemTemplate, amount int) int {
	if amount <= 0 {
		return 1
	}
	if !purchaseAmountIsSane(templ, amount) {
		return 1
	}
	if !templ.IsStackable && amount != 1 {
		return 1
	}
	return amount
}

func purchaseAmountIsSane(templ ItemTemplate, amount int) bool {
	return amount > 0 && amount <= maxSanePurchaseAmount(templ)
}

func preferMiddlePurchaseAmount(templ ItemTemplate, referenceAmount int, middleAmount int) bool {
	if !purchaseAmountIsSane(templ, middleAmount) {
		return false
	}
	return !purchaseAmountIsSane(templ, referenceAmount)
}

func maxSanePurchaseAmount(templ ItemTemplate) int {
	if !templ.IsStackable {
		return 1
	}
	max := templ.MaxStack
	if max <= 0 {
		max = 1
	}
	if max > 999 {
		max = 999
	}
	return max
}

func purchaseRequestScore(templ ItemTemplate, rawSane bool, middleSane bool, chosenAmount int) int {
	score := 6
	if templ.ItemID > 0 {
		score += 2
	}
	if templ.BuyPrice > 0 {
		score++
	}
	if rawSane {
		score += 12
	} else {
		score -= 30
	}
	if middleSane {
		score += 3
	}
	if chosenAmount == 1 {
		score += 2
	}
	return score
}

func readSellRequests(chr *Character, data []byte) []itemSellRequest {
	bestRequests := []itemSellRequest{}
	bestScore := -1
	maxStart := len(data)
	if maxStart > 36 {
		maxStart = 36
	}
	for start := 0; start <= maxStart; start++ {
		requests, score := readSellRequestsAt(chr, data, start)
		if score > bestScore {
			bestScore = score
			bestRequests = requests
		}
	}
	return bestRequests
}

func readSellRequestsAt(chr *Character, data []byte, start int) ([]itemSellRequest, int) {
	requests := make([]itemSellRequest, 0, 5)
	score := 0
	for offset := start; offset+10 < len(data) && len(requests) < 5; offset += 11 {
		slot := int16(binary.LittleEndian.Uint16(data[offset:]))
		inv := data[offset+4]
		amount := int(int32(binary.LittleEndian.Uint32(data[offset+5:])))
		if slot < 0 || (inv != types.InventoryRegular && inv != types.InventoryShop) {
			continue
		}
		item := findItem(chr, inv, slot)
		if item == nil || item.ItemID == int(goldLootItemID) {
			continue
		}
		score += 10
		if amount <= 0 || amount > item.Amount {
			amount = item.Amount
		}
		if amount > 0 {
			score += 2
		}
		requests = append(requests, itemSellRequest{slot: slot, inv: inv, amount: amount})
	}
	return requests, score
}

func logBuyRejected(c *Client, status byte, requests []itemPurchaseRequest, payload []byte) {
	if c == nil || c.Char == nil {
		return
	}
	totalPrice, priceOK := purchaseTotalPrice(requests)
	if priceOK {
		log.Printf("[Items] buy rejected char=%q status=%d gold=%d totalPrice=%d requests=%v payload=% X",
			c.Char.Name, status, c.Char.Gold, totalPrice, requests, limitedPacketBytes(payload))
		return
	}
	log.Printf("[Items] buy rejected char=%q status=%d gold=%d totalPrice=overflow requests=%v payload=% X",
		c.Char.Name, status, c.Char.Gold, requests, limitedPacketBytes(payload))
}

func logSellRejected(c *Client, status byte, reason string, payload []byte) {
	if c == nil || c.Char == nil {
		return
	}
	log.Printf("[Items] sell rejected char=%q status=%d reason=%s payload=% X",
		c.Char.Name, status, reason, limitedPacketBytes(payload))
}

func limitedPacketBytes(data []byte) []byte {
	if len(data) > 64 {
		return data[:64]
	}
	return data
}

func purchaseTotalPrice(requests []itemPurchaseRequest) (int64, bool) {
	totalPrice := int64(0)
	for _, req := range requests {
		if req.itemID <= 0 {
			continue
		}
		templ := itemTemplateByID(req.itemID)
		amount := normalizePurchaseAmount(templ, req.amount)
		price := templ.BuyPrice * int64(amount)
		totalPrice += price
		if price < 0 || totalPrice < 0 || totalPrice > 2147483647 {
			return totalPrice, false
		}
	}
	return totalPrice, true
}

func validateVendorPurchase(c *Client, requests []itemPurchaseRequest) byte {
	npc, ok := activeVendorNpc(c)
	if !ok {
		if c != nil && c.Char != nil {
			debugNpcInteractionf("rejected vendor buy char=%q reason=no-active-vendor", c.Char.Name)
		}
		return buyStatusFail
	}
	if len(requests) == 0 {
		return buyStatusBadItemID
	}
	// WCell Asda2PlayerInventory.BuyItems checks RegularShopRecord-backed
	// CanBuyInRegularShop before money, slots, or weight are applied.
	for _, req := range requests {
		if req.itemID <= 0 {
			continue
		}
		if !vendorCanSellItem(npc.EntryID, req.itemID) {
			if c != nil && c.Char != nil {
				debugNpcInteractionf("rejected vendor buy char=%q vendor=%d item=%d reason=stock",
					c.Char.Name, npc.EntryID, req.itemID)
			}
			return buyStatusBadItemID
		}
	}
	return buyStatusOK
}

func validateVendorSell(c *Client) byte {
	if _, ok := activeVendorNpc(c); !ok {
		if c != nil && c.Char != nil {
			debugNpcInteractionf("rejected vendor sell char=%q reason=no-active-vendor", c.Char.Name)
		}
		return itemStatusFail
	}
	return itemStatusOK
}

func handleUseItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendCharUsedItem(c, itemStatusFail, nil)
		return
	}
	req, ok := readUseItemRequest(c.Char, p.Data)
	if !ok {
		log.Printf("[Items] use rejected char=%q reason=no-valid-payload len=%d payload=% X", c.Char.Name, len(p.Data), p.Data)
		sendCharUsedItem(c, itemStatusFail, nil)
		return
	}
	inv := req.inv
	slot := req.slot
	if inv != types.InventoryRegular && inv != types.InventoryShop {
		sendCharUsedItem(c, itemStatusFail, nil)
		return
	}
	item := findItem(c.Char, inv, slot)
	if item == nil {
		sendCharUsedItem(c, itemStatusFail, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendCharUsedItem(c, itemStatusFail, item)
		return
	}
	out := applyInventoryItemUse(c.Char, item)
	sendCharUsedItem(c, out.status, item)
	if out.status == useItemStatusOK {
		if out.added != nil {
			sendSingleInventoryUpdate(c, out.added)
		}
		if out.healthChanged {
			sendCharacterHealthUpdate(c)
		}
		if out.powerChanged {
			sendCharacterMPUpdate(c)
		}
		if out.warehouseExpanded {
			sendWarehouseSlotsExpanded(c, false)
		}
		if out.skillsChanged {
			sendLearnedSkillsInfo(c)
		}
		if out.functionalUsed {
			sendFunctionalShopItemUsed(c, item, 0)
		}
		if out.consumed {
			sendInventoryWeightUpdate(c, item)
		}
	}
}

func handleMoveSoulStoneOut(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 3 {
		sendSowelRemoved(c, itemStatusFail, nil)
		return
	}
	slot := int16(binary.LittleEndian.Uint16(p.Data[0:]))
	socket := int(p.Data[2])
	item := findItem(c.Char, types.InventoryEquipment, slot)
	if item == nil {
		item = findItem(c.Char, types.InventoryRegular, slot)
	}
	if item == nil || socket < 0 || socket > 3 {
		sendSowelRemoved(c, itemStatusFail, item)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendSowelRemoved(c, itemStatusFail, item)
		return
	}
	if templ := itemTemplateByID(item.ItemID); templ.SowelSockets > 0 && socket >= int(templ.SowelSockets) {
		sendSowelRemoved(c, itemStatusFail, item)
		return
	}
	removedID := 0
	switch socket {
	case 0:
		removedID, item.Soul1ID = item.Soul1ID, 0
	case 1:
		removedID, item.Soul2ID = item.Soul2ID, 0
	case 2:
		removedID, item.Soul3ID = item.Soul3ID, 0
	case 3:
		removedID, item.Soul4ID = item.Soul4ID, 0
	}
	if removedID <= 0 {
		sendSowelRemoved(c, itemStatusFail, item)
		return
	}
	_, err := createCharacterItem(c.Char, removedID, 1, types.InventoryRegular, -1)
	if err != nil || SaveItem(item) != nil {
		sendSowelRemoved(c, itemStatusFail, item)
		return
	}
	sendSowelRemoved(c, itemStatusOK, item)
}

func handleMoveSoulStoneIn(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 5 {
		sendItemSoweled(c, itemStatusFail, nil, nil)
		return
	}
	itemSlot := int16(binary.LittleEndian.Uint16(p.Data[0:]))
	sowelSlot := int16(binary.LittleEndian.Uint16(p.Data[2:]))
	socket := int(p.Data[4])
	item := findItem(c.Char, types.InventoryEquipment, itemSlot)
	if item == nil {
		item = findItem(c.Char, types.InventoryRegular, itemSlot)
	}
	sowel := findItem(c.Char, types.InventoryRegular, sowelSlot)
	if item == nil || sowel == nil || socket < 0 || socket > 3 {
		sendItemSoweled(c, itemStatusFail, item, sowel)
		return
	}
	if itemLockedForTrading(c.Char, item) || itemLockedForTrading(c.Char, sowel) {
		sendItemSoweled(c, itemStatusFail, item, sowel)
		return
	}
	itemTempl := itemTemplateByID(item.ItemID)
	sowelTempl := itemTemplateByID(sowel.ItemID)
	if sowelTempl.Kind != types.ItemKindUnknown && sowelTempl.Kind != types.ItemKindSowel {
		sendItemSoweled(c, itemStatusFail, item, sowel)
		return
	}
	if itemTempl.SowelSockets > 0 && socket >= int(itemTempl.SowelSockets) {
		sendItemSoweled(c, itemStatusFail, item, sowel)
		return
	}
	switch socket {
	case 0:
		if item.Soul1ID != 0 {
			sendItemSoweled(c, itemStatusFail, item, sowel)
			return
		}
		item.Soul1ID = sowel.ItemID
	case 1:
		if item.Soul2ID != 0 {
			sendItemSoweled(c, itemStatusFail, item, sowel)
			return
		}
		item.Soul2ID = sowel.ItemID
	case 2:
		if item.Soul3ID != 0 {
			sendItemSoweled(c, itemStatusFail, item, sowel)
			return
		}
		item.Soul3ID = sowel.ItemID
	case 3:
		if item.Soul4ID != 0 {
			sendItemSoweled(c, itemStatusFail, item, sowel)
			return
		}
		item.Soul4ID = sowel.ItemID
	}
	if err := removeCharacterItem(c.Char, sowel, 1); err != nil || SaveItem(item) != nil {
		sendItemSoweled(c, itemStatusFail, item, sowel)
		return
	}
	sendItemSoweled(c, itemStatusOK, item, sowel)
}
func handleSetFastItemSlot(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 1 {
		return
	}
	panel := p.ReadUint8()
	if panel > 5 {
		return
	}

	slots := make([]*FastSlotRow, 0, 12)
	for panelSlot := byte(0); panelSlot < 12; panelSlot++ {
		if p.Remaining() < 10 {
			return
		}
		srcInfo := p.ReadUint8()
		invType := p.ReadUint8()
		invSlot := p.ReadInt16()
		amount := p.ReadInt32()
		itemOrSkillID := p.ReadInt16()

		if srcInfo == 0 && invType == 0 && invSlot == -1 && amount == 0 && itemOrSkillID == -1 {
			continue
		}
		slots = append(slots, &FastSlotRow{
			OwnerID:       c.Char.GUID,
			PanelNum:      panel,
			PanelSlot:     panelSlot,
			InventoryType: invType,
			ItemOrSkillID: int(itemOrSkillID),
			InventorySlot: invSlot,
			SrcInfo:       int16(srcInfo),
			Amount:        int(amount),
		})
	}

	if err := ReplaceFastSlotsForOwnerPanel(c.Char.GUID, panel, slots); err != nil {
		log.Printf("[FastSlot] failed to save panel %d for %q: %v", panel, c.Char.Name, err)
		return
	}

	kept := c.Char.FastSlots[:0]
	for _, slot := range c.Char.FastSlots {
		if slot.PanelNum != panel {
			kept = append(kept, slot)
		}
	}
	c.Char.FastSlots = append(kept, slots...)
}

func handleCombineItem(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 8 {
		sendItemRemoved(c, itemStatusFail, nil, 0)
		return
	}
	slotA := int16(binary.LittleEndian.Uint16(p.Data[0:]))
	slotB := int16(binary.LittleEndian.Uint16(p.Data[4:]))
	a := findItem(c.Char, types.InventoryRegular, slotA)
	b := findItem(c.Char, types.InventoryRegular, slotB)
	if a == nil || b == nil || a.ItemID != b.ItemID || !effectiveItemStackable(a) {
		sendItemRemoved(c, itemStatusFail, a, 0)
		return
	}
	if itemLockedForTrading(c.Char, a) || itemLockedForTrading(c.Char, b) {
		sendItemRemoved(c, itemStatusFail, a, 0)
		return
	}
	a.Amount += b.Amount
	if err := DeleteItem(b.Guid); err != nil || SaveItem(a) != nil {
		sendItemRemoved(c, itemStatusFail, a, 0)
		return
	}
	for i, it := range c.Char.Items {
		if it == b || it.Guid == b.Guid {
			c.Char.Items = append(c.Char.Items[:i], c.Char.Items[i+1:]...)
			break
		}
	}
	sendSingleInventoryUpdate(c, a)
	sendItemRemoved(c, itemStatusOK, b, b.Amount)
}

func handleDisassembleAvatar(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 2 {
		sendEquipmentDisasembled(c, itemStatusFail, nil)
		return
	}
	slot := int16(binary.LittleEndian.Uint16(p.Data))
	item := findItem(c.Char, types.InventoryRegular, slot)
	if item == nil {
		item = findItem(c.Char, types.InventoryEquipment, slot)
	}
	if item == nil {
		sendEquipmentDisasembled(c, itemStatusFail, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendEquipmentDisasembled(c, itemStatusFail, item)
		return
	}
	materialID := item.ItemID
	if materialID > 1000 {
		materialID = 30000 + item.ItemID%1000
	}
	if err := removeCharacterItem(c.Char, item, 1); err != nil {
		sendEquipmentDisasembled(c, itemStatusFail, item)
		return
	}
	_, _ = createCharacterItem(c.Char, materialID, 1, types.InventoryRegular, -1)
	sendEquipmentDisasembled(c, itemStatusOK, item)
}

func handleStartAvatarSynthesis(c *Client, p *PacketIn) {
	sendCrafted(c, false, 0, nil, nil)
}

func handleSoketAvatar(c *Client, p *PacketIn) {
	handleMoveSoulStoneIn(c, p)
}

func handleOpenBooster(c *Client, p *PacketIn) {
	openContainerItem(c, p, BosterOpened)
}

func handleOpenPackage(c *Client, p *PacketIn) {
	openContainerItem(c, p, OpenPackageResponse)
}

// Some native clients prepend variable request bytes before the WCell item
// layout. Score against the live inventory so valid slots survive that prefix.
func readMoveItemRequest(chr *Character, data []byte) (itemMoveRequest, bool) {
	best := itemMoveRequest{}
	bestScore := -1
	for offset := 0; offset+20 <= len(data); offset++ {
		req := readMoveItemRequestAt(data[offset:])
		score := scoreMoveItemRequest(chr, req)
		if score > bestScore {
			best = req
			bestScore = score
		}
	}
	if bestScore < 100 {
		return itemMoveRequest{}, false
	}
	return best, true
}

func readMoveItemRequestAt(data []byte) itemMoveRequest {
	req := itemMoveRequest{
		srcSlot:    int16(binary.LittleEndian.Uint16(data[0:])),
		srcInv:     data[4],
		srcAmount:  int32(binary.LittleEndian.Uint32(data[5:])),
		srcWeight:  int16(binary.LittleEndian.Uint16(data[9:])),
		destSlot:   int16(binary.LittleEndian.Uint16(data[11:])),
		destInv:    data[15],
		destAmount: int32(binary.LittleEndian.Uint32(data[16:])),
	}
	if req.srcInv == 0 {
		req.srcInv = types.InventoryEquipment
	}
	if req.destInv == 0 {
		req.destInv = types.InventoryEquipment
	}
	if len(data) >= 22 {
		req.destWeight = int16(binary.LittleEndian.Uint16(data[20:]))
	}
	return req
}

func scoreMoveItemRequest(chr *Character, req itemMoveRequest) int {
	score := 0
	if req.srcInv == types.InventoryShop || req.srcInv == types.InventoryRegular || req.srcInv == types.InventoryEquipment {
		score += 10
	}
	if req.destInv == types.InventoryShop || req.destInv == types.InventoryRegular || req.destInv == types.InventoryEquipment {
		score += 10
	}
	if req.srcSlot >= 0 && req.srcSlot < 120 {
		score += 5
	}
	if req.destSlot >= 0 && req.destSlot < 120 {
		score += 5
	}
	source := findItem(chr, req.srcInv, req.srcSlot)
	if source != nil && source.ItemID > 0 {
		score += 100
		if req.srcAmount == itemRequestAmount(source) {
			score += 10
		}
	}
	if req.srcAmount >= 0 && req.srcAmount <= 1_000_000 {
		score += 2
	}
	if req.destAmount >= 0 && req.destAmount <= 1_000_000 {
		score += 2
	}
	if req.srcWeight >= -1 && req.srcWeight <= 30_000 {
		score++
	}
	if req.destWeight >= -1 && req.destWeight <= 30_000 {
		score++
	}
	return score
}

func readUseItemRequest(chr *Character, data []byte) (itemUseRequest, bool) {
	best := itemUseRequest{}
	bestScore := -1
	for offset := 0; offset+5 <= len(data); offset++ {
		inv := data[offset]
		if inv != types.InventoryRegular && inv != types.InventoryShop {
			continue
		}
		slot32 := int32(binary.LittleEndian.Uint32(data[offset+1:]))
		if slot32 < 0 || slot32 > 32767 {
			continue
		}
		req := itemUseRequest{inv: inv, slot: int16(slot32)}
		score := scoreUseItemRequest(chr, req)
		if score > bestScore {
			best = req
			bestScore = score
		}
	}
	if bestScore < 100 {
		return itemUseRequest{}, false
	}
	return best, true
}

func scoreUseItemRequest(chr *Character, req itemUseRequest) int {
	if req.slot < 0 || req.slot >= 120 {
		return -1
	}
	item := findItem(chr, req.inv, req.slot)
	if item == nil || item.ItemID == int(goldLootItemID) {
		return -1
	}
	score := 100
	if effectiveItemStackable(item) {
		score += 10
	}
	return score
}

func readRemoveItemRequest(chr *Character, data []byte) (itemRemoveRequest, bool) {
	best := itemRemoveRequest{}
	bestScore := -1
	for offset := 0; offset+5 <= len(data); offset++ {
		inv := data[offset]
		if inv != types.InventoryRegular && inv != types.InventoryShop {
			continue
		}
		slot := int16(binary.LittleEndian.Uint16(data[offset+1:]))
		amount := int(int16(binary.LittleEndian.Uint16(data[offset+3:])))
		req := itemRemoveRequest{inv: inv, slot: slot, amount: amount}
		score := scoreRemoveItemRequest(chr, req)
		if score > bestScore {
			best = req
			bestScore = score
		}
	}
	if bestScore < 100 {
		return itemRemoveRequest{}, false
	}
	return best, true
}

func scoreRemoveItemRequest(chr *Character, req itemRemoveRequest) int {
	if req.slot < 0 || req.slot >= 120 || req.amount < 0 {
		return -1
	}
	if req.inv == types.InventoryRegular && req.slot == types.ReservedRegularInventorySlot {
		return -1
	}
	item := findItem(chr, req.inv, req.slot)
	if item == nil || item.ItemID == int(goldLootItemID) {
		return -1
	}
	score := 100
	if req.amount == 0 || req.amount <= item.Amount {
		score += 10
	}
	if effectiveItemStackable(item) {
		score += 5
	}
	return score
}

func itemRequestAmount(item *ItemRow) int32 {
	if item == nil {
		return -1
	}
	if effectiveItemStackable(item) {
		return int32(item.Amount)
	}
	return 0
}

func sendItemReplaced(c *Client, status byte, srcSlot int16, srcInv byte, srcAmount int32, srcWeight int16, destSlot int16, destInv byte, destAmount int32, destWeight int16, secondItemIsNull bool) {
	if c == nil {
		return
	}
	p := NewPacket(ItemReplaced)
	p.WriteUint8(status)
	if secondItemIsNull {
		p.WriteUint8(0)
	} else {
		p.WriteUint8(1)
	}
	p.WriteInt16(srcSlot)
	if secondItemIsNull {
		p.WriteInt16(-1)
	} else {
		p.WriteInt16(0)
	}
	p.WriteUint8(srcInv)
	p.WriteInt32(srcAmount)
	p.WriteInt16(srcWeight)
	p.WriteInt16(destSlot)
	p.WriteInt16(0)
	p.WriteUint8(destInv)
	p.WriteInt32(destAmount)
	p.WriteInt16(destWeight)
	c.Send(p)
}

func sendItemRemoved(c *Client, status byte, item *ItemRow, amount int) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ItemRemovedFromInventory)
	p.WriteUint8(status)
	p.WriteInt32(clampInt32(c.Char.Gold))
	p.WriteInt16(itemWeight(c.Char))
	p.WriteInt16(int16(amount))
	p.WriteInt16(0)
	writeItemInfoToPacket(p, item, c.Char, true)
	c.Send(p)
}

func sendSellItemResponse(c *Client, status byte, items []*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SellItemResponse)
	p.WriteUint8(status)
	for i := 0; i < 5; i++ {
		var item *ItemRow
		if i < len(items) {
			item = items[i]
		}
		if item == nil {
			p.WriteInt32(0)
			p.WriteUint8(0)
			p.WriteInt32(0)
			p.WriteInt16(0)
			continue
		}
		p.WriteInt32(int32(item.Slot))
		p.WriteUint8(item.InventoryType)
		p.WriteInt32(int32(item.Amount))
		p.WriteInt16(int16(item.Weight))
	}
	p.WriteInt32(clampInt32(c.Char.Gold))
	p.WriteInt16(itemWeight(c.Char))
	c.Send(p)
}

func sendBuyItemResponse(c *Client, status byte, items []*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(BuyItemResponse)
	p.WriteUint8(status)
	for i := 0; i < 7; i++ {
		var item *ItemRow
		if i < len(items) {
			item = items[i]
		}
		writeItemInfoToPacket(p, item, c.Char, false)
	}
	p.WriteInt32(clampInt32(c.Char.Gold))
	p.WriteInt16(itemWeight(c.Char))
	c.Send(p)
}

func sendCharUsedItem(c *Client, status byte, item *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(CharUsedItem)
	p.WriteUint8(status)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt32(int32(c.Char.AccID))
	writeItemInfoToPacket(p, item, c.Char, true)
	c.Send(p)
}

func sendSowelRemoved(c *Client, status byte, item *ItemRow) {
	p := NewPacket(SowelRemoved)
	p.WriteUint8(status)
	writeItemInfoToPacket(p, item, c.Char, false)
	c.Send(p)
}

func sendItemSoweled(c *Client, status byte, item *ItemRow, sowel *ItemRow) {
	p := NewPacket(ItemSoweled)
	p.WriteUint8(status)
	writeItemInfoToPacket(p, item, c.Char, false)
	writeItemInfoToPacket(p, sowel, c.Char, true)
	c.Send(p)
}

func sendEquipmentDisasembled(c *Client, status byte, item *ItemRow) {
	p := NewPacket(EquipmentDisasembled)
	p.WriteUint8(status)
	writeItemInfoToPacket(p, item, c.Char, true)
	c.Send(p)
}

func openContainerItem(c *Client, in *PacketIn, op Opcode) {
	if c.Char == nil {
		p := NewPacket(op)
		p.WriteUint8(itemStatusFail)
		c.Send(p)
		return
	}
	inv, slot, ok := readContainerItemLocation(in.Data)
	if !ok {
		p := NewPacket(op)
		p.WriteUint8(itemStatusFail)
		c.Send(p)
		return
	}
	container := findItem(c.Char, inv, slot)
	if container == nil {
		p := NewPacket(op)
		p.WriteUint8(itemStatusFail)
		c.Send(p)
		return
	}
	if itemLockedForTrading(c.Char, container) {
		p := NewPacket(op)
		p.WriteUint8(itemStatusFail)
		c.Send(p)
		return
	}
	resultID := itemTemplateByID(container.ItemID).ValueOnUse
	if resultID <= 0 {
		resultID = container.ItemID + 1
	}
	result, err := createCharacterItem(c.Char, resultID, 1, types.InventoryRegular, -1)
	if err != nil || result == nil {
		p := NewPacket(op)
		p.WriteUint8(itemStatusFail)
		c.Send(p)
		return
	}
	_ = removeCharacterItem(c.Char, container, 1)
	p := NewPacket(op)
	p.WriteUint8(itemStatusOK)
	writeItemInfoToPacket(p, result, c.Char, false)
	writeItemInfoToPacket(p, container, c.Char, true)
	c.Send(p)
}

func readContainerItemLocation(data []byte) (byte, int16, bool) {
	if len(data) >= 7 {
		inv := data[4]
		slot := int16(binary.LittleEndian.Uint16(data[5:]))
		if (inv == types.InventoryRegular || inv == types.InventoryShop) && slot >= 0 {
			return inv, slot, true
		}
	}
	if len(data) >= 6 {
		inv := data[3]
		slot := int16(binary.LittleEndian.Uint16(data[4:]))
		if (inv == types.InventoryRegular || inv == types.InventoryShop) && slot >= 0 {
			return inv, slot, true
		}
	}
	if len(data) >= 2 {
		return types.InventoryRegular, int16(binary.LittleEndian.Uint16(data)), true
	}
	return 0, -1, false
}
