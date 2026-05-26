package main

import (
	"encoding/binary"
	"log"
	"math"
	"math/rand"

	"asda2/shared/types"
)

const (
	upgradeStatusOK   byte = 1
	upgradeStatusFail byte = 2
)

const (
	advancedEnchantStatusFail          byte = 0
	advancedEnchantStatusOK            byte = 1
	advancedEnchantStatusNotEnoughGold byte = 2
	advancedEnchantStatusNoMaterials   byte = 5
)

const (
	exchangeOptionStatusParamError    byte = 0
	exchangeOptionStatusOK            byte = 1
	exchangeOptionStatusScrollInvalid byte = 2
	exchangeOptionStatusItemInvalid   byte = 3
)

const (
	guaranteedUpgradeStoneID = 31868
	upgradeResetGoldCost     = int64(1_000_000)
)

var (
	upgradeRandFloat                = rand.Float64
	upgradeResetScrollCostByEnchant = map[byte]int{
		10: 8,
		11: 7,
		12: 7,
		13: 6,
		14: 6,
		15: 5,
		16: 5,
		17: 4,
		18: 4,
		19: 3,
		20: 3,
	}
)

type upgradeRequest struct {
	itemSlot        int16
	stoneSlot       int16
	chanceBoostSlot int16
	protectSlot     int16
}

type itemUpgradeResultStatus byte

const (
	itemUpgradeSuccess itemUpgradeResultStatus = iota
	itemUpgradeFail
	itemUpgradeReduceToZero
	itemUpgradeReduceOne
	itemUpgradeBreak
)

type itemUpgradeResult struct {
	status itemUpgradeResultStatus
	chance float64
}

