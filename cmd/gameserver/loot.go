package main

import (
	"encoding/binary"
	"log"
	"math/rand"
)

const (
	goldLootItemID        = int32(20551)
	defaultMonsterGoldMin = int32(3)
	defaultMonsterGoldMax = int32(7)
)

type lootKey struct {
	x int16
	y int16
}

type LootItem struct {
	ItemID        int32
	Amount        int32
	X             int16
	Y             int16
	OwnerSession  int16
	OwnerAccount  uint32
	SourceEntryID uint16
	InventoryItem *ItemRow
}

func (m *GameMap) dropMonsterLoot(owner *Client, monster *Monster) {
	if m == nil || owner == nil || owner.Char == nil || monster == nil {
		return
	}
	drops := rollMonsterDrops(monster.EntryID)
	if len(drops) == 0 {
		drops = []LootItem{fallbackMonsterGoldLoot(monster.EntryID)}
	}
	for i := range drops {
		loot := drops[i]
		loot.X = monster.LocalX
		loot.Y = monster.LocalY
		loot.OwnerSession = owner.Char.SessionID
		loot.OwnerAccount = owner.Char.AccID
		loot.SourceEntryID = monster.EntryID
		m.addLoot(&loot)
		log.Printf("[Loot] monster session=%d entry=%d dropped item=%d amount=%d at %d,%d for %q",
			monster.SessionID, monster.EntryID, loot.ItemID, loot.Amount, loot.X, loot.Y, owner.Char.Name)
	}
}

func rollMonsterDrops(entryID uint16) []LootItem {
	monsterDrops.RLock()
	table := append([]MonsterDropRow(nil), monsterDrops.byEntry[entryID]...)
	monsterDrops.RUnlock()
	if len(table) == 0 {
		return nil
	}

	out := make([]LootItem, 0, 2)
	for _, drop := range table {
		if drop.Chance <= 0 || rand.Float64()*100 >= drop.Chance {
			continue
		}
		out = append(out, LootItem{
			ItemID: drop.ItemID,
			Amount: randomDropAmount(drop.MinAmount, drop.MaxAmount),
		})
	}
	return out
}

func randomDropAmount(minAmount, maxAmount int32) int32 {
	if minAmount <= 0 {
		minAmount = 1
	}
	if maxAmount < minAmount {
		maxAmount = minAmount
	}
	if maxAmount == minAmount {
		return minAmount
	}
	return minAmount + rand.Int31n(maxAmount-minAmount+1)
}

func fallbackMonsterGoldLoot(entryID uint16) LootItem {
	amount := defaultMonsterGoldMin + int32(entryID)%((defaultMonsterGoldMax-defaultMonsterGoldMin)+1)
	return LootItem{ItemID: goldLootItemID, Amount: amount}
}

func (m *GameMap) addLoot(loot *LootItem) {
	if m == nil || loot == nil {
		return
	}
	m.mu.Lock()
	loot.X, loot.Y = m.freeLootCoordinatesLocked(loot.X, loot.Y)
	m.loots[lootKey{x: loot.X, y: loot.Y}] = loot
	m.mu.Unlock()
	m.broadcastLootVisible(loot)
}

func (m *GameMap) freeLootCoordinatesLocked(x int16, y int16) (int16, int16) {
	if _, ok := m.loots[lootKey{x: x, y: y}]; !ok {
		return x, y
	}
	for radius := int16(1); radius <= 3; radius++ {
		candidates := [][2]int16{
			{x + radius, y},
			{x - radius, y},
			{x, y + radius},
			{x, y - radius},
			{x + radius, y + radius},
			{x - radius, y - radius},
			{x + radius, y - radius},
			{x - radius, y + radius},
		}
		for _, candidate := range candidates {
			key := lootKey{x: candidate[0], y: candidate[1]}
			if _, ok := m.loots[key]; !ok {
				return key.x, key.y
			}
		}
	}
	return x, y
}

func (m *GameMap) takeLootAt(x int16, y int16, looter *Client) (*LootItem, bool) {
	if m == nil || looter == nil || looter.Char == nil {
		return nil, false
	}
	key := lootKey{x: x, y: y}
	m.mu.Lock()
	loot, ok := m.loots[key]
	if ok {
		delete(m.loots, key)
	}
	m.mu.Unlock()
	if !ok {
		return nil, false
	}
	m.broadcastLootRemoved(loot)
	return loot, true
}

func (m *GameMap) pickupCoordinatesFromPayload(data []byte) (int16, int16, bool) {
	if m == nil || len(data) < 4 {
		return 0, 0, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for offset := 0; offset <= len(data)-4; offset++ {
		x := int16(binary.LittleEndian.Uint16(data[offset:]))
		y := int16(binary.LittleEndian.Uint16(data[offset+2:]))
		if _, ok := m.loots[lootKey{x: x, y: y}]; ok {
			return x, y, true
		}
		if _, ok := m.loots[lootKey{x: y, y: x}]; ok {
			return y, x, true
		}
	}
	return 0, 0, false
}

func (m *GameMap) broadcastLootVisible(loot *LootItem) {
	for _, c := range m.Characters() {
		sendLootVisible(c, loot)
	}
}

func (m *GameMap) broadcastLootRemoved(loot *LootItem) {
	for _, c := range m.Characters() {
		sendLootRemoved(c, loot)
	}
}

func sendLootVisible(c *Client, loot *LootItem) {
	if c == nil || c.Char == nil || loot == nil {
		return
	}
	p := NewPacket(ItemDroped)
	p.WriteInt32(loot.ItemID)
	p.WriteInt16(loot.X)
	p.WriteInt16(loot.Y)
	if loot.ItemID == goldLootItemID {
		p.WriteInt32(loot.Amount)
	} else {
		p.WriteInt32(int32(loot.OwnerSession))
	}
	p.WriteUint8(10)
	c.Send(p)
}

func sendLootRemoved(c *Client, loot *LootItem) {
	if c == nil || loot == nil {
		return
	}
	p := NewPacket(RemoveItemFromWorld)
	p.WriteInt16(loot.X)
	p.WriteInt16(loot.Y)
	p.WriteInt32(loot.ItemID)
	c.Send(p)
}

func sendLootPickupResponse(c *Client, status byte, loot *LootItem) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ItemPickuped)
	p.WriteUint8(status)
	p.WriteInt16(c.Char.SessionID)
	writeLootItemInfoToPacket(p, loot)
	p.WriteInt16(0)
	c.Send(p)
}

func writeLootItemInfoToPacket(p *PacketOut, loot *LootItem) {
	if loot == nil {
		writeItemInfoToPacket(p, nil, nil, false)
		return
	}
	if loot.InventoryItem != nil {
		writeItemInfoToPacket(p, loot.InventoryItem, nil, false)
		return
	}
	p.WriteInt32(loot.ItemID)
	p.WriteUint8(0) // regular inventory
	p.WriteInt16(-1)
	p.WriteInt16(0)
	p.WriteInt32(loot.Amount)
	p.WriteUint8(0)
	p.WriteInt16(0)
	p.WriteInt16(0)
	p.WriteInt16(0)
	p.WriteInt16(0)
	p.WriteInt16(0)
	p.WriteUint8(0)
	p.WriteInt16(0)
	p.WriteUint8(0)
	p.WriteUint8(0)
	for i := 0; i < 5; i++ {
		p.WriteInt16(0)
		p.WriteInt16(0)
	}
	p.WriteUint8(8)
	p.WriteUint8(0)
	p.WriteInt32(6)
	p.WriteInt16(7)
}
