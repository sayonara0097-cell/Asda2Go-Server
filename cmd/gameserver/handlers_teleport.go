package main

import (
	"log"
	"strings"
	"unicode/utf8"

	"asda2/shared/types"
)

const (
	teleportStatusServerDown byte = 0
	teleportStatusOK         byte = 1
	teleportStatusNotGold    byte = 12

	locationSavedFail byte = 0
	locationSavedOK   byte = 1
)

type teleportCrystalDestination struct {
	mapID uint16
	x     float32
	y     float32
	price int64
}

type teleportCrystalOptionSet struct {
	zeroBased []teleportCrystalDestination
	oneBased  []teleportCrystalDestination
}

var teleportCrystalDestinations = map[byte]teleportCrystalDestination{
	1:  {mapID: types.MapIDRainRiver, x: 1295, y: 1235, price: 0},
	3:  {mapID: types.MapIDAlpia, x: 3117, y: 3389, price: 0},
	0:  {mapID: types.MapIDSilaris, x: 393, y: 397, price: 3000},
	7:  {mapID: types.MapIDFlamio, x: 7135, y: 7188, price: 10000},
	5:  {mapID: types.MapIDAquaton, x: 5394, y: 5342, price: 15000},
	25: {mapID: types.MapIDDesolatedMarsh, x: 25274, y: 25326, price: 5000},
	23: {mapID: types.MapIDIceQuarry, x: 23253, y: 23304, price: 15000},
	2:  {mapID: types.MapIDConquestLand, x: 2303, y: 2309, price: 1500},
	6:  {mapID: types.MapIDSunnyCoast, x: 6365, y: 6110, price: 1500},
	13: {mapID: types.MapIDFlabis, x: 13208, y: 13388, price: 5000},
	24: {mapID: types.MapIDBurnedoutForest, x: 24318, y: 24310, price: 5000},
}

var teleportCrystalOptionsByMap = map[uint16]teleportCrystalOptionSet{
	types.MapIDSilaris: {
		zeroBased: []teleportCrystalDestination{
			{mapID: types.MapIDRainRiver, x: 1295, y: 1235, price: 0},
			{mapID: types.MapIDConquestLand, x: 2303, y: 2309, price: 1500},
			{mapID: types.MapIDSunnyCoast, x: 6365, y: 6110, price: 1500},
			{mapID: types.MapIDFlamio, x: 7135, y: 7188, price: 10000},
			{mapID: types.MapIDAquaton, x: 5394, y: 5342, price: 15000},
		},
	},
}

func init() {
	for mapID, set := range teleportCrystalOptionsByMap {
		if len(set.oneBased) == 0 {
			oneBased := make([]teleportCrystalDestination, len(set.zeroBased)+1)
			copy(oneBased[1:], set.zeroBased)
			set.oneBased = oneBased
			teleportCrystalOptionsByMap[mapID] = set
		}
	}
}

// ---- Teleport ----

func handleTeleportByCristal(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 1 {
		return
	}
	id, offset, dest, ok := readTeleportCrystalDestination(p.Data, c.Char.MapID)
	if !ok {
		log.Printf("[Teleport] crystal request from %q has no known destination payloadLen=%d data=% X", c.Char.Name, len(p.Data), p.Data)
		sendTeleportedByCristal(c, teleportStatusServerDown, c.Char.MapID, int16(asda2X(c.Char.X, c.Char.MapID)), int16(asda2Y(c.Char.Y, c.Char.MapID)))
		return
	}
	if World.GetMap(dest.mapID) == nil {
		log.Printf("[Teleport] crystal request from %q unknown id=%d payloadLen=%d data=% X", c.Char.Name, id, len(p.Data), p.Data)
		sendTeleportedByCristal(c, teleportStatusServerDown, c.Char.MapID, int16(asda2X(c.Char.X, c.Char.MapID)), int16(asda2Y(c.Char.Y, c.Char.MapID)))
		return
	}
	if dest.price > c.Char.Gold {
		log.Printf("[Teleport] crystal request from %q id=%d failed gold=%d price=%d", c.Char.Name, id, c.Char.Gold, dest.price)
		sendTeleportedByCristal(c, teleportStatusNotGold, c.Char.MapID, int16(asda2X(c.Char.X, c.Char.MapID)), int16(asda2Y(c.Char.Y, c.Char.MapID)))
		return
	}
	log.Printf("[Teleport] crystal request from %q id=%d offset=%d to map=%d local=%.0f,%.0f price=%d", c.Char.Name, id, offset, dest.mapID, asda2X(dest.x, dest.mapID), asda2Y(dest.y, dest.mapID), dest.price)
	c.Char.Gold -= dest.price
	teleportClientToWorld(c, dest.mapID, dest.x, dest.y)
}

