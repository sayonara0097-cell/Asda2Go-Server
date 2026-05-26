package main

import (
	"encoding/binary"
	"log"

	"asda2/shared/types"
)

// ---- Trade ----

func handleTradeRequest(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	targetSession, tradeType := readTradeRequest(c, p.Data)
	target := getClientBySessionID(targetSession)
	if !canStartTrade(c, target, tradeType) {
		log.Printf("[Trade] rejected sender=%s target=%s session=%d type=%d payload=% X",
			clientDebugLabel(c), clientDebugLabel(target), targetSession, tradeType, limitedPacketBytes(p.Data))
		sendTradeRejected(c)
		return
	}
	if !tradeRuntime.begin(c, target, tradeType) {
		sendTradeRejected(c)
		return
	}
	sendTradeRequestResponse(target, c, tradeType)
}

func handleStartTradeResponse(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	if !readStartTradeAccepted(p.Data) {
		sendTradeRejectedToAll(tradeRuntime.cancelFor(c))
		return
	}
	session, ok := tradeRuntime.accept(c)
	if !ok || session == nil {
		sendTradeRejected(c)
		return
	}
	sendTradeStartedResponse(session.First, session.Second, session.TradeType)
	sendTradeStartedResponse(session.Second, session.First, session.TradeType)
}

func handleCancelTrade(c *Client, p *PacketIn) {
	sendTradeRejectedToAll(tradeRuntime.cancelFor(c))
}

func handlePushItemToTrade(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	inv, slot, amount := readPushItemToTradeRequest(c, p.Data)
	status, ref, canceled := tradeRuntime.pushItem(c, inv, slot, amount)
	if len(canceled) > 0 {
		sendTradeRejectedToAll(canceled)
		return
	}
	sendItemToTradePushedResponse(c, status, ref)
}

func handlePopItemFromTrade(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	inv, slot := readPopItemFromTradeRequest(p.Data)
	sendTradeRejectedToAll(tradeRuntime.popItem(c, inv, slot))
}

func handleConfimTradeOne(c *Client, p *PacketIn) {
	receiver, items := tradeRuntime.showItems(c)
	if receiver == nil {
		return
	}
	sendConfimTradeFromOponentResponse(receiver, items)
}

func handleConfimTradeTwo(c *Client, p *PacketIn) {
	session, result, canceled, err := tradeRuntime.confirm(c)
	if err != nil {
		log.Printf("[Trade] confirm failed char=%s: %v", clientDebugLabel(c), err)
	}
	if len(canceled) > 0 {
		sendTradeRejectedToAll(canceled)
		return
	}
	if session == nil {
		return
	}
	if session.TradeType == tradeTypeShopItems {
		sendShopTradeCompleteResponse(session.First, result.FirstReceives)
		sendShopTradeCompleteResponse(session.Second, result.SecondReceives)
		return
	}
	sendRegularTradeCompleteResponse(session.First, result.FirstReceives)
	sendRegularTradeCompleteResponse(session.Second, result.SecondReceives)
}

func canStartTrade(sender *Client, target *Client, tradeType byte) bool {
	if sender == nil || sender.Char == nil || target == nil || target.Char == nil {
		return false
	}
	if sender == target || sender.Char.GUID == target.Char.GUID {
		return false
	}
	if sender.Channel != target.Channel || sender.Char.MapID != target.Char.MapID {
		return false
	}
	if tradeType == tradeTypeShopItems {
		if !tradeSettingsEnabled(target.Char, settingsShopItemTrade) {
			return false
		}
	} else if !tradeSettingsEnabled(target.Char, settingsGeneralTrade) {
		return false
	}
	if privateShopRuntime.isBusy(sender) || privateShopRuntime.isBusy(target) {
		return false
	}
	return true
}

func tradeSettingsEnabled(chr *Character, index int) bool {
	if chr == nil || index < 0 || index >= len(chr.SettingsFlags) {
		return false
	}
	return chr.SettingsFlags[index] == 1
}

func readTradeRequest(sender *Client, data []byte) (int16, byte) {
	if len(data) >= 3 {
		sessionID := int16(binary.LittleEndian.Uint16(data))
		target := getClientBySessionID(sessionID)
		if target != nil && target != sender {
			return sessionID, normalizeTradeType(data[2])
		}
	}
	if sessionID, tradeType, ok := findClientSessionInPacket(sender, data); ok {
		return sessionID, normalizeTradeType(tradeType)
	}
	return 0, tradeTypeRegular
}

func readStartTradeAccepted(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if data[0] == 0 || data[0] == 1 {
		return data[0] == 1
	}
	if len(data) > 16 && (data[16] == 0 || data[16] == 1) {
		return data[16] == 1
	}
	for _, b := range data {
		if b == 1 {
			return true
		}
		if b == 0 {
			return false
		}
	}
	return false
}

