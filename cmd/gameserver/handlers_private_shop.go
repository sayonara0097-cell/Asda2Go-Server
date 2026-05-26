package main

import (
	"encoding/binary"
	"log"
	"strings"

	"asda2/shared/types"
)

// ---- Private Shop ----

func handleOpenPrivateShopWindow(c *Client, p *PacketIn) {
	status := privateShopRuntime.openWindow(c)
	sendPrivateShopWindoOpenedResponse(c, status)
}

func handleOpenPrivateShop(c *Client, p *PacketIn) {
	refs, title, parseStatus := readOpenPrivateShopRequest(c, p.Data)
	if parseStatus != privateShopOpenedOK {
		sendPrivateShopOpenedResponse(c, parseStatus, nil)
		return
	}
	status, shop := privateShopRuntime.start(c, refs, title)
	sendPrivateShopOpenedResponse(c, status, shop)
	if status == privateShopOpenedOK && shop != nil {
		sendTradeStatusTextWindowToArea(c, true, shop.Title)
	}
}

func handleCloseCharacterTradeShop(c *Client, p *PacketIn) {
	status, shop, notify, ownerClosed := privateShopRuntime.closeFor(c)
	if ownerClosed {
		for _, viewer := range notify {
			sendCloseCharacterTradeShopToOwnerResponse(viewer, privateShopCloseHostClosed)
		}
		sendCloseCharacterTradeShopToOwnerResponse(c, status)
		sendTradeStatusTextWindowToArea(c, false, "")
		return
	}
	sendCloseCharacterTradeShopToOwnerResponse(c, status)
	if shop != nil && c != nil && c.Char != nil {
		for _, receiver := range notify {
			sendPrivateShopChatNotification(receiver, c.Char.AccID, privateShopNotifyLeft)
		}
	}
}

func handleViewCharacterTradeShop(c *Client, p *PacketIn) {
	ownerAccID := readViewPrivateShopRequest(p.Data)
	status, shop, notify := privateShopRuntime.view(c, ownerAccID)
	if status != privateShopInfoOK || shop == nil {
		sendCharacterPrivateShopInfoResponse(c, status, nil)
		return
	}
	if c != nil && c.Char != nil {
		for _, receiver := range notify {
			sendPrivateShopChatNotification(receiver, c.Char.AccID, privateShopNotifyJoined)
		}
	}
	sendCharacterPrivateShopInfoResponse(c, status, shop)
}

func handleBuyItemFromCharacterPrivateShop(c *Client, p *PacketIn) {
	requests := readBuyPrivateShopRequest(p.Data)
	status, shop, sold, bought, err := privateShopRuntime.buy(c, requests)
	if err != nil {
		log.Printf("[PrivateShop] buy failed buyer=%s: %v", clientDebugLabel(c), err)
	}
	sendItemBuyedFromPrivateShopResponse(c, status, bought)
	if status != privateShopBuyOK || shop == nil || shop.Owner == nil {
		return
	}
	sendItemBuyedFromPrivateShopToOwnerNotifyResponse(shop, sold, c)
	sendPrivateShopChatNotificationAboutBuyResponse(shop, sold, c)
}

func handlePrivateShopChatReq(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	shop := privateShopRuntime.shopForClient(c)
	if shop == nil {
		return
	}
	msg := strings.TrimSpace(readPrivateShopChatRequest(c, p.Data))
	if msg == "" {
		return
	}
	for _, receiver := range privateShopParticipants(shop) {
		sendPrivateShopChatResResponse(receiver, c, msg)
	}
}

