package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"asda2/shared/crypt"
	charstats "asda2/shared/stats"
	"asda2/shared/types"
)

const (
	enterGameMinDamageOffset      = 88
	enterGameMaxDamageOffset      = 90
	enterGameMinMagicDamageOffset = 92
	enterGameMaxMagicDamageOffset = 94
	enterGameMagicDefenceOffset   = 96
	enterGameDefenceMinOffset     = 98
	enterGameDefenceMaxOffset     = 100
)

func TestEnterGameResponseIncludesPhysicalCombatStats(t *testing.T) {
	types.SetItemTemplates([]types.ItemTemplate{
		{ItemID: 100, Kind: types.ItemKindWeapon, Category: types.ItemCategoryOneHandedSword, EquipmentSlot: 9},
		{ItemID: 200, SowelBonusType: charstats.SowelBonusWeaponAtk, SowelBonusValue: 10},
	})
	t.Cleanup(func() { types.SetItemTemplates(nil) })

	payload := captureEnterGamePayload(t, &Character{
		Name:         "StatHero",
		CharNum:      10,
		Class:        byte(types.Asda2ClassOHS),
		Level:        10,
		BaseStrength: 100,
		BaseSpirit:   25,
		MaxHP:        100,
		HP:           100,
		MaxMP:        50,
		MP:           50,
		Items: []*ItemRow{
			{
				ItemID:        100,
				InventoryType: types.InventoryEquipment,
				Slot:          9,
				Soul1ID:       200,
				Amount:        1,
			},
			{
				ItemID:        300,
				InventoryType: types.InventoryEquipment,
				Slot:          1,
				Param1Type:    charstats.ItemBonusDefence,
				Param1Value:   12,
				Amount:        1,
			},
		},
	})

	minDamage := int16At(payload, enterGameMinDamageOffset)
	maxDamage := int16At(payload, enterGameMaxDamageOffset)
	magicDefence := int16At(payload, enterGameMagicDefenceOffset)
	defenceMin := int16At(payload, enterGameDefenceMinOffset)
	defenceMax := int16At(payload, enterGameDefenceMaxOffset)
	if minDamage <= 0 || maxDamage < minDamage {
		t.Fatalf("enter-game physical damage = %d-%d, want non-zero combat stats", minDamage, maxDamage)
	}
	if magicDefence <= 0 {
		t.Fatalf("enter-game magic defence = %d, want non-zero stat", magicDefence)
	}
	if defenceMin <= 0 || defenceMax < defenceMin {
		t.Fatalf("enter-game defence = %d-%d, want non-zero defence range", defenceMin, defenceMax)
	}
}

func TestEnterGameResponseIncludesMageMagicAttack(t *testing.T) {
	types.SetItemTemplates([]types.ItemTemplate{
		{ItemID: 101, Kind: types.ItemKindWeapon, Category: types.ItemCategoryStaff, EquipmentSlot: 9},
		{ItemID: 201, SowelBonusType: charstats.SowelBonusWeaponMAtk, SowelBonusValue: 5},
	})
	t.Cleanup(func() { types.SetItemTemplates(nil) })

	payload := captureEnterGamePayload(t, &Character{
		Name:          "MagicHero",
		CharNum:       10,
		Class:         byte(types.Asda2ClassAttackMage),
		Level:         10,
		BaseIntellect: 100,
		BaseSpirit:    25,
		MaxHP:         100,
		HP:            100,
		MaxMP:         50,
		MP:            50,
		Items: []*ItemRow{{
			ItemID:        101,
			InventoryType: types.InventoryEquipment,
			Slot:          9,
			Soul1ID:       201,
			Amount:        1,
		}},
	})

	minMagicDamage := int16At(payload, enterGameMinMagicDamageOffset)
	maxMagicDamage := int16At(payload, enterGameMaxMagicDamageOffset)
	if minMagicDamage <= 0 || maxMagicDamage < minMagicDamage {
		t.Fatalf("enter-game magic damage = %d-%d, want non-zero mage magic stats", minMagicDamage, maxMagicDamage)
	}
}

func captureEnterGamePayload(t *testing.T, chr *Character) []byte {
	t.Helper()
	server, clientConn := net.Pipe()
	defer server.Close()
	defer clientConn.Close()

	c := NewClient(server, ServerKindLogin, nil, nil)
	c.Locale = crypt.LocaleAny
	c.Account = &Account{ID: 123}
	c.Char = chr

	done := make(chan struct{})
	go func() {
		sendEnterGameResponse(c)
		close(done)
	}()
	raw := readRawPacket(t, clientConn)
	<-done

	op, payload := decodeServerPayload(t, raw)
	if op != EnterGameRespose {
		t.Fatalf("opcode = %d, want EnterGameRespose", op)
	}
	return payload
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