func readPushItemToTradeRequest(c *Client, data []byte) (byte, int16, int) {
	type candidate struct {
		inv    byte
		slot   int16
		amount int
		score  int
	}
	best := candidate{amount: 1, score: -1}
	for offset := 0; offset+6 < len(data) && offset < 24; offset++ {
		inv := data[offset]
		slot := int16(binary.LittleEndian.Uint16(data[offset+1:]))
		amount := int(int32(binary.LittleEndian.Uint32(data[offset+3:])))
		score := tradeItemRequestScore(c, inv, slot, amount)
		if score > best.score {
			best = candidate{inv: inv, slot: slot, amount: amount, score: score}
		}
	}
	if best.score >= 0 {
		return best.inv, best.slot, best.amount
	}
	return 0, -1, 0
}

func readPopItemFromTradeRequest(data []byte) (byte, int16) {
	if len(data) >= 7 {
		return data[4], int16(binary.LittleEndian.Uint16(data[5:]))
	}
	if len(data) >= 3 {
		return data[0], int16(binary.LittleEndian.Uint16(data[1:]))
	}
	return 0, -1
}

func tradeItemRequestScore(c *Client, inv byte, slot int16, amount int) int {
	if inv != types.InventoryRegular && inv != types.InventoryShop {
		return -1
	}
	if amount <= 0 {
		return -1
	}
	if inv == types.InventoryRegular && slot == types.ReservedRegularInventorySlot {
		return 20
	}
	item := findItem(c.Char, inv, slot)
	if item == nil {
		return -1
	}
	score := 10
	if amount <= item.Amount {
		score += 5
	}
	return score
}

func normalizeTradeType(tradeType byte) byte {
	if tradeType == tradeTypeShopItems {
		return tradeTypeShopItems
	}
	return tradeTypeRegular
}

func sendTradeRequestResponse(receiver *Client, sender *Client, tradeType byte) {
	if receiver == nil || sender == nil || sender.Char == nil {
		return
	}
	p := NewPacket(TradeRequestResponse)
	p.WriteUint8(1)
	p.WriteInt16(sender.Char.SessionID)
	p.WriteAsdaStringLocale(sender.Char.Name, 20, receiver.Locale)
	p.WriteUint8(normalizeTradeType(tradeType))
	receiver.Send(p)
}

func sendTradeRejected(c *Client) {
	if c == nil {
		return
	}
	c.Send(NewPacket(TradeRejected))
}

func sendTradeRejectedToAll(list []*Client) {
	for _, c := range list {
		sendTradeRejected(c)
	}
}

func sendTradeStartedResponse(receiver *Client, other *Client, tradeType byte) {
	if receiver == nil || receiver.Char == nil || other == nil || other.Char == nil {
		return
	}
	p := NewPacket(TradeStarted)
	p.WriteUint8(tradeStartedOK)
	if normalizeTradeType(tradeType) == tradeTypeRegular {
		p.WriteUint8(0)
	} else {
		p.WriteUint8(1)
	}
	p.WriteInt32(1)
	p.WriteInt16(receiver.Char.SessionID)
	p.WriteAsdaStringLocale(receiver.Char.Name, 20, receiver.Locale)
	p.WriteInt16(other.Char.SessionID)
	p.WriteAsdaStringLocale(other.Char.Name, 20, receiver.Locale)
	receiver.Send(p)
}

func sendItemToTradePushedResponse(c *Client, status byte, ref *tradeItemRef) {
	if c == nil {
		return
	}
	p := NewPacket(ItemToTradePushed)
	p.WriteUint8(status)
	if ref == nil {
		p.WriteUint8(0)
		p.WriteInt16(0)
		p.WriteInt32(0)
	} else {
		p.WriteUint8(ref.InventoryType)
		p.WriteInt16(ref.Slot)
		p.WriteInt32(int32(ref.Amount))
	}
	c.Send(p)
}

func sendConfimTradeFromOponentResponse(c *Client, items [tradeSlotCount]*tradeItemRef) {
	if c == nil {
		return
	}
	p := NewPacket(ConfimTradeFromOponent)
	p.WriteInt32(1)
	p.WriteUint8(1)
	p.WriteUint8(1)
	for i := 0; i < tradeSlotCount; i++ {
		writeTradePreviewRef(p, items[i])
	}
	c.Send(p)
}

func sendRegularTradeCompleteResponse(c *Client, items [tradeSlotCount]*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(RegularTradeComplete)
	p.WriteUint8(tradeSlotCount)
	p.WriteInt32(int32(itemWeight(c.Char)))
	for i := 0; i < tradeSlotCount; i++ {
		item := items[i]
		if item == nil {
			p.WriteInt32(-1)
			p.WriteUint8(0)
			p.WriteInt16(-1)
			p.WriteInt32(0)
			p.WriteInt16(0)
			continue
		}
		p.WriteInt32(int32(item.ItemID))
		p.WriteUint8(item.InventoryType)
		p.WriteInt16(item.Slot)
		p.WriteInt32(int32(item.Amount))
		p.WriteInt16(clampInt16(int32(itemUnitWeight(item))))
	}
	c.Send(p)
}

func sendShopTradeCompleteResponse(c *Client, items [tradeSlotCount]*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ShopTradeComplete)
	p.WriteUint8(tradeSlotCount)
	p.WriteInt32(int32(itemWeight(c.Char)))
	for i := 0; i < tradeSlotCount; i++ {
		writeShopTradeCompleteItem(p, items[i], c.Char)
	}
	c.Send(p)
}