func readOpenPrivateShopRequest(c *Client, data []byte) ([privateShopItemSlots]*tradeItemRef, string, byte) {
	var refs [privateShopItemSlots]*tradeItemRef
	if c == nil || c.Char == nil {
		return refs, "", privateShopOpenedError
	}
	offset := 1
	seen := map[byte]map[int16]bool{}
	nextTradeSlot := byte(0)
	for index := 0; index < privateShopItemSlots && offset+44 < len(data); index++ {
		marker := binary.LittleEndian.Uint32(data[offset:])
		inv := data[offset+4]
		slot := int16(binary.LittleEndian.Uint16(data[offset+5:]))
		amount := int(int16(binary.LittleEndian.Uint16(data[offset+7:])))
		price := int64(int32(binary.LittleEndian.Uint32(data[offset+13:])))
		offset += 45

		if marker == 0 {
			continue
		}
		if amount < 0 || price < 0 || (inv != types.InventoryRegular && inv != types.InventoryShop) {
			continue
		}
		item := findItem(c.Char, inv, slot)
		if item == nil {
			return refs, "", privateShopOpenedNoItemInfo
		}
		if item.ItemID == int(goldLootItemID) {
			return refs, "", privateShopOpenedGoldDenied
		}
		if item.IsSoulBound {
			return refs, "", privateShopOpenedUnexchangeable
		}
		if amount <= 0 {
			amount = 1
		}
		if amount > item.Amount {
			amount = item.Amount
		}
		if seen[inv] == nil {
			seen[inv] = map[int16]bool{}
		}
		if seen[inv][slot] {
			return refs, "", privateShopOpenedItemAlreadyPlaced
		}
		seen[inv][slot] = true
		if nextTradeSlot >= privateShopItemSlots {
			return refs, "", privateShopOpenedTooManyItems
		}
		refs[nextTradeSlot] = &tradeItemRef{
			Item:          item,
			Amount:        amount,
			Price:         price,
			TradeSlot:     nextTradeSlot,
			InventoryType: inv,
			Slot:          slot,
		}
		nextTradeSlot++
	}
	title := ""
	if offset < len(data) {
		in := &PacketIn{Data: data}
		in.Seek(offset)
		title = strings.TrimSpace(in.ReadAsdaStringLocale(50, c.Locale))
	}
	return refs, title, privateShopOpenedOK
}

