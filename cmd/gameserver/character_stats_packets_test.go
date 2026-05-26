package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"asda2/shared/crypt"
	"asda2/shared/types"
)

func TestUpdateStatsPacketIncludesCombatStats(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 100, Kind: types.ItemKindWeapon, Category: types.ItemCategoryOneHandedSword, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 200, SowelBonusType: sowelBonusWeaponAtk, SowelBonusValue: 10},
	})
	defer setItemTemplates(nil)

	server, clientConn := net.Pipe()
	defer server.Close()
	defer clientConn.Close()

	c := NewClient(server, ServerKindGame, nil, nil)
	c.Locale = crypt.LocaleAny
	c.Char = &Character{
		Class:        byte(types.Asda2ClassOHS),
		BaseStrength: 100,
		BaseSpirit:   25,
		MaxHP:        100,
		HP:           100,
		MaxMP:        50,
		MP:           50,
		Items: []*ItemRow{{
			ItemID:        100,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       200,
			Amount:        1,
		}},
	}

	done := make(chan struct{})
	go func() {
		sendUpdateStats(c)
		close(done)
	}()
	raw := readRawPacket(t, clientConn)
	<-done

	op, payload := decodeServerPayload(t, raw)
	if op != UpdateStats {
		t.Fatalf("opcode = %d, want UpdateStats", op)
	}
	minDamage := int16At(payload, 12)
	maxDamage := int16At(payload, 14)
	magicDefence := int16At(payload, 20)
	if minDamage <= 0 || maxDamage < minDamage {
		t.Fatalf("update stats damage = %d-%d, want non-zero combat stats", minDamage, maxDamage)
	}
	if magicDefence <= 0 {
		t.Fatalf("update stats magic defence = %d, want non-zero stat", magicDefence)
	}
}

func TestSkillLearnedPacketIncludesCombatStats(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 100, Kind: types.ItemKindWeapon, Category: types.ItemCategoryOneHandedSword, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 200, SowelBonusType: sowelBonusWeaponAtk, SowelBonusValue: 10},
	})
	defer setItemTemplates(nil)

	server, clientConn := net.Pipe()
	defer server.Close()
	defer clientConn.Close()

	c := NewClient(server, ServerKindGame, nil, nil)
	c.Locale = crypt.LocaleAny
	c.Char = &Character{
		Class:        byte(types.Asda2ClassOHS),
		BaseStrength: 100,
		BaseSpirit:   25,
		MaxHP:        100,
		HP:           100,
		MaxMP:        50,
		MP:           50,
		Items: []*ItemRow{{
			ItemID:        100,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       200,
			Amount:        1,
		}},
	}

	done := make(chan struct{})
	go func() {
		sendSkillLearnedResponse(c, skillLearnOK, 501, 1)
		close(done)
	}()
	raw := readRawPacket(t, clientConn)
	<-done

	op, payload := decodeServerPayload(t, raw)
	if op != SkillLearned {
		t.Fatalf("opcode = %d, want SkillLearned", op)
	}
	minDamage := int16At(payload, 59)
	maxDamage := int16At(payload, 61)
	magicDefence := int16At(payload, 67)
	if minDamage <= 0 || maxDamage < minDamage {
		t.Fatalf("skill learned damage = %d-%d, want non-zero combat stats", minDamage, maxDamage)
	}
	if magicDefence <= 0 {
		t.Fatalf("skill learned magic defence = %d, want non-zero stat from spirit/base rules", magicDefence)
	}
}

func TestLevelUpPacketIncludesCombatStats(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 101, Kind: types.ItemKindWeapon, Category: types.ItemCategoryStaff, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 201, SowelBonusType: sowelBonusWeaponMAtk, SowelBonusValue: 10},
	})
	defer setItemTemplates(nil)

	var raw []byte
	c := NewClient(nil, ServerKindGame, nil, func(_ *Client, p *PacketOut) {
		raw = p.Finalize(crypt.LocaleAny)
	})
	c.Char = &Character{
		SessionID:     42,
		Class:         byte(types.Asda2ClassAttackMage),
		Level:         2,
		BaseIntellect: 100,
		BaseSpirit:    25,
		MaxHP:         100,
		HP:            100,
		MaxMP:         50,
		MP:            50,
		Items: []*ItemRow{{
			ItemID:        101,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       201,
			Amount:        1,
		}},
	}

	sendLevelUpResponse(c)
	op, payload := decodeServerPayload(t, raw)
	if op != LvlUp {
		t.Fatalf("opcode = %d, want LvlUp", op)
	}
	minMagicDamage := int16At(payload, 65)
	maxMagicDamage := int16At(payload, 67)
	magicDefence := int32At(payload, 69)
	if minMagicDamage <= 0 || maxMagicDamage < minMagicDamage {
		t.Fatalf("level-up magic damage = %d-%d, want non-zero magic stats", minMagicDamage, maxMagicDamage)
	}
	if magicDefence <= 0 {
		t.Fatalf("level-up magic defence = %d, want non-zero stat", magicDefence)
	}
}

func readRawPacket(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	header := make([]byte, 3)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("read header: %v", err)
	}
	length := int(binary.LittleEndian.Uint16(header[1:]))
	rest := make([]byte, length-3)
	if _, err := io.ReadFull(conn, rest); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return append(header, rest...)
}

func decodeServerPayload(t *testing.T, raw []byte) (Opcode, []byte) {
	t.Helper()
	if len(raw) < 11 {
		t.Fatalf("packet too short: %d", len(raw))
	}
	buf := append([]byte(nil), raw...)
	crypt.XorData(buf, 3, len(buf)-4, crypt.LocaleAny)
	op := Opcode(binary.LittleEndian.Uint16(buf[8:10]))
	payloadEnd := len(buf) - 7
	if payloadEnd < 10 {
		t.Fatalf("packet payload end before header: %d", payloadEnd)
	}
	return op, buf[10:payloadEnd]
}

func int16At(payload []byte, offset int) int16 {
	return int16(binary.LittleEndian.Uint16(payload[offset:]))
}

func int32At(payload []byte, offset int) int32 {
	return int32(binary.LittleEndian.Uint32(payload[offset:]))
}
