package main

import (
	"log"
	"strings"
	"unicode"

	"asda2/shared/types"
)

// ---- Chat ----

const (
	announcementSender = "~Server~"
	dodgerBlueARGB     = 0xFF1E90FF
	maxChatMessageLen  = 200
)

func handleNormalChat(c *Client, p *PacketIn) {
	if c.Char == nil || c.Char.ChatBanned {
		return
	}
	// WCell ChatMgr.NormalChatRequest enters after the game-packet preamble,
	// rewinds two bytes, then reads a null-terminated ASCII string.
	p.Seek(26)
	msg := strings.TrimSpace(p.ReadCStringLocale(200, c.Locale))
	if msg == "" {
		return
	}
	log.Printf("[Chat] %s: %s", c.Char.Name, msg)

	for _, target := range World.AreaRecipients(c, true) {
		resp := NewPacket(NormalChatResponse)
		resp.WriteInt16(c.Char.SessionID)
		resp.WriteUint32(c.Char.AccID)
		resp.WriteAsdaStringLocale(c.Char.Name, 20, target.Locale)
		resp.WriteCStringLocale(msg, maxChatMessageLen, target.Locale)
		target.Send(resp)
	}
}

func sendWorldAnnouncementChat(message string) int {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return 0
	}

	targets := gameClientsSnapshot()
	for _, c := range targets {
		sendGlobalChatWithItemResponse(c, announcementSender, msg, dodgerBlueARGB)
	}
	return len(targets)
}

func sendSystemGlobalChatResponse(c *Client, sender string, message string) {
	sendGlobalChatWithItemResponse(c, sender, message, dodgerBlueARGB)
}

func sendGlobalChatWithItemResponse(c *Client, sender string, message string, color uint32) {
	if c == nil {
		return
	}
	p := NewPacket(GlobalChatWithItemResponse)
	p.WriteInt32(1)
	p.WriteUint32(color)
	p.WriteInt32(0)
	p.WriteAsdaStringLocale(sender, 20, c.Locale)
	p.WriteCStringLocale(message, maxChatMessageLen, c.Locale)
	c.Send(p)
}

func handleWishperChatRequest(c *Client, p *PacketIn) {
	if c.Char == nil || c.Char.ChatBanned {
		return
	}
	soulmate, targetName, msg := readWhisperRequest(c, p)
	if targetName == "" || msg == "" {
		return
	}
	if len(msg) > 100 {
		sendSystemGlobalChatResponse(c, announcementSender, "Whisper message is too long.")
		return
	}

	target := getClientByCharacterName(targetName)
	if target == nil || target.Char == nil {
		sendWhisperChatResponse(c, soulmate, 0, c.Char.AccID, c.Char.SessionID, targetName, msg)
		return
	}

	sendWhisperChatResponse(c, soulmate, 1, c.Char.AccID, c.Char.SessionID, c.Char.Name, msg)
	sendWhisperChatResponse(target, soulmate, 1, c.Char.AccID, target.Char.SessionID, c.Char.Name, msg)
}

func handleGlobalChatWithItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	msg := readGlobalChatWithItemMessage(c, p)
	if msg == "" {
		return
	}

	item, ok := useGlobalChatItem(c.Char)
	sendGlobalChatRemoveItem(c, ok, item)
	if !ok {
		sendSystemGlobalChatResponse(c, announcementSender, "You need a global chat item.")
		return
	}
	if c.Char.ChatBanned {
		sendSystemGlobalChatResponse(c, announcementSender, "You are chat banned.")
		return
	}
	if len([]rune(msg)) > maxChatMessageLen {
		sendSystemGlobalChatResponse(c, announcementSender, "Global chat message is too long.")
		return
	}

	log.Printf("[GlobalChat] %s: %s", c.Char.Name, msg)
	color := globalChatColor(c.Char)
	for _, target := range gameClientsSnapshot() {
		sendGlobalChatWithItemResponse(target, c.Char.Name, msg, color)
	}
}

func readGlobalChatWithItemMessage(c *Client, p *PacketIn) string {
	if p == nil {
		return ""
	}
	for _, offset := range []int{1, 0, 24, 26} {
		if offset < 0 || offset >= len(p.Data) {
			continue
		}
		p.Seek(offset)
		msg := strings.TrimSpace(p.ReadCStringLocale(maxChatMessageLen+1, c.Locale))
		if globalChatMessageLooksSane(msg) {
			return msg
		}
	}
	return ""
}

func globalChatMessageLooksSane(msg string) bool {
	printable := 0
	for _, r := range msg {
		if unicode.IsControl(r) {
			return false
		}
		if !unicode.IsSpace(r) {
			printable++
		}
	}
	return printable > 0
}

func useGlobalChatItem(chr *Character) (*ItemRow, bool) {
	for _, item := range inventoryItems(chr, types.InventoryShop) {
		if item == nil || item.Amount <= 0 {
			continue
		}
		if itemTemplateByID(item.ItemID).Category != types.ItemCategoryGlobalChat {
			continue
		}
		if err := removeCharacterItem(chr, item, 1); err != nil {
			log.Printf("[GlobalChat] consume item failed char=%q item=%d guid=%d: %v", chr.Name, item.ItemID, item.Guid, err)
			return item, false
		}
		return item, true
	}
	return nil, false
}

func sendGlobalChatRemoveItem(c *Client, success bool, item *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(GlobalChatRemoveItem)
	if success {
		p.WriteUint8(itemStatusOK)
	} else {
		p.WriteUint8(itemStatusFail)
	}
	p.WriteInt16(itemWeight(c.Char))
	writeGlobalChatRemovedItemInfo(p, item)
	c.Send(p)
}

func writeGlobalChatRemovedItemInfo(p *PacketOut, item *ItemRow) {
	itemID := int32(0)
	inv := byte(0)
	slot := int16(-1)
	amount := int32(0)
	weight := int16(0)
	if item != nil {
		itemID = int32(item.ItemID)
		inv = item.InventoryType
		slot = item.Slot
		amount = int32(item.Amount)
		weight = int16(itemUnitWeight(item))
	}
	p.WriteInt32(itemID)
	p.WriteUint8(inv)
	p.WriteInt16(slot)
	p.WriteInt16(0)
	p.WriteInt32(amount)
	p.WriteUint8(0)
	p.WriteInt16(weight)
	p.WriteInt16(-1)
	p.WriteInt16(-1)
	p.WriteInt16(-1)
	p.WriteInt16(-1)
	p.WriteUint8(0)
	p.WriteUint8(0)
	p.WriteInt16(0)
	p.WriteInt16(-1)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteInt16(-1)
	p.WriteInt16(-1)
	p.WriteUint8(0)
	p.WriteUint8(0)
	p.WriteInt16(0)
}

func globalChatColor(chr *Character) uint32 {
	if chr != nil && chr.GlobalChatColorDB != 0 {
		return uint32(chr.GlobalChatColorDB)
	}
	return dodgerBlueARGB
}