func readViewPrivateShopRequest(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func readBuyPrivateShopRequest(data []byte) []privateShopBuyRequest {
	requests := make([]privateShopBuyRequest, 0, privateShopBuySlots)
	offset := 7
	for index := 0; index < privateShopBuySlots && offset+44 < len(data); index++ {
		marker := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if marker == 0 {
			break
		}
		offset += 3
		amount := int(int32(binary.LittleEndian.Uint32(data[offset:])))
		offset += 4
		slot := int16(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2
		offset += 32
		if slot < 0 || slot >= privateShopItemSlots {
			continue
		}
		if amount <= 0 {
			amount = 1
		}
		requests = append(requests, privateShopBuyRequest{TradeSlot: byte(slot), Amount: amount})
	}
	if len(requests) == 0 && len(data) >= 6 {
		amount := int(int32(binary.LittleEndian.Uint32(data)))
		slot := int16(binary.LittleEndian.Uint16(data[4:]))
		if slot >= 0 && slot < privateShopItemSlots {
			if amount <= 0 {
				amount = 1
			}
			requests = append(requests, privateShopBuyRequest{TradeSlot: byte(slot), Amount: amount})
		}
	}
	return requests
}

func readPrivateShopChatRequest(c *Client, data []byte) string {
	offset := 22
	if len(data) < offset {
		offset = 0
	}
	in := &PacketIn{Data: data}
	in.Seek(offset)
	return in.ReadCStringLocale(maxChatMessageLen, c.Locale)
}

func sendPrivateShopWindoOpenedResponse(c *Client, status byte) {
	if c == nil {
		return
	}
	p := NewPacket(PrivateShopWindoOpened)
	p.WriteUint8(status)
	c.Send(p)
}

func sendPrivateShopOpenedResponse(c *Client, status byte, shop *privateShop) {
	if c == nil {
		return
	}
	p := NewPacket(PrivateShopOpened)
	p.WriteUint8(status)
	if status == privateShopOpenedOK && shop != nil {
		p.WriteUint8(shopItemsCount(shop))
		for i := 0; i < privateShopItemSlots; i++ {
			writePrivateShopOpenedItem(p, shop.Items[i])
		}
	}
	c.Send(p)
}

func sendCharacterPrivateShopInfoResponse(c *Client, status byte, shop *privateShop) {
	if c == nil {
		return
	}
	p := NewPacket(CharacterPrivateShopInfo)
	p.WriteUint8(status)
	if status == privateShopInfoOK && shop != nil && shop.Owner != nil && shop.Owner.Char != nil {
		p.WriteUint32(shop.Owner.Char.AccID)
		p.WriteInt16(shop.Owner.Char.SessionID)
		p.WriteUint8(shopItemsCount(shop))
		p.WriteAsdaStringLocale(shop.Title, 50, c.Locale)
		p.WriteAsdaStringLocale(shop.Owner.Char.Name, 20, c.Locale)
		for i := 0; i < privateShopItemSlots; i++ {
			writePrivateShopInfoItem(p, shop.Items[i])
		}
	}
	c.Send(p)
}

func sendItemBuyedFromPrivateShopResponse(c *Client, status byte, bought []*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ItemBuyedFromPrivateShop)
	p.WriteUint8(status)
	if status == privateShopBuyOK {
		p.WriteInt16(itemWeight(c.Char))
		p.WriteInt32(clampInt32(c.Char.Gold))
		p.WriteUint8(byte(countItems(bought)))
		for i := 0; i < privateShopBuySlots; i++ {
			var item *ItemRow
			if i < len(bought) {
				item = bought[i]
			}
			writeItemInfoToPacket(p, item, c.Char, false)
		}
	}
	c.Send(p)
}

func sendCloseCharacterTradeShopToOwnerResponse(c *Client, status byte) {
	if c == nil {
		return
	}
	p := NewPacket(CloseCharacterTradeShopResponseToOwner)
	p.WriteUint8(status)
	for i := 0; i < privateShopItemSlots; i++ {
		writeItemInfoToPacket(p, nil, c.Char, false)
	}
	c.Send(p)
}

func sendPrivateShopChatNotification(receiver *Client, triggererAccID uint32, notificationType byte) {
	if receiver == nil {
		return
	}
	p := NewPacket(PrivateShopChatNotification)
	p.WriteUint8(notificationType)
	p.WriteUint32(triggererAccID)
	receiver.Send(p)
}

func sendPrivateShopChatResResponse(receiver *Client, sender *Client, msg string) {
	if receiver == nil || sender == nil || sender.Char == nil {
		return
	}
	p := NewPacket(PrivateShopChatRes)
	p.WriteInt32(1)
	p.WriteAsdaStringLocale(sender.Char.Name, 20, receiver.Locale)
	p.WriteCStringLocale(msg, maxChatMessageLen, receiver.Locale)
	p.WriteUint8(0)
	receiver.Send(p)
}

func sendItemBuyedFromPrivateShopToOwnerNotifyResponse(shop *privateShop, sold []privateShopSoldRef, buyer *Client) {
	if shop == nil || shop.Owner == nil || shop.Owner.Char == nil || buyer == nil || buyer.Char == nil {
		return
	}
	p := NewPacket(ItemBuyedFromPrivateShopToOwnerNotify)
	p.WriteInt16(itemWeight(shop.Owner.Char))
	p.WriteInt32(clampInt32(shop.Owner.Char.Gold))
	p.WriteUint8(byte(len(sold)))
	p.WriteUint32(buyer.Char.AccID)
	for i := 0; i < privateShopBuySlots; i++ {
		var ref *tradeItemRef
		if i < len(sold) {
			ref = sold[i].Ref
		}
		writePrivateShopOwnerSoldItem(p, ref)
	}
	shop.Owner.Send(p)
}

func sendPrivateShopChatNotificationAboutBuyResponse(shop *privateShop, sold []privateShopSoldRef, buyer *Client) {
	if shop == nil || shop.Owner == nil || shop.Owner.Char == nil || buyer == nil || buyer.Char == nil {
		return
	}
	for _, receiver := range privateShopParticipants(shop) {
		p := NewPacket(PrivateShopChatNotificationAboutBuy)
		p.WriteUint32(shop.Owner.Char.AccID)
		p.WriteInt16(shop.Owner.Char.SessionID)
		p.WriteUint8(byte(len(sold)))
		p.WriteUint32(buyer.Char.AccID)
		for i := 0; i < privateShopBuySlots; i++ {
			var ref *tradeItemRef
			if i < len(sold) {
				ref = sold[i].Ref
			}
			writePrivateShopBuyChatItem(p, ref)
		}
		receiver.Send(p)
	}
}

func sendTradeStatusTextWindowToArea(c *Client, enabled bool, title string) {
	if c == nil || c.Char == nil {
		return
	}
	for _, receiver := range World.AreaRecipients(c, true) {
		p := NewPacket(TradeStatusTextWindow)
		if enabled {
			p.WriteUint8(1)
		} else {
			p.WriteUint8(0)
		}
		p.WriteUint8(0)
		p.WriteUint32(c.Char.AccID)
		p.WriteAsdaStringLocale(title, 50, receiver.Locale)
		receiver.Send(p)
	}
}

func writePrivateShopOpenedItem(p *PacketOut, ref *tradeItemRef) {
	if ref == nil || ref.Item == nil || ref.Amount == -1 {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(-1)
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteInt32(0)
		for i := 0; i < 4; i++ {
			p.WriteInt16(-1)
		}
		for i := 0; i < 5; i++ {
			p.WriteInt16(-1)
			p.WriteInt16(-1)
		}
		return
	}
	item := ref.Item
	p.WriteInt32(int32(item.ItemID))
	p.WriteUint8(item.InventoryType)
	p.WriteInt16(item.Slot)
	p.WriteInt32(int32(ref.Amount))
	p.WriteUint8(item.Durability)
	p.WriteUint8(item.Enchant)
	p.WriteInt32(clampInt32(ref.Price))
	p.WriteInt16(int16(item.Soul1ID))
	p.WriteInt16(int16(item.Soul2ID))
	p.WriteInt16(int16(item.Soul3ID))
	p.WriteInt16(int16(item.Soul4ID))
	writeItemParams(p, item, true)
}

func writePrivateShopInfoItem(p *PacketOut, ref *tradeItemRef) {
	if ref == nil || ref.Item == nil || ref.Amount == -1 {
		p.WriteInt32(0)
		p.WriteInt32(-1)
		p.WriteUint8(0)
		p.WriteInt16(0)
		for i := 0; i < 4; i++ {
			p.WriteInt16(0)
		}
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteUint8(0)
		for i := 0; i < 5; i++ {
			p.WriteInt16(0)
			p.WriteInt16(0)
		}
		p.WriteUint8(0)
		p.WriteInt32(-1)
		p.WriteInt32(0)
		p.WriteInt16(0)
		return
	}
	item := ref.Item
	p.WriteInt32(int32(item.ItemID))
	p.WriteInt32(int32(ref.Amount))
	p.WriteUint8(item.Durability)
	p.WriteInt16(clampInt16(int32(itemUnitWeight(item))))
	p.WriteInt16(int16(item.Soul1ID))
	p.WriteInt16(int16(item.Soul2ID))
	p.WriteInt16(int16(item.Soul3ID))
	p.WriteInt16(int16(item.Soul4ID))
	p.WriteInt16(int16(item.Enchant))
	p.WriteInt16(0)
	p.WriteUint8(0)
	writeItemParams(p, item, false)
	p.WriteUint8(0)
	p.WriteInt32(clampInt32(ref.Price))
	p.WriteInt32(264156)
	p.WriteInt16(1)
}

func writePrivateShopOwnerSoldItem(p *PacketOut, ref *tradeItemRef) {
	if ref == nil || ref.Item == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(0)
		p.WriteInt32(-1)
		p.WriteInt32(-1)
		p.WriteInt16(0)
		p.WriteBytes(privateShopEmptyTradeStub())
		return
	}
	p.WriteInt32(int32(ref.Item.ItemID))
	p.WriteUint8(ref.Item.InventoryType)
	p.WriteInt16(ref.Item.Slot)
	p.WriteInt32(int32(ref.Amount))
	p.WriteInt32(int32(ref.TradeSlot))
	p.WriteInt16(0)
	p.WriteBytes(privateShopEmptyTradeStub())
}

