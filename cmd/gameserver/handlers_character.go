package main

import (
	"encoding/binary"
	"log"
)

var emoteCounter byte

var characterFullInfoStub = []byte{
	199, 78, 0, 0, 211, 78, 0, 0, 212, 78, 0, 0,
	134, 78, 0, 0, 135, 78, 0, 0, 190, 78, 0, 0,
	189, 78, 0, 0, 172, 78, 0, 0, 191, 78, 0, 0,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	5, 10, 0, 0, 199, 78, 211, 78, 212, 78, 134, 78,
	135, 78, 190, 78, 189, 78, 172, 78, 191, 78,
	0xFF, 0xFF, 0xFF, 0xFF, 54, 2, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0xFF, 0xFF, 0xFF, 0xFF, 0, 0,
}

// ---- Character ----

func handleGetCharecterInfo(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	target := c.Char
	for _, sessID := range characterInfoSessionCandidates(p) {
		if client := getClientBySessionID(int16(sessID)); client != nil && client.Char != nil {
			target = client.Char
			break
		}
	}
	sendCharacterFullInfoResponse(c, target)
	sendCharacterRegularEquipmentInfoResponse(c, target)
}

func characterInfoSessionCandidates(p *PacketIn) []uint16 {
	if p == nil {
		return nil
	}
	seen := make(map[uint16]struct{})
	out := make([]uint16, 0, 4)
	add := func(offset int) {
		if offset < 0 || offset+2 > len(p.Data) {
			return
		}
		value := binary.LittleEndian.Uint16(p.Data[offset:])
		if value == 0 {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	add(0)
	add(24)
	for offset := 2; offset+2 <= len(p.Data); offset += 2 {
		add(offset)
	}
	return out
}

func sendCharacterFullInfoResponse(c *Client, target *Character) {
	if c == nil || target == nil {
		return
	}
	p := NewPacket(CharacterFullInfo)
	p.WriteUint8(target.Level)
	p.WriteUint8(target.ProfessionLevel)
	p.WriteUint8(target.Class)
	p.WriteAsdaStringLocale("", 17, c.Locale)
	p.WriteBytes(characterFullInfoStub)
	p.WriteInt32(int32(target.AccID))
	p.WriteUint8(3)
	for slot := int16(11); slot < 20; slot++ {
		writeItemInfoToPacket(p, itemInInventorySlot(target.Items, 3, slot), target, false)
	}
	c.Send(p)
}

func sendCharacterRegularEquipmentInfoResponse(c *Client, target *Character) {
	if c == nil || target == nil {
		return
	}
	p := NewPacket(CharacterRegularEquipmentInfo)
	p.WriteInt32(int32(target.AccID))
	for slot := int16(0); slot < 11; slot++ {
		writeItemInfoToPacket(p, itemInInventorySlot(target.Items, 3, slot), target, false)
	}
	c.Send(p)
}

func handleSettingsFlags(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	flags, avatarMask, ok := readSettingsFlagsRequest(p.Data)
	if !ok {
		return
	}
	copy(c.Char.SettingsFlags[:], flags[:])
	c.Char.AvatarMask = avatarMask
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Settings] save %q: %v", c.Char.Name, err)
	}
}

func handleChangeFaceOrHair(c *Client, p *PacketIn) {
	// TODO: update character appearance, broadcast FaceOrHairChanged
}

func handleEmote(c *Client, p *PacketIn) {
	// Mirrors Asda2CharacterHandler.EmoteRequest.
	if c.Char == nil || p.Remaining() < 2 {
		return
	}

	emote := p.ReadInt16()
	switch emote {
	case 108, 109, 131:
		return
	}

	action := byte(1)
	a := float32(0.0596617)
	b := float32(-0.9982219)
	if p.Remaining() >= 9 {
		action = p.ReadUint8()
		a = p.ReadFloat32()
		b = p.ReadFloat32()
	}
	sendEmoteResponse(c, emote, action, a, b)
}

func sendEmoteResponse(c *Client, emote int16, action byte, a float32, b float32) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(EmoteResponse)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(emote)
	p.WriteUint8(action)
	p.WriteFloat32(a)
	p.WriteFloat32(b)
	p.WriteUint8(emoteCounter)
	emoteCounter++
	c.SendToArea(p)
}

func readSettingsFlagsRequest(data []byte) ([16]byte, int32, bool) {
	var flags [16]byte
	if len(data) < 16 {
		return flags, 0, false
	}
	offset := 0
	// WCell Asda2CharacterHandler.SettingsFlagsRequest uses Position -= 12,
	// which maps to offset 16 in our stripped game payload.
	if len(data) >= 36 {
		offset = 16
	}
	copy(flags[:], data[offset:offset+16])
	var avatarMask int32
	if len(data) >= offset+20 {
		avatarMask = int32(uint32(data[offset+16]) |
			uint32(data[offset+17])<<8 |
			uint32(data[offset+18])<<16 |
			uint32(data[offset+19])<<24)
	}
	return flags, avatarMask, true
}