func handleUseTeleportScroll(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	if p.Remaining() >= 2 {
		p.Skip(2)
	}
	name := strings.TrimSpace(p.ReadAsdaStringLocale(20, c.Locale))
	if name == "" || strings.EqualFold(name, c.Char.Name) {
		return
	}
	target := getClientByCharacterName(name)
	if target == nil || target.Char == nil {
		return
	}
	if isFightingMap(c.Char.MapID) || isFightingMap(target.Char.MapID) {
		return
	}
	scroll := findTeleportScroll(c.Char)
	if scroll == nil {
		return
	}
	if err := removeCharacterItem(c.Char, scroll, 1); err != nil {
		log.Printf("[Teleport] failed to consume scroll char=%q item=%d: %v", c.Char.Name, scroll.ItemID, err)
		return
	}
	sendSingleInventoryUpdate(c, scroll)
	teleportClientToWorld(c, target.Char.MapID, target.Char.X, target.Char.Y)
}

func handleSaveBindLocation(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	advanceCharacterMovement(c)
	if err := SaveCharacterBindLocation(c.Char); err != nil {
		log.Printf("[Teleport] failed to save bind location char=%q: %v", c.Char.Name, err)
	}
}

func handleSaveLocation(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 36 {
		sendLocationSaved(c, locationSavedFail, nil, 0)
		return
	}
	name := trimTeleportPointName(p.ReadAsdaStringLocale(32, c.Locale))
	if p.Remaining() >= 1 {
		p.Skip(1)
	}
	if p.Remaining() >= 1 {
		_ = p.ReadUint8()
	}
	if p.Remaining() < 2 {
		sendLocationSaved(c, locationSavedFail, nil, 0)
		return
	}
	slot := p.ReadUint16()
	if slot > 9 {
		sendLocationSaved(c, locationSavedFail, nil, 0)
		return
	}
	if p.Remaining() >= 4 {
		_ = p.ReadInt16()
		_ = p.ReadInt16()
	}
	advanceCharacterMovement(c)
	rec := c.Char.TeleportPoints[slot]
	if rec == nil {
		rec = &TeleportPointRow{
			OwnerID: c.Char.GUID,
			Slot:    byte(slot),
		}
		c.Char.TeleportPoints[slot] = rec
	}
	rec.Name = name
	rec.MapID = c.Char.MapID
	rec.X = int16(c.Char.X)
	rec.Y = int16(c.Char.Y)
	if err := SaveTeleportPoint(rec); err != nil {
		log.Printf("[Teleport] failed to save point char=%q slot=%d: %v", c.Char.Name, slot, err)
		sendLocationSaved(c, locationSavedFail, nil, 0)
		return
	}
	sendLocationSaved(c, locationSavedOK, rec, int16(slot))
}

func handleDeleteSavedLocation(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 1 {
		sendSavedLocationDeleted(c, locationSavedFail, -1)
		return
	}
	slot := p.ReadUint8()
	if slot >= 10 || c.Char.TeleportPoints[slot] == nil {
		sendSavedLocationDeleted(c, locationSavedFail, -1)
		return
	}
	if err := DeleteTeleportPoint(c.Char.GUID, slot); err != nil {
		log.Printf("[Teleport] failed to delete point char=%q slot=%d: %v", c.Char.Name, slot, err)
		sendSavedLocationDeleted(c, locationSavedFail, -1)
		return
	}
	c.Char.TeleportPoints[slot] = nil
	sendSavedLocationDeleted(c, locationSavedOK, int16(slot))
}

func teleportClientToWorld(c *Client, mapID uint16, worldX float32, worldY float32) {
	if c == nil || c.Char == nil {
		return
	}
	mapID = normalizeAsda2MapID(mapID)
	if World.GetMap(mapID) == nil {
		sendTeleportedByCristal(c, teleportStatusServerDown, c.Char.MapID, int16(asda2X(c.Char.X, c.Char.MapID)), int16(asda2Y(c.Char.Y, c.Char.MapID)))
		return
	}

	advanceCharacterMovement(c)
	oldMapID := c.Char.MapID
	sameMap := oldMapID == mapID
	if !sameMap {
		World.LeaveMap(c)
	}
	c.Char.MapID = mapID
	c.Char.X = worldX
	c.Char.Y = worldY
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.IsMoving = false
	localX := int16(asda2X(c.Char.X, c.Char.MapID))
	localY := int16(asda2Y(c.Char.Y, c.Char.MapID))
	if !sameMap {
		c.IsTeleporting = true
		rememberPendingMapChange(c)
	}
	sendTeleportedByCristal(c, teleportStatusOK, c.Char.MapID, localX, localY)
	if sameMap {
		World.RefreshCharacterVisibility(c)
		World.RefreshPortalVisibility(c)
	} else {
		log.Printf("[Teleport] waiting for map-change reconnect char=%q account=%d slot=%d map=%d local=%d,%d",
			c.Char.Name, c.Char.AccID, c.Char.CharNum, c.Char.MapID, localX, localY)
	}
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Teleport] failed to save teleported character %q: %v", c.Char.Name, err)
	}
}