func writePrivateShopBuyChatItem(p *PacketOut, ref *tradeItemRef) {
	if ref == nil || ref.Item == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(-1)
		p.WriteInt32(-1)
		p.WriteInt32(-1)
		p.WriteInt16(0)
		p.WriteBytes(privateShopEmptyTradeStub())
		return
	}
	p.WriteInt32(int32(ref.Item.ItemID))
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteInt32(int32(ref.Amount))
	p.WriteInt32(int32(ref.TradeSlot))
	p.WriteInt16(0)
	p.WriteBytes(privateShopEmptyTradeStub())
}

func writeItemParams(p *PacketOut, item *ItemRow, nilAsNegative bool) {
	if item == nil {
		empty := int16(0)
		if nilAsNegative {
			empty = -1
		}
		for i := 0; i < 5; i++ {
			p.WriteInt16(empty)
			p.WriteInt16(empty)
		}
		return
	}
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
}

func privateShopEmptyTradeStub() []byte {
	out := make([]byte, 28)
	for i := range out {
		out[i] = 0xFF
	}
	return out
}

func privateShopParticipants(shop *privateShop) []*Client {
	if shop == nil {
		return nil
	}
	out := make([]*Client, 0, len(shop.Joined)+1)
	if shop.Owner != nil {
		out = append(out, shop.Owner)
	}
	for _, c := range shop.Joined {
		out = append(out, c)
	}
	return out
}

func shopItemsCount(shop *privateShop) byte {
	if shop == nil {
		return 0
	}
	var count byte
	for _, ref := range shop.Items {
		if ref != nil && ref.Item != nil && ref.Amount != -1 {
			count++
		}
	}
	return count
}

func countItems(items []*ItemRow) int {
	count := 0
	for _, item := range items {
		if item != nil {
			count++
		}
	}
	return count
}
