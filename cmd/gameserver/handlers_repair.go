package main

import "encoding/binary"

// ---- Repair ----

func handleRepairItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	var item *ItemRow
	if len(p.Data) >= 3 {
		inv := p.Data[0]
		slot := int16(binary.LittleEndian.Uint16(p.Data[1:]))
		item = findItem(c.Char, inv, slot)
	}
	status := itemStatusOK
	cost := int64(0)
	if item != nil {
		if itemLockedForTrading(c.Char, item) {
			sendRepairItemResponse(c, itemStatusFail, item)
			return
		}
		templ := itemTemplateByID(item.ItemID)
		maxDurability := templ.MaxDurability
		if maxDurability == 0 {
			maxDurability = 100
		}
		if item.Durability < maxDurability {
			cost = int64(maxDurability-item.Durability) * 2
		}
		if cost > c.Char.Gold {
			status = itemStatusFail
		} else {
			c.Char.Gold -= cost
			item.Durability = maxDurability
			if err := SaveItem(item); err != nil {
				status = itemStatusFail
			}
			_ = SaveCharacter(c.Char)
		}
	} else {
		for _, it := range c.Char.Items {
			if it == nil {
				continue
			}
			templ := itemTemplateByID(it.ItemID)
			maxDurability := templ.MaxDurability
			if maxDurability == 0 {
				continue
			}
			if it.Durability < maxDurability {
				cost += int64(maxDurability-it.Durability) * 2
			}
		}
		if cost > c.Char.Gold {
			status = itemStatusFail
		} else {
			c.Char.Gold -= cost
			for _, it := range c.Char.Items {
				if itemLockedForTrading(c.Char, it) {
					continue
				}
				templ := itemTemplateByID(it.ItemID)
				if templ.MaxDurability > 0 {
					it.Durability = templ.MaxDurability
					_ = SaveItem(it)
				}
			}
			_ = SaveCharacter(c.Char)
		}
	}
	sendRepairItemResponse(c, status, item)
}

func sendRepairItemResponse(c *Client, status byte, item *ItemRow) {
	p := NewPacket(RepairItemResponse)
	p.WriteUint8(status)
	p.WriteInt32(clampInt32(c.Char.Gold))
	writeItemInfoToPacket(p, item, c.Char, false)
	c.Send(p)
}