func readTeleportCrystalDestination(data []byte, currentMapID uint16) (byte, int, teleportCrystalDestination, bool) {
	type candidate struct {
		id     byte
		offset int
		dest   teleportCrystalDestination
		local  bool
	}
	candidates := make([]candidate, 0, 3)
	currentMapID = normalizeAsda2MapID(currentMapID)
	optionSet, hasOptionSet := teleportCrystalOptionsByMap[currentMapID]
	for _, offset := range []int{25, 0, 24} {
		if offset >= len(data) {
			continue
		}
		id := data[offset]
		if hasOptionSet {
			if dest, ok := optionDestination(optionSet.zeroBased, id); ok {
				candidates = append(candidates, candidate{id: id, offset: offset, dest: dest, local: true})
			}
			if dest, ok := optionDestination(optionSet.oneBased, id); ok {
				candidates = append(candidates, candidate{id: id, offset: offset, dest: dest, local: true})
			}
		}
		if dest, ok := teleportCrystalDestinations[id]; ok {
			candidates = append(candidates, candidate{id: id, offset: offset, dest: dest})
		}
	}
	for _, c := range candidates {
		if c.local && normalizeAsda2MapID(c.dest.mapID) != currentMapID {
			return c.id, c.offset, c.dest, true
		}
	}
	for _, c := range candidates {
		if normalizeAsda2MapID(c.dest.mapID) != currentMapID {
			return c.id, c.offset, c.dest, true
		}
	}
	if len(candidates) > 0 {
		return candidates[0].id, candidates[0].offset, candidates[0].dest, true
	}
	return 0, -1, teleportCrystalDestination{}, false
}

func optionDestination(options []teleportCrystalDestination, option byte) (teleportCrystalDestination, bool) {
	if int(option) >= len(options) {
		return teleportCrystalDestination{}, false
	}
	dest := options[option]
	return dest, dest.mapID != 0 || dest.x != 0 || dest.y != 0
}

func sendTeleportedByCristal(c *Client, status byte, mapID uint16, x int16, y int16) {
	p := NewPacket(TeleportedByCristal)
	p.WriteUint8(status)
	p.WriteAsdaString(gamePublicIP, 20)
	p.WriteUint16(gamePublicPort)
	p.WriteInt16(int16(mapID))
	p.WriteInt16(x)
	p.WriteInt16(y)
	p.WriteUint8(0)
	if mapID == types.MapIDGuildwave {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteInt64(-1)
	p.WriteInt64(-1)
	p.WriteInt32(-1)
	if mapID == types.MapIDBatleField {
		p.WriteInt16(1)
	} else {
		p.WriteInt16(-1)
	}
	c.Send(p)
}

func sendLocationSaved(c *Client, status byte, rec *TeleportPointRow, slot int16) {
	p := NewPacket(LocationSaved)
	p.WriteUint8(status)
	writeTeleportPointPayload(p, rec, slot)
	c.Send(p)
}

func sendSavedLocationDeleted(c *Client, status byte, slot int16) {
	p := NewPacket(SavedLocationDeleted)
	p.WriteUint8(status)
	p.WriteInt16(slot)
	c.SendNoCounter(p)
}

func sendSavedLocationsInit(c *Client) {
	if c == nil || c.Char == nil || !hasSavedTeleportPoints(c.Char) {
		return
	}
	p := NewPacket(SavedLocationsInit)
	p.WriteUint8(1)
	for slot, rec := range c.Char.TeleportPoints {
		pointSlot := int16(slot)
		if rec == nil {
			pointSlot = -1
		}
		writeTeleportPointPayload(p, rec, pointSlot)
	}
	c.Send(p)
}

func writeTeleportPointPayload(p *PacketOut, rec *TeleportPointRow, slot int16) {
	if rec == nil {
		p.WriteAsdaString("", 32)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteInt16(slot)
		p.WriteInt16(0)
		p.WriteInt16(0)
		return
	}
	mapID := byte(rec.MapID)
	p.WriteAsdaString(rec.Name, 32)
	p.WriteUint8(0)
	p.WriteUint8(mapID)
	p.WriteInt16(slot)
	p.WriteInt16(rec.X - int16(1000*rec.MapID))
	p.WriteInt16(rec.Y - int16(1000*rec.MapID))
}

func hasSavedTeleportPoints(chr *Character) bool {
	if chr == nil {
		return false
	}
	for _, rec := range chr.TeleportPoints {
		if rec != nil {
			return true
		}
	}
	return false
}

func findTeleportScroll(chr *Character) *ItemRow {
	for _, item := range itemsInInventory(chr.Items, types.InventoryShop) {
		if item == nil || item.Amount <= 0 {
			continue
		}
		if itemTemplateByID(item.ItemID).Category == types.ItemCategoryTeleportToCharacter {
			return item
		}
	}
	return nil
}

func isFightingMap(mapID uint16) bool {
	t := World.GetTemplate(mapID)
	return t != nil && t.IsAsda2Fighting
}

func trimTeleportPointName(name string) string {
	name = strings.TrimSpace(name)
	for len([]byte(name)) > 32 {
		_, size := utf8.DecodeLastRuneInString(name)
		if size <= 0 {
			return ""
		}
		name = name[:len(name)-size]
	}
	return name
}
