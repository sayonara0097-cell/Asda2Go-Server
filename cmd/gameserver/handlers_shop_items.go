package main

import (
	"encoding/binary"

	"asda2/shared/types"
)

// ---- Shop Items ----

func handleUseShopItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendUpdateShopItemInfo(c, itemStatusFail, nil)
		return
	}
	slot, param, ok := readUseShopItemRequest(p.Data)
	if !ok {
		sendUpdateShopItemInfo(c, itemStatusFail, nil)
		return
	}
	item := findShopUseItem(c.Char, slot)
	if item == nil && len(p.Data) >= 2 {
		fallbackSlot := int16(binary.LittleEndian.Uint16(p.Data[0:]))
		if fallbackSlot != slot {
			item = findShopUseItem(c.Char, fallbackSlot)
		}
	}
	if item == nil {
		sendUpdateShopItemInfo(c, itemStatusFail, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendUpdateShopItemInfo(c, itemStatusFail, item)
		return
	}
	out := applyInventoryItemUse(c.Char, item, param)
	sendUpdateShopItemInfo(c, out.status, item)
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
	}
}

func handleCancelFunctionalItem(c *Client, p *PacketIn) {
	pkt := NewPacket(StopUseFunctionalItem)
	pkt.WriteUint8(itemStatusOK)
	c.Send(pkt)
}

func readUseShopItemRequest(data []byte) (int16, uint32, bool) {
	if len(data) < 2 {
		return 0, 0, false
	}
	slot := int16(binary.LittleEndian.Uint16(data[0:]))
	if len(data) >= 8 {
		slot = int16(binary.LittleEndian.Uint16(data[6:]))
	}
	param := uint32(0)
	if len(data) >= 259 {
		param = binary.LittleEndian.Uint32(data[255:])
	}
	return slot, param, true
}

func findShopUseItem(chr *Character, slot int16) *ItemRow {
	if item := findItem(chr, types.InventoryShop, slot); item != nil {
		return item
	}
	return findItem(chr, types.InventoryRegular, slot)
}

func sendUpdateShopItemInfo(c *Client, status byte, item *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UpdateShopItemInfo)
	p.WriteUint8(status)
	p.WriteInt16(c.Char.SessionID)
	writeItemInfoToPacket(p, item, c.Char, true)
	p.WriteInt16(itemWeight(c.Char))
	p.WriteInt32(0)
	p.WriteInt16(int16(maxWeight(c.Char)))
	c.Send(p)
}

func sendFunctionalShopItemUsed(c *Client, item *ItemRow, durationSecs int32) {
	if c == nil || c.Char == nil || item == nil {
		return
	}
	p := NewPacket(ShopItemUsed)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt32(int32(item.ItemID))
	p.WriteInt32(durationSecs)
	for _, viewer := range World.CharactersOnMap(c.Char.MapID) {
		if viewer != nil {
			viewer.Send(p)
		}
	}
}
