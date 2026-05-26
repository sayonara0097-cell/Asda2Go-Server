package main

import (
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	portalTriggerRadius   float64 = 3.0
	portalTriggerCooldown         = 2 * time.Second
)

var (
	nextPortalSessionID int32 = 24000
	portalCooldowns           = struct {
		sync.Mutex
		byCharacter map[uint32]time.Time
		areaByChar  map[uint32]uint32
	}{byCharacter: make(map[uint32]time.Time), areaByChar: make(map[uint32]uint32)}
)

type Portal struct {
	SessionID     int16
	WorldEntityID int32
	ID            uint32
	FromMap       uint16
	FromX         int16
	FromY         int16
	ToMap         uint16
	ToX           int16
	ToY           int16
}

func initPortalRuntime() error {
	rows, err := LoadPortals()
	if err != nil {
		return err
	}
	loaded := World.LoadPortals(rows)
	log.Printf("[Portal] %d portals loaded source=db", loaded)
	return nil
}

func newPortal(row PortalRow) (*Portal, bool) {
	fromMap := normalizeAsda2MapID(row.FromMap)
	toMap := normalizeAsda2MapID(row.ToMap)
	if World.GetMap(fromMap) == nil || World.GetMap(toMap) == nil {
		return nil, false
	}
	return &Portal{
		SessionID:     portalSessionID(row.ID),
		WorldEntityID: allocWorldEntityID(),
		ID:            row.ID,
		FromMap:       fromMap,
		FromX:         row.FromX,
		FromY:         row.FromY,
		ToMap:         toMap,
		ToX:           row.ToX,
		ToY:           row.ToY,
	}, true
}

func portalSessionID(id uint32) int16 {
	_ = id
	return allocPortalSessionID()
}

func allocPortalSessionID() int16 {
	id := int16(atomic.AddInt32(&nextPortalSessionID, 1) & maxPositiveSessionID)
	if id == 0 {
		return allocPortalSessionID()
	}
	return id
}

func (w *worldMgr) LoadPortals(rows []PortalRow) int {
	loaded := 0
	for _, row := range rows {
		portal, ok := newPortal(row)
		if !ok {
			log.Printf("[Portal] ignoring id=%d fromMap=%d toMap=%d: map is not registered", row.ID, row.FromMap, row.ToMap)
			continue
		}
		gm := w.GetMap(portal.FromMap)
		if gm == nil {
			continue
		}
		gm.AddPortal(portal)
		loaded++
	}
	return loaded
}

func (m *GameMap) AddPortal(portal *Portal) {
	if portal == nil {
		return
	}
	m.mu.Lock()
	m.portals[portal.SessionID] = portal
	m.mu.Unlock()
	debugVisibilityf("portal spawned id=%d session=%d map=%d from=%d,%d to=%d:%d,%d",
		portal.ID, portal.SessionID, portal.FromMap, portal.FromX, portal.FromY, portal.ToMap, portal.ToX, portal.ToY)
}

func (m *GameMap) broadcastPortalVisible(portal *Portal) {
}

func (w *worldMgr) RefreshPortalVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	gm.RefreshPortalVisibility(c)
}

func (m *GameMap) RefreshPortalVisibility(c *Client) {
}

func (w *worldMgr) CheckPortalTriggers(c *Client) bool {
	if c == nil || c.Char == nil {
		return false
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return false
	}
	return gm.CheckPortalTriggers(c)
}

func (m *GameMap) CheckPortalTriggers(c *Client) bool {
	if c == nil || c.Char == nil || portalOnCooldown(c.Char.GUID) {
		return false
	}
	localX := asda2X(c.Char.X, c.Char.MapID)
	localY := asda2Y(c.Char.Y, c.Char.MapID)

	m.mu.RLock()
	var hit *Portal
	for _, portal := range m.portals {
		if portal.FromMap != c.Char.MapID {
			continue
		}
		if math.Hypot(float64(localX)-float64(portal.FromX), float64(localY)-float64(portal.FromY)) <= portalTriggerRadius {
			hit = portal
			break
		}
	}
	m.mu.RUnlock()
	if hit == nil {
		clearPortalArea(c.Char.GUID)
		return false
	}
	if currentPortalArea(c.Char.GUID) == hit.ID {
		return false
	}

	setPortalArea(c.Char.GUID, hit.ID)
	setPortalCooldown(c.Char.GUID)
	teleportClientToWorld(c, hit.ToMap, mapOffset(hit.ToMap)+float32(hit.ToX), mapOffset(hit.ToMap)+float32(hit.ToY))
	setCurrentPortalArea(c)
	return true
}

func portalOnCooldown(characterID uint32) bool {
	portalCooldowns.Lock()
	defer portalCooldowns.Unlock()
	until := portalCooldowns.byCharacter[characterID]
	if until.IsZero() || time.Now().After(until) {
		delete(portalCooldowns.byCharacter, characterID)
		return false
	}
	return true
}

func setPortalCooldown(characterID uint32) {
	portalCooldowns.Lock()
	portalCooldowns.byCharacter[characterID] = time.Now().Add(portalTriggerCooldown)
	portalCooldowns.Unlock()
}

func currentPortalArea(characterID uint32) uint32 {
	portalCooldowns.Lock()
	defer portalCooldowns.Unlock()
	return portalCooldowns.areaByChar[characterID]
}

func setPortalArea(characterID uint32, portalID uint32) {
	portalCooldowns.Lock()
	portalCooldowns.areaByChar[characterID] = portalID
	portalCooldowns.Unlock()
}

func clearPortalArea(characterID uint32) {
	portalCooldowns.Lock()
	delete(portalCooldowns.areaByChar, characterID)
	portalCooldowns.Unlock()
}

func setCurrentPortalArea(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		clearPortalArea(c.Char.GUID)
		return
	}
	localX := asda2X(c.Char.X, c.Char.MapID)
	localY := asda2Y(c.Char.Y, c.Char.MapID)

	gm.mu.RLock()
	defer gm.mu.RUnlock()
	for _, portal := range gm.portals {
		if math.Hypot(float64(localX)-float64(portal.FromX), float64(localY)-float64(portal.FromY)) <= portalTriggerRadius {
			setPortalArea(c.Char.GUID, portal.ID)
			return
		}
	}
	clearPortalArea(c.Char.GUID)
}
