package main

import (
	"encoding/binary"

	"asda2/shared/types"
)

// ---- Display Item ----

func handleDisplayItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	if len(p.Data) < 5 {
		return
	}

	targetSession := int16(binary.LittleEndian.Uint16(p.Data[0:]))
	inv := types.InventoryRegular
	if p.Data[2] == 1 {
		inv = types.InventoryShop
	}
	slot := int16(binary.LittleEndian.Uint16(p.Data[3:]))
	item := findItem(c.Char, inv, slot)
	if item == nil {
		return
	}

	if target := getClientBySessionID(targetSession); target != nil && target.Char != nil {
		sendItemDisplayed(c, target, item)
		return
	}
	for _, receiver := range World.AreaRecipients(c, true) {
		sendItemDisplayed(c, receiver, item)
	}
}

func sendItemDisplayed(sender *Client, receiver *Client, item *ItemRow) {
	if sender == nil || sender.Char == nil || receiver == nil || item == nil {
		return
	}
	p := NewPacket(ItemDisplayed)
	p.WriteUint32(sender.Char.AccID)
	p.WriteAsdaStringLocale(sender.Char.Name, 20, receiver.Locale)
	writeItemInfoToPacket(p, item, sender.Char, false)
	receiver.Send(p)
}