func writeTradePreviewRef(p *PacketOut, ref *tradeItemRef) {
	if ref == nil {
		writeTradePreviewValues(p, nil, 0, false)
		return
	}
	if ref.IsGold {
		item := &ItemRow{
			ItemID:        int(goldLootItemID),
			InventoryType: types.InventoryRegular,
			Slot:          types.ReservedRegularInventorySlot,
			Amount:        ref.Amount,
			IsStackable:   true,
		}
		writeTradePreviewValues(p, item, ref.Amount, false)
		return
	}
	writeTradePreviewValues(p, ref.Item, ref.Amount, false)
}

func writeTradePreviewValues(p *PacketOut, item *ItemRow, amount int, nilNegative bool) {
	if item == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(0)
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(0)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteInt32(0)
		p.WriteInt16(0)
		for i := 0; i < 5; i++ {
			if nilNegative {
				p.WriteInt16(-1)
				p.WriteInt16(-1)
			} else {
				p.WriteInt16(-1)
				p.WriteInt16(-1)
			}
		}
		p.WriteUint8(0)
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteInt16(0)
		return
	}
	if amount <= 0 {
		amount = item.Amount
	}
	p.WriteInt32(int32(item.ItemID))
	p.WriteUint8(item.Durability)
	p.WriteInt16(clampInt16(int32(itemUnitWeight(item))))
	p.WriteInt32(int32(amount))
	p.WriteUint8(item.Enchant)
	p.WriteInt16(0)
	p.WriteUint8(item.EnchantResetCount)
	p.WriteUint8(item.SealCount)
	p.WriteInt32(0)
	p.WriteInt16(0)
	p.WriteInt16(item.Param1Type)
	p.WriteInt16(item.Param1Value)
	p.WriteInt16(item.Param2Type)
	p.WriteInt16(item.Param2Value)
	p.WriteInt16(item.Param3Type)
	p.WriteUint16(item.Param3Value)
	p.WriteInt16(item.Param4Type)
	p.WriteInt16(item.Param4Value)
	p.WriteInt16(item.Param5Type)
	p.WriteInt16(item.Param5Value)
	p.WriteUint8(0)
	p.WriteInt16(int16(item.Soul1ID))
	p.WriteInt16(int16(item.Soul2ID))
	p.WriteInt16(int16(item.Soul3ID))
	p.WriteInt16(int16(item.Soul4ID))
}

func writeShopTradeCompleteItem(p *PacketOut, item *ItemRow, owner *Character) {
	if item == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(-1)
		p.WriteUint8(0xFF)
		p.WriteInt16(-1)
		p.WriteUint8(0xFF)
		p.WriteInt16(0)
		p.WriteUint8(0xFF)
		p.WriteUint8(0xFF)
		p.WriteInt32(0)
		p.WriteInt16(0)
		for i := 0; i < 5; i++ {
			p.WriteInt16(-1)
			p.WriteInt16(-1)
		}
		p.WriteUint8(0)
		p.WriteInt32(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		return
	}
	p.WriteInt32(int32(item.ItemID))
	p.WriteUint8(item.InventoryType)
	p.WriteInt16(item.Slot)
	p.WriteUint8(item.Durability)
	p.WriteInt16(clampInt16(int32(itemUnitWeight(item))))
	p.WriteUint8(item.Enchant)
	p.WriteInt16(0)
	p.WriteUint8(item.EnchantResetCount)
	p.WriteUint8(item.SealCount)
	p.WriteInt32(0)
	p.WriteInt16(0)
	p.WriteInt16(item.Param1Type)
	p.WriteInt16(item.Param1Value)
	p.WriteInt16(item.Param2Type)
	p.WriteInt16(item.Param2Value)
	p.WriteInt16(item.Param3Type)
	p.WriteUint16(item.Param3Value)
	p.WriteInt16(item.Param4Type)
	p.WriteInt16(item.Param4Value)
	p.WriteInt16(item.Param5Type)
	p.WriteInt16(item.Param5Value)
	p.WriteUint8(0)
	writeTradeItemAmount(p, item, owner, false)
	p.WriteInt16(int16(item.Soul1ID))
	p.WriteInt16(int16(item.Soul2ID))
	p.WriteInt16(int16(item.Soul3ID))
	p.WriteInt16(int16(item.Soul4ID))
}

func writeTradeItemAmount(p *PacketOut, item *ItemRow, owner *Character, setDeletedToZero bool) {
	if item == nil {
		p.WriteInt32(-1)
		return
	}
	if item.ItemID == int(goldLootItemID) {
		if owner != nil {
			p.WriteInt32(clampInt32(owner.Gold))
		} else {
			p.WriteInt32(int32(item.Amount))
		}
		return
	}
	if item.Amount <= 0 {
		if setDeletedToZero {
			p.WriteInt32(0)
		} else {
			p.WriteInt32(-1)
		}
		return
	}
	if effectiveItemStackable(item) {
		p.WriteInt32(int32(item.Amount))
		return
	}
	p.WriteInt32(0)
}
