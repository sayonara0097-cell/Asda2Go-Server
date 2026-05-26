package types

import (
	"time"

	"asda2/shared/packet"
)

type ItemRow struct {
	Guid              int64
	OwnerID           uint32
	ItemID            int
	InventoryType     byte
	Slot              int16
	CreatorID         int
	Durability        byte
	Duration          int
	IsSoulBound       bool
	IsAuctioned       bool
	MailID            int
	Soul1ID           int
	Soul2ID           int
	Soul3ID           int
	Soul4ID           int
	Enchant           byte
	EnchantResetCount byte
	Param1Type        int16
	Param1Value       int16
	Param2Type        int16
	Param2Value       int16
	Param3Type        int16
	Param3Value       uint16
	Param4Type        int16
	Param4Value       int16
	Param5Type        int16
	Param5Value       int16
	IsStackable       bool
	CreatorEntityID   int64
	Weight            uint16
	SealCount         byte
	Amount            int
	AuctionPrice      int
	AuctionEndTime    time.Time
	OwnerName         string
	IsCrafted         bool
}

type FastSlotRow struct {
	Guid          int64
	OwnerID       uint32
	PanelNum      byte
	PanelSlot     byte
	InventoryType byte
	ItemOrSkillID int
	InventorySlot int16
	SrcInfo       int16
	Amount        int
}

const goldItemID = 20551

func ownedItem(item *ItemRow) bool {
	return item != nil && !item.IsAuctioned && item.MailID == 0
}

func ItemsInInventory(items []*ItemRow, inv byte) []*ItemRow {
	out := make([]*ItemRow, 0)
	for _, item := range items {
		if ownedItem(item) && item.InventoryType == inv {
			out = append(out, item)
		}
	}
	return out
}

func ItemInInventorySlot(items []*ItemRow, inv byte, slot int16) *ItemRow {
	for _, item := range items {
		if ownedItem(item) && item.InventoryType == inv && item.Slot == slot {
			return item
		}
	}
	return nil
}

func ChunkItems(items []*ItemRow, size int) [][]*ItemRow {
	if size <= 0 {
		return nil
	}
	chunks := make([][]*ItemRow, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func itemAmountForPacket(item *ItemRow, owner *Character, setAmountTo0WhenDeleted bool) int32 {
	if item == nil {
		return -1
	}
	if item.ItemID == goldItemID {
		if owner != nil {
			return int32(owner.Gold)
		}
		return int32(item.Amount)
	}
	if item.Amount <= 0 {
		if setAmountTo0WhenDeleted {
			return 0
		}
		return -1
	}
	if ItemIsStackable(item) {
		return int32(item.Amount)
	}
	return 0
}

func ItemIsStackable(item *ItemRow) bool {
	if item == nil {
		return false
	}
	if item.IsStackable {
		return true
	}
	templ := ItemTemplateByID(item.ItemID)
	if templ.IsStackable || templ.MaxStack > 1 {
		return true
	}
	// Some legacy rows were saved with IsStackable=false even though they
	// already carry stack amounts. Preserve that runtime state for packets and
	// split/consume operations until template data is fully seeded.
	return item.Amount > 1 && templ.Kind != ItemKindWeapon && templ.Kind != ItemKindArmor &&
		templ.Kind != ItemKindAvatar && templ.Kind != ItemKindAccessory
}

func ApplyItemTemplateToRow(item *ItemRow) {
	if item == nil || item.ItemID <= 0 {
		return
	}
	templ := ItemTemplateByID(item.ItemID)
	stackable := templ.IsStackable || templ.MaxStack > 1 || ItemIsStackable(item)
	if stackable {
		item.IsStackable = true
		if item.ItemID != goldItemID && item.Amount <= 0 {
			item.Amount = 1
		}
	}
	if item.Weight == 0 || templ.Kind == ItemKindCurrency {
		item.Weight = templ.Weight
	}
}

// writeItemInfoToPacket mirrors Asda2InventoryHandler.WriteItemInfoToPacket in
// the WCell reference. The avatar-template special case for sowel bonus values
// needs item templates; until those are ported, stored parameter values are sent.
func WriteItemInfoToPacket(p *packet.PacketOut, item *ItemRow, owner *Character, setAmountTo0WhenDeleted bool) {
	if item == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt32(-1)
		p.WriteUint8(0xFF)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteUint8(0xFF)
		p.WriteInt16(0)
		p.WriteUint8(0xFF)
		p.WriteUint8(0xFF)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		p.WriteUint8(8)
		p.WriteUint8(0xFF)
		p.WriteInt32(6)
		p.WriteInt16(7)
		return
	}

	p.WriteInt32(int32(item.ItemID))
	p.WriteUint8(item.InventoryType)
	p.WriteInt16(item.Slot)
	p.WriteInt16(0)
	p.WriteInt32(itemAmountForPacket(item, owner, setAmountTo0WhenDeleted))
	p.WriteUint8(item.Durability)
	p.WriteUint16(item.Weight)
	p.WriteInt16(int16(item.Soul1ID))
	p.WriteInt16(int16(item.Soul2ID))
	p.WriteInt16(int16(item.Soul3ID))
	p.WriteInt16(int16(item.Soul4ID))
	p.WriteUint8(item.Enchant)
	p.WriteInt16(0)
	p.WriteUint8(item.EnchantResetCount)
	p.WriteUint8(item.SealCount)
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
	p.WriteUint8(8)
	if item.IsSoulBound {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteInt32(6)
	p.WriteInt16(7)
}

func FastSlotAt(slots []*FastSlotRow, panel byte, panelSlot byte) *FastSlotRow {
	for _, slot := range slots {
		if slot != nil && slot.PanelNum == panel && slot.PanelSlot == panelSlot {
			return slot
		}
	}
	return nil
}

func SendAllFastItemSlotsInfo(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	for panel := byte(0); panel <= 5; panel++ {
		p := packet.NewPacket(packet.FastItemSlotsInfo)
		p.WriteUint8(panel)
		for panelSlot := byte(0); panelSlot <= 12; panelSlot++ {
			slot := FastSlotAt(c.Char.FastSlots, panel, panelSlot)
			if slot == nil {
				p.WriteUint8(0)
				p.WriteUint8(0)
				p.WriteInt16(-1)
				p.WriteInt32(0)
				p.WriteInt16(-1)
				continue
			}
			p.WriteUint8(byte(slot.SrcInfo))
			p.WriteUint8(slot.InventoryType)
			p.WriteInt16(slot.InventorySlot)
			p.WriteInt32(int32(slot.Amount))
			p.WriteInt16(int16(slot.ItemOrSkillID))
		}
		c.Send(p)
	}
}