func handleUpgradeItemRequest(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	req, ok := readUpgradeRequest(p.Data)
	if !ok {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	item := findUpgradeableItem(c.Char, req.itemSlot)
	stone := findItem(c.Char, types.InventoryRegular, req.stoneSlot)
	if stone == nil {
		stone = findItem(c.Char, types.InventoryShop, req.stoneSlot)
	}
	if item == nil || stone == nil || !canUseUpgradeStone(item, stone) {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) || itemLockedForTrading(c.Char, stone) {
		sendUpgradeItemResponse(c, upgradeStatusFail, item, stone, nil, nil)
		return
	}
	chanceBoost := optionalShopOrRegularItem(c.Char, req.chanceBoostSlot)
	protect := optionalShopOrRegularItem(c.Char, req.protectSlot)
	if req.chanceBoostSlot >= 0 && !isUpgradeChanceBoostItem(chanceBoost) {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	if req.protectSlot >= 0 && !isUpgradeProtectionItem(protect) {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	if itemLockedForTrading(c.Char, chanceBoost) || itemLockedForTrading(c.Char, protect) {
		sendUpgradeItemResponse(c, upgradeStatusFail, item, stone, chanceBoost, protect)
		return
	}
	price := enchantPrice(item.Enchant, itemTemplateByID(item.ItemID))
	if price > c.Char.Gold {
		sendUpgradeItemResponse(c, upgradeStatusFail, nil, nil, nil, nil)
		return
	}
	useProtect, noEnchantLose := upgradeProtectionFlags(item, protect)
	chanceBoostValue := 0
	if chanceBoost != nil {
		chanceBoostValue = itemTemplateByID(chanceBoost.ItemID).ValueOnUse
	}
	forceSuccess := stone.ItemID == guaranteedUpgradeStoneID
	result := calculateItemUpgradeResult(
		itemTemplateByID(stone.ItemID).Quality,
		itemTemplateByID(item.ItemID).Quality,
		item.Enchant,
		itemTemplateByID(item.ItemID).RequiredLevel,
		int(c.Char.BaseLuck),
		useProtect,
		chanceBoostValue,
		noEnchantLose,
		forceSuccess,
	)
	c.Char.Gold -= price
	if err := removeCharacterItem(c.Char, stone, 1); err != nil {
		log.Printf("[Upgrade] remove stone failed char=%q item=%d: %v", c.Char.Name, stone.ItemID, err)
		sendUpgradeItemResponse(c, upgradeStatusFail, item, stone, chanceBoost, protect)
		return
	}
	if chanceBoost != nil {
		_ = removeCharacterItem(c.Char, chanceBoost, 1)
	}
	if protect != nil {
		_ = removeCharacterItem(c.Char, protect, 1)
	}
	responseStatus := upgradeStatusFail
	switch result.status {
	case itemUpgradeSuccess:
		item.Enchant++
		generateUpgradeEquipmentOption(item)
		responseStatus = upgradeStatusOK
	case itemUpgradeReduceToZero:
		item.Enchant = 0
	case itemUpgradeReduceOne:
		if item.Enchant > 0 {
			item.Enchant--
		}
	case itemUpgradeBreak:
		_ = removeCharacterItem(c.Char, item, 1)
	default:
	}
	if result.status != itemUpgradeBreak {
		_ = SaveItem(item)
	}
	_ = SaveCharacter(c.Char)
	sendUpgradeItemResponse(c, responseStatus, item, stone, chanceBoost, protect)
}

func handleUpgradeResetRequest(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 9 {
		sendUpgradeResetResponse(c, 0, nil, nil)
		return
	}
	itemSlot := int16(binary.LittleEndian.Uint16(p.Data[5:]))
	scrollID := int(binary.LittleEndian.Uint16(p.Data[7:]))
	item := findUpgradeableItem(c.Char, itemSlot)
	scroll := findItemByID(c.Char, types.InventoryShop, scrollID)
	if scroll == nil {
		scroll = findItemByID(c.Char, types.InventoryRegular, scrollID)
	}
	required, ok := upgradeResetScrollCostByEnchant[itemEnchant(item)]
	if item == nil || scroll == nil || !isUpgradeResetScroll(scroll) || !ok || scroll.Amount < required {
		sendUpgradeResetResponse(c, 0, item, scroll)
		return
	}
	if itemLockedForTrading(c.Char, item) || itemLockedForTrading(c.Char, scroll) {
		sendUpgradeResetResponse(c, 0, item, scroll)
		return
	}
	if c.Char.Gold < upgradeResetGoldCost {
		sendUpgradeResetResponse(c, 0, item, scroll)
		return
	}
	c.Char.Gold -= upgradeResetGoldCost
	if err := removeCharacterItem(c.Char, scroll, required); err != nil {
		sendUpgradeResetResponse(c, 0, item, scroll)
		return
	}
	if item.Enchant > 13 {
		item.Enchant -= 13
	} else {
		item.Enchant = 0
	}
	item.EnchantResetCount++
	if err := SaveItem(item); err != nil {
		log.Printf("[UpgradeReset] save item failed char=%q item=%d: %v", c.Char.Name, item.ItemID, err)
	}
	_ = SaveCharacter(c.Char)
	sendUpgradeResetResponse(c, 1, item, scroll)
}

func handleAdvancedEnchantWeapon(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	itemSlot, res1Slot, res2Slot, res3Slot, ok := readAdvancedEnchantRequest(p.Data)
	if !ok {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	item := findUpgradeableItem(c.Char, itemSlot)
	if item == nil || !canAdvancedEnchant(item) {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, item, nil, nil, nil)
		return
	}
	res1 := findItem(c.Char, types.InventoryRegular, res1Slot)
	res2 := findItem(c.Char, types.InventoryRegular, res2Slot)
	res3 := findItem(c.Char, types.InventoryRegular, res3Slot)
	cost, req1, req2, req3, ok := advancedEnchantCostAndMaterials(item)
	if !ok || res1 == nil || res2 == nil || res3 == nil || !advancedEnchantMaterialsMatch(item, res1, res2, res3) {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	if itemLockedForTrading(c.Char, res1) || itemLockedForTrading(c.Char, res2) || itemLockedForTrading(c.Char, res3) {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, item, res1, res2, res3)
		return
	}
	if res1.Amount < req1 || res2.Amount < req2 || res3.Amount < req3 {
		sendAdvancedEnchantDone(c, advancedEnchantStatusNoMaterials, nil, nil, nil, nil)
		return
	}
	if c.Char.Gold < cost {
		sendAdvancedEnchantDone(c, advancedEnchantStatusNotEnoughGold, nil, nil, nil, nil)
		return
	}
	c.Char.Gold -= cost
	if err := removeCharacterItem(c.Char, res1, req1); err != nil {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	if err := removeCharacterItem(c.Char, res2, req2); err != nil {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	if err := removeCharacterItem(c.Char, res3, req3); err != nil {
		sendAdvancedEnchantDone(c, advancedEnchantStatusFail, nil, nil, nil, nil)
		return
	}
	generateAdvancedEquipmentOption(item)
	if err := SaveItem(item); err != nil {
		log.Printf("[AdvancedEnchant] save item failed char=%q item=%d: %v", c.Char.Name, item.ItemID, err)
	}
	_ = SaveCharacter(c.Char)
	sendAdvancedEnchantDone(c, advancedEnchantStatusOK, item, res1, res2, res3)
}

func handleDisasembleEquipment(c *Client, p *PacketIn) {
	sendEquipmentDisasembled(c, itemStatusFail, nil)
}

func handleExchangeOption(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendExchangeItemOptionResult(c, exchangeOptionStatusParamError, nil, nil)
		return
	}
	scrollSlot, itemSlot, ok := readExchangeOptionRequest(p.Data)
	if !ok {
		sendExchangeItemOptionResult(c, exchangeOptionStatusParamError, nil, nil)
		return
	}
	scroll := optionalShopOrRegularItem(c.Char, scrollSlot)
	if scroll == nil || itemTemplateByID(scroll.ItemID).Category != types.ItemCategoryItemOptionExchange {
		sendExchangeItemOptionResult(c, exchangeOptionStatusScrollInvalid, nil, nil)
		return
	}
	item := findUpgradeableItem(c.Char, itemSlot)
	if item == nil || !isEquipmentTemplate(itemTemplateByID(item.ItemID)) {
		sendExchangeItemOptionResult(c, exchangeOptionStatusItemInvalid, nil, nil)
		return
	}
	if itemLockedForTrading(c.Char, scroll) || itemLockedForTrading(c.Char, item) {
		sendExchangeItemOptionResult(c, exchangeOptionStatusItemInvalid, item, scroll)
		return
	}
	if err := removeCharacterItem(c.Char, scroll, 1); err != nil {
		sendExchangeItemOptionResult(c, exchangeOptionStatusScrollInvalid, nil, nil)
		return
	}
	generateNewEquipmentOptions(item)
	if err := SaveItem(item); err != nil {
		log.Printf("[Options] save exchanged item failed char=%q item=%d: %v", c.Char.Name, item.ItemID, err)
		sendExchangeItemOptionResult(c, exchangeOptionStatusParamError, item, scroll)
		return
	}
	sendExchangeItemOptionResult(c, exchangeOptionStatusOK, item, scroll)
}

func readUpgradeRequest(data []byte) (upgradeRequest, bool) {
	if len(data) >= 16 {
		return upgradeRequest{
			itemSlot:        int16(binary.LittleEndian.Uint16(data[5:])),
			stoneSlot:       int16(binary.LittleEndian.Uint16(data[8:])),
			chanceBoostSlot: int16(binary.LittleEndian.Uint16(data[11:])),
			protectSlot:     int16(binary.LittleEndian.Uint16(data[14:])),
		}, true
	}
	if len(data) >= 7 {
		return upgradeRequest{itemSlot: int16(binary.LittleEndian.Uint16(data[5:])), stoneSlot: -1, chanceBoostSlot: -1, protectSlot: -1}, true
	}
	return upgradeRequest{}, false
}

func readAdvancedEnchantRequest(data []byte) (int16, int16, int16, int16, bool) {
	if len(data) < 33 {
		return -1, -1, -1, -1, false
	}
	itemSlot := int16(binary.LittleEndian.Uint16(data[4:]))
	res1 := int16(binary.LittleEndian.Uint32(data[7:]))
	res2 := int16(binary.LittleEndian.Uint32(data[18:]))
	res3 := int16(binary.LittleEndian.Uint32(data[29:]))
	return itemSlot, res1, res2, res3, true
}

func readExchangeOptionRequest(data []byte) (int16, int16, bool) {
	if len(data) < 14 {
		return -1, -1, false
	}
	scrollSlot := int16(binary.LittleEndian.Uint16(data[4:]))
	itemSlot := int16(binary.LittleEndian.Uint16(data[12:]))
	return scrollSlot, itemSlot, true
}

func findUpgradeableItem(chr *Character, slot int16) *ItemRow {
	for _, inv := range []byte{types.InventoryShop, types.InventoryEquipment, types.InventoryRegular} {
		if item := findItem(chr, inv, slot); item != nil {
			return item
		}
	}
	return nil
}

func optionalShopOrRegularItem(chr *Character, slot int16) *ItemRow {
	if slot < 0 {
		return nil
	}
	if item := findItem(chr, types.InventoryShop, slot); item != nil {
		return item
	}
	return findItem(chr, types.InventoryRegular, slot)
}

func itemEnchant(item *ItemRow) byte {
	if item == nil {
		return 0
	}
	return item.Enchant
}

func canUseUpgradeStone(item *ItemRow, stone *ItemRow) bool {
	itemTempl := itemTemplateByID(item.ItemID)
	stoneTempl := itemTemplateByID(stone.ItemID)
	if stone.ItemID == guaranteedUpgradeStoneID {
		return isEquipmentTemplate(itemTempl)
	}
	isWeapon := itemTempl.Kind == types.ItemKindWeapon
	isArmor := itemTempl.Kind == types.ItemKindArmor
	level := itemTempl.RequiredLevel
	switch stoneTempl.Category {
	case types.ItemCategoryEnchantWeaponStoneD:
		return isWeapon && level <= 20
	case types.ItemCategoryEnchantWeaponStoneC:
		return isWeapon && level <= 40
	case types.ItemCategoryEnchantWeaponStoneB:
		return isWeapon && level <= 60
	case types.ItemCategoryEnchantWeaponStoneA:
		return isWeapon && level <= 80
	case types.ItemCategoryEnchantWeaponStoneS:
		return isWeapon
	case types.ItemCategoryEnchantArmorStoneD:
		return isArmor && level <= 20
	case types.ItemCategoryEnchantArmorStoneC:
		return isArmor && level <= 40
	case types.ItemCategoryEnchantArmorStoneB:
		return isArmor && level <= 60
	case types.ItemCategoryEnchantArmorStoneA:
		return isArmor && level <= 80
	case types.ItemCategoryEnchantArmorStoneS, types.ItemCategoryEnchantArmorStoneE:
		return isArmor
	case types.ItemCategoryEnchantUniversalStoneE, types.ItemCategoryEnchantUniversalStoneS:
		return isArmor || isWeapon
	case types.ItemCategoryEnchantUniversalStoneD:
		return (isArmor || isWeapon) && level <= 20
	case types.ItemCategoryEnchantUniversalStoneC:
		return (isArmor || isWeapon) && level <= 40
	case types.ItemCategoryEnchantUniversalStoneB:
		return (isArmor || isWeapon) && level <= 60
	case types.ItemCategoryEnchantUniversalStoneA:
		return (isArmor || isWeapon) && level <= 80
	case types.ItemCategoryEnchant100Stone:
		return isEquipmentTemplate(itemTempl)
	default:
		return false
	}
}

func isUpgradeChanceBoostItem(item *ItemRow) bool {
	if item == nil {
		return false
	}
	category := itemTemplateByID(item.ItemID).Category
	return category == types.ItemCategoryIncreaseUpgradeChance || category == types.ItemCategorySuperiorUpgradeBoost
}

func isUpgradeProtectionItem(item *ItemRow) bool {
	return item != nil && itemTemplateByID(item.ItemID).Category == types.ItemCategoryUpgradeProtectScroll
}

func upgradeProtectionFlags(item *ItemRow, protect *ItemRow) (bool, bool) {
	if item == nil || protect == nil {
		return false, false
	}
	value := itemTemplateByID(protect.ItemID).ValueOnUse
	if item.Enchant >= 10 {
		switch value {
		case 1:
			return true, false
		case 2:
			return true, true
		}
		return false, false
	}
	return value == 0, false
}

func calculateItemUpgradeResult(stoneQuality types.ItemQuality, itemQuality types.ItemQuality, enchant byte, requiredLevel byte, ownerLuck int, useProtect bool, useChanceBoost int, noEnchantLose bool, forceSuccess bool) itemUpgradeResult {
	base := 200.0
	switch itemQuality {
	case types.ItemQualityWhite:
		base -= 5
	case types.ItemQualityYellow:
		base -= 10
	case types.ItemQualityPurple:
		base -= 25
	case types.ItemQualityGreen:
		base -= 40
	case types.ItemQualityOrange:
		base -= 50
	}
	switch stoneQuality {
	case types.ItemQualityYellow:
		base += 10
	case types.ItemQualityPurple:
		base += 30
	case types.ItemQualityGreen, types.ItemQualityOrange:
		base += 1000
	}
	ownerBoost := math.Min(math.Max(float64(ownerLuck), 0)*0.0005, 0.3)
	chanceBase := base / math.Pow(float64(enchant)+0.1, 0.75)
	if enchant < 10 {
		chanceBase = chanceBase*1.3 + float64(enchant)*0.9
	}
	if enchant > 15 {
		chanceBase *= 0.4
	}
	chance := chanceBase * (1 + ownerBoost - math.Pow(float64(requiredLevel), 0.85)/100)
	chance *= 1 + float64(useChanceBoost)/100
	if forceSuccess {
		chance = 100
	}
	if chance > 100 {
		chance = 100
	}
	if chance < 0 {
		chance = 0
	}
	if enchant >= 20 && !forceSuccess {
		chance = 0
	}
	roll := upgradeRandFloat() * 100
	if roll <= chance {
		return itemUpgradeResult{status: itemUpgradeSuccess, chance: chance}
	}
	if roll < chance+(100-chance)/6 {
		if useProtect {
			if enchant <= 10 || noEnchantLose {
				return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
			}
			return itemUpgradeResult{status: itemUpgradeReduceOne, chance: chance}
		}
		if enchant > 7 {
			return itemUpgradeResult{status: itemUpgradeBreak, chance: chance}
		}
		if enchant > 4 {
			return itemUpgradeResult{status: itemUpgradeReduceOne, chance: chance}
		}
		return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
	}
	if roll < chance+(100-chance)/4 {
		if useProtect {
			if enchant <= 10 || noEnchantLose {
				return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
			}
			return itemUpgradeResult{status: itemUpgradeReduceOne, chance: chance}
		}
		if enchant > 7 {
			return itemUpgradeResult{status: itemUpgradeReduceToZero, chance: chance}
		}
		return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
	}
	if roll >= chance+(100-chance)/2 || useProtect {
		return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
	}
	if enchant > 7 {
		return itemUpgradeResult{status: itemUpgradeReduceOne, chance: chance}
	}
	return itemUpgradeResult{status: itemUpgradeFail, chance: chance}
}

func enchantPrice(enchant byte, templ ItemTemplate) int64 {
	start := enchantStartPrice(int(templ.RequiredLevel))
	step := enchantStep(templ.Quality)
	base := start + step + (start * int(enchant)) + ((int(enchant) + 1) * step)
	adjustment := 0
	if enchant%2 != 0 {
		adjustment = step
	}
	return int64((base - adjustment) + enchantLevelPriceAdjustment(int(enchant), step))
}

func enchantStartPrice(level int) int {
	switch {
	case level < 20:
		return 0
	case level < 40:
		return 500
	case level < 60:
		return 1000
	case level < 80:
		return 3000
	case level < 100:
		return 6000
	default:
		return 0
	}
}

func enchantStep(quality types.ItemQuality) int {
	switch quality {
	case types.ItemQualityWhite:
		return 5
	case types.ItemQualityYellow:
		return 25
	case types.ItemQualityPurple:
		return 250
	case types.ItemQualityGreen:
		return 1250
	case types.ItemQualityOrange:
		return 5000
	default:
		return 0
	}
}

func enchantLevelPriceAdjustment(enchant int, step int) int {
	switch enchant {
	case 0:
		return -step
	case 3:
		return step * 2
	case 4:
		return step * 4
	case 5, 6:
		return step * 6
	case 7, 8:
		return step * 8
	case 9, 10:
		return step * 10
	case 11, 12:
		return step * 12
	case 13, 14:
		return step * 14
	case 15, 16:
		return step * 16
	case 17, 18:
		return step * 18
	case 19:
		return step * 20
	case 20:
		return step * 19
	default:
		return 0
	}
}

func canAdvancedEnchant(item *ItemRow) bool {
	quality := itemTemplateByID(item.ItemID).Quality
	return quality == types.ItemQualityGreen || quality == types.ItemQualityPurple
}

func advancedEnchantMaterialsMatch(item *ItemRow, res1 *ItemRow, res2 *ItemRow, res3 *ItemRow) bool {
	quality := itemTemplateByID(item.ItemID).Quality
	if quality == types.ItemQualityGreen {
		return res1.ItemID == 33706 && res2.ItemID == 20681 && res3.ItemID == 33705
	}
	if quality == types.ItemQualityPurple {
		return res1.ItemID == 20681 && res2.ItemID == 20680 && res3.ItemID == 33705
	}
	return false
}

func advancedEnchantCostAndMaterials(item *ItemRow) (int64, int, int, int, bool) {
	level := itemTemplateByID(item.ItemID).AuctionLevel
	switch level {
	case 0:
		return 50_000, 1, 1, 3, true
	case 1:
		return 100_000, 1, 2, 6, true
	case 2:
		return 200_000, 2, 3, 9, true
	case 3:
		return 400_000, 2, 4, 12, true
	default:
		return 800_000, 3, 5, 15, true
	}
}

func isUpgradeResetScroll(scroll *ItemRow) bool {
	return scroll != nil && (scroll.ItemID == 668 || scroll.ItemID == 669)
}

func sendUpgradeItemResponse(c *Client, status byte, item *ItemRow, stone *ItemRow, chanceBoost *ItemRow, protect *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UpgradeItemResponse)
	p.WriteUint8(status)
	p.WriteInt32(int32(itemWeight(c.Char)))
	p.WriteInt32(clampInt32(c.Char.Gold))
	writeItemInfoToPacket(p, item, c.Char, false)
	writeItemInfoToPacket(p, stone, c.Char, false)
	writeItemInfoToPacket(p, chanceBoost, c.Char, false)
	writeItemInfoToPacket(p, protect, c.Char, false)
	c.Send(p)
}

func sendUpgradeResetResponse(c *Client, status byte, item *ItemRow, scroll *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UpgradeResetResponse)
	p.WriteUint8(status)
	p.WriteInt32(int32(itemWeight(c.Char)))
	p.WriteInt32(clampInt32(c.Char.Gold))
	writeItemInfoToPacket(p, item, c.Char, false)
	writeItemInfoToPacket(p, scroll, c.Char, false)
	c.Send(p)
}

func sendAdvancedEnchantDone(c *Client, status byte, item *ItemRow, res1 *ItemRow, res2 *ItemRow, res3 *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(AdvancedEnchantDone)
	p.WriteUint8(status)
	p.WriteInt16(itemWeight(c.Char))
	p.WriteInt32(clampInt32(c.Char.Gold))
	writeItemInfoToPacket(p, item, c.Char, false)
	writeItemInfoToPacket(p, res1, c.Char, false)
	writeItemInfoToPacket(p, res2, c.Char, false)
	writeItemInfoToPacket(p, res3, c.Char, false)
	c.Send(p)
}

func sendExchangeItemOptionResult(c *Client, status byte, item *ItemRow, scroll *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ExchangeItemOptionResult)
	p.WriteUint8(status)
	writeItemInfoToPacket(p, scroll, c.Char, false)
	writeItemInfoToPacket(p, item, c.Char, false)
	p.WriteInt16(itemWeight(c.Char))
	p.WriteInt16(0)
	c.Send(p)
}
