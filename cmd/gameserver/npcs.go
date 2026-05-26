package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"asda2/shared/types"
	"asda2/shared/worlddata"
)

const (
	npcKindTrainer                        byte   = 1
	defaultNpcSessionIDStart              int32  = 1000
	npcVisibilityRange                           = 100.0
	npcVisibilityResyncInterval                  = 15
	npcServerVisibilityResyncInterval            = 2 * time.Second
	npcServerVisibilityFullResyncInterval        = 12 * time.Second
	npcServerMaxVisibleNpcsPerRefresh            = 10
	npcServerMaxDeletesPerRefresh                = 8
	ignoredNpcEntryID                     uint16 = 145
)

var nextNpcSessionID int32 = defaultNpcSessionIDStart

var npcVisibleStab96 = []byte{
	0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0,
	0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

var npcTemplates = struct {
	sync.RWMutex
	byEntry map[uint16]NpcTemplateRow
}{
	byEntry: make(map[uint16]NpcTemplateRow),
}

type Npc struct {
	SessionID       int16
	WorldEntityID   int32
	SpawnID         uint32
	EntryID         uint16
	Name            string
	Kind            byte
	ClassGroup      byte
	IsTrainer       bool
	InteractionKind types.NpcInteractionKind
	MapID           uint16
	LocalX          int16
	LocalY          int16
	Channel         int16
}

func initNpcRuntime(channel byte) error {
	templates, spawns, source, err := loadNpcRuntimeData(channel)
	if err != nil {
		return err
	}
	setNpcTemplates(templates)
	loaded := World.LoadNpcSpawns(spawns)
	log.Printf("[NPC] %d spawns loaded for channel %d source=%s", loaded, channel, source)
	log.Printf("[NPCVisibility] enabled range=%.0f resync=off", npcVisibilityRange)
	return nil
}

func loadNpcRuntimeData(channel byte) ([]NpcTemplateRow, []NpcSpawnRow, string, error) {
	templates, spawns, source, ok, err := worlddata.LoadNpcs("", channel)
	if err != nil {
		return nil, nil, "", err
	}
	if ok {
		return templates, spawns, source, nil
	}

	templates, err = LoadNpcTemplates()
	if err != nil {
		return nil, nil, "", fmt.Errorf("load npc templates: %w", err)
	}
	spawns, err = LoadNpcSpawns(channel)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load npc spawns: %w", err)
	}
	return templates, spawns, "db", nil
}

func setNpcTemplates(rows []NpcTemplateRow) {
	npcTemplates.Lock()
	npcTemplates.byEntry = make(map[uint16]NpcTemplateRow, len(rows))
	for _, row := range rows {
		npcTemplates.byEntry[row.EntryID] = row
	}
	npcTemplates.Unlock()
	log.Printf("[NPC] %d templates loaded", len(rows))
}

func newNpc(row NpcSpawnRow) (*Npc, bool) {
	if row.EntryID == ignoredNpcEntryID {
		return nil, false
	}
	npcTemplates.RLock()
	template, ok := npcTemplates.byEntry[row.EntryID]
	npcTemplates.RUnlock()
	if !ok {
		return nil, false
	}
	template = types.NormalizeNpcTemplate(template)
	return &Npc{
		SessionID:       npcSessionID(row.SpawnID),
		WorldEntityID:   allocWorldEntityID(),
		SpawnID:         row.SpawnID,
		EntryID:         row.EntryID,
		Name:            template.Name,
		Kind:            template.Kind,
		ClassGroup:      template.ClassGroup,
		IsTrainer:       template.IsTrainer,
		InteractionKind: template.InteractionKind,
		MapID:           row.MapID,
		LocalX:          row.LocalX,
		LocalY:          row.LocalY,
		Channel:         row.Channel,
	}, true
}

func npcSessionID(spawnID uint32) int16 {
	if spawnID > 0 && spawnID < uint32(maxPositiveSessionID) {
		return int16(spawnID)
	}
	return allocNpcSessionID()
}

func allocNpcSessionID() int16 {
	id := int16(atomic.AddInt32(&nextNpcSessionID, 1) & 0x7FFF)
	if id == 0 {
		return allocNpcSessionID()
	}
	return id
}

func (w *worldMgr) LoadNpcSpawns(rows []NpcSpawnRow) int {
	loaded := 0
	for _, row := range rows {
		gm := w.GetMap(row.MapID)
		if gm == nil {
			log.Printf("[NPC] ignoring spawn=%d entry=%d: map %d is not registered", row.SpawnID, row.EntryID, row.MapID)
			continue
		}
		npc, ok := newNpc(row)
		if !ok {
			log.Printf("[NPC] ignoring spawn=%d entry=%d: template not loaded or blocked", row.SpawnID, row.EntryID)
			continue
		}
		gm.AddNpc(npc)
		loaded++
	}
	return loaded
}

func (m *GameMap) AddNpc(npc *Npc) {
	if npc == nil {
		return
	}
	m.mu.Lock()
	m.npcs[npc.SessionID] = npc
	m.mu.Unlock()
	if m.PlayerCount() > 0 {
		m.broadcastNpcVisible(npc)
	}
	debugNpcSpawnf("spawned entry=%d session=%d spawn=%d map=%d x=%d y=%d trainer=%t",
		npc.EntryID, npc.SessionID, npc.SpawnID, npc.MapID, npc.LocalX, npc.LocalY, npc.IsTrainer)
}

func (m *GameMap) Npcs() []*Npc {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Npc, 0, len(m.npcs))
	for _, npc := range m.npcs {
		out = append(out, npc)
	}
	return out
}

func (m *GameMap) FindNpcByClientTargetID(targetID uint16) (*Npc, bool) {
	if targetID == 0 {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if npc, ok := m.npcs[int16(targetID)]; ok {
		return npc, true
	}
	for _, npc := range m.npcs {
		if npc.EntryID == targetID ||
			uint16(npc.SpawnID) == targetID ||
			uint16(npc.WorldEntityID) == targetID {
			return npc, true
		}
	}
	return nil, false
}

func (m *GameMap) sendVisibleNpcsTo(c *Client) int {
	return m.RefreshNpcVisibility(c)
}

func (w *worldMgr) RefreshNpcVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	gm.RefreshNpcVisibility(c)
}

func (w *worldMgr) ResyncNpcVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	gm.ResyncNpcVisibility(c)
}

func (w *worldMgr) TickNpcVisibility(c *Client, now time.Time) {
	if c == nil || c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	gm.TickNpcVisibility(c, now)
}

func (m *GameMap) RefreshNpcVisibility(c *Client) int {
	if c == nil || c.Char == nil {
		return 0
	}
	if disableNpcVisibility {
		return 0
	}
	if npcServerClient != nil {
		return m.refreshNpcVisibilityFromServer(c, false)
	}
	npcs := m.refreshNpcVisibility(c, false)
	if len(npcs) > 0 {
		debugNpcVisibility(c, "refresh", len(npcs))
		sendNpcVisibleList(c, npcs)
	}
	return len(npcs)
}

func (m *GameMap) ResyncNpcVisibility(c *Client) {
	if c == nil || c.Char == nil || disableNpcVisibility {
		return
	}
	if npcServerClient != nil {
		m.refreshNpcVisibilityFromServer(c, true)
		return
	}
	npcs := m.refreshNpcVisibility(c, true)
	if len(npcs) > 0 {
		debugNpcVisibility(c, "resync", len(npcs))
		sendNpcVisibleList(c, npcs)
	}
}

func (m *GameMap) TickNpcVisibility(c *Client, now time.Time) {
	if c == nil || c.Char == nil || disableNpcVisibility {
		return
	}
	if !m.npcResyncDue(c.ID, now) {
		return
	}
	if npcServerClient != nil {
		m.RefreshNpcVisibility(c)
		return
	}
	m.ResyncNpcVisibility(c)
}

func (m *GameMap) npcResyncDue(viewerID uint32, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if next := m.nextNpcResync[viewerID]; !next.IsZero() && now.Before(next) {
		return false
	}
	interval := npcVisibilityResyncInterval * time.Second
	if npcServerClient != nil {
		interval = npcServerVisibilityResyncInterval
	}
	m.nextNpcResync[viewerID] = now.Add(interval)
	return true
}

func (m *GameMap) refreshNpcVisibility(c *Client, forceResend bool) []*Npc {
	if c == nil || c.Char == nil {
		return nil
	}
	m.mu.Lock()
	if _, ok := m.characters[c.ID]; !ok {
		m.mu.Unlock()
		return nil
	}
	known := m.ensureKnownNpcSetLocked(c.ID)
	visible := make([]*Npc, 0, len(m.npcs))
	for sessionID, npc := range m.npcs {
		_, wasKnown := known[sessionID]
		if clientCanSeeNpc(c, npc) {
			if forceResend || !wasKnown {
				known[sessionID] = struct{}{}
				visible = append(visible, npc)
			}
			continue
		}
		if wasKnown {
			delete(known, sessionID)
		}
	}
	m.mu.Unlock()

	return visible
}

func (m *GameMap) refreshNpcVisibilityFromServer(c *Client, forceResend bool) int {
	npcs, _, err := npcServerClient.VisibleWorld(c, forceResend)
	if err != nil {
		logNpcServerUnavailable(err)
		m.flushKnownNpcsFor(c)
		return 0
	}

	m.mu.Lock()
	if _, ok := m.characters[c.ID]; !ok {
		m.mu.Unlock()
		return 0
	}
	known := m.ensureKnownNpcSetLocked(c.ID)
	current := make(map[int16]struct{}, len(npcs))
	visible := make([]*Npc, 0, len(npcs))
	disappeared := make([]*Npc, 0)
	for _, npc := range npcs {
		if npc == nil {
			continue
		}
		current[npc.SessionID] = struct{}{}
		m.npcs[npc.SessionID] = npc
		_, wasKnown := known[npc.SessionID]
		if forceResend || !wasKnown {
			known[npc.SessionID] = struct{}{}
			visible = append(visible, npc)
		}
	}
	for sessionID := range known {
		if _, ok := current[sessionID]; !ok {
			if npc := m.npcs[sessionID]; npc != nil {
				disappeared = append(disappeared, npc)
			}
			delete(known, sessionID)
		}
	}
	m.mu.Unlock()

	for _, npc := range disappeared {
		sendEntityDelete(c, npc.SessionID, npc.WorldEntityID)
	}
	if len(visible) > 0 {
		debugNpcVisibility(c, "remote", len(visible))
		sendNpcVisibleList(c, visible)
	}
	return len(visible)
}

func (m *GameMap) flushKnownNpcsFor(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	type deleteAction struct {
		viewer        *Client
		sessionID     int16
		worldEntityID int32
	}
	var actions []deleteAction
	m.mu.Lock()
	for viewerID, known := range m.knownNpcs {
		viewer := m.characters[viewerID]
		for sessionID := range known {
			if npc := m.npcs[sessionID]; npc != nil && viewer != nil {
				actions = append(actions, deleteAction{
					viewer:        viewer,
					sessionID:     npc.SessionID,
					worldEntityID: npc.WorldEntityID,
				})
			}
			delete(known, sessionID)
		}
	}
	m.npcs = make(map[int16]*Npc)
	m.mu.Unlock()

	for _, action := range actions {
		sendEntityDelete(action.viewer, action.sessionID, action.worldEntityID)
	}
	if len(actions) > 0 {
		log.Printf("[NpcServer] hid %d NPC visibility record(s) after remote visibility loss", len(actions))
	}
}

func sendNpcVisibleList(c *Client, npcs []*Npc) {
	labels := make([]string, 0, 8)
	for _, npc := range npcs {
		sendNpcVisible(c, npc)
		if len(labels) < 8 {
			labels = append(labels, fmt.Sprintf("%d:%d@%d,%d", npc.SessionID, npc.EntryID, npc.LocalX, npc.LocalY))
		}
	}
	if len(npcs) > 0 {
		log.Printf("[NPC] visible to %q count=%d first=%s", c.Char.Name, len(npcs), strings.Join(labels, " "))
	}
}

func (m *GameMap) ensureKnownNpcSetLocked(viewerID uint32) map[int16]struct{} {
	known := m.knownNpcs[viewerID]
	if known == nil {
		known = make(map[int16]struct{})
		m.knownNpcs[viewerID] = known
	}
	return known
}

func (m *GameMap) broadcastNpcVisible(npc *Npc) {
	if npc == nil {
		return
	}
	for _, c := range m.Characters() {
		if !clientCanSeeNpc(c, npc) {
			continue
		}
		sendNpcVisible(c, npc)
	}
}

func clientCanSeeNpc(c *Client, npc *Npc) bool {
	if c == nil || c.Char == nil || npc == nil {
		return false
	}
	if c.Char.MapID != npc.MapID {
		return false
	}
	if npc.Channel >= 0 && npc.Channel != int16(c.Channel) {
		return false
	}
	return distance2D(
		float64(npc.LocalX),
		float64(npc.LocalY),
		float64(asda2X(c.Char.X, c.Char.MapID)),
		float64(asda2Y(c.Char.Y, c.Char.MapID)),
	) < npcVisibilityRange
}

func sendNpcVisible(c *Client, npc *Npc) {
	if c == nil || c.Char == nil || npc == nil {
		return
	}
	if npc.EntryID == ignoredNpcEntryID {
		return
	}
	p := NewPacket(NpcVisiableNow)
	p.WriteInt16(npc.SessionID)
	p.WriteUint16(npc.EntryID)
	p.WriteInt32(npc.WorldEntityID)
	p.WriteInt16(npc.LocalX)
	p.WriteInt16(npc.LocalY)
	p.WriteBytes(npcVisibleStab96)
	c.SendNoCounter(p)
}

func debugNpcSpawnf(format string, args ...any) {
	if !visibilityDebugEnabled {
		return
	}
	log.Printf("[NPC] "+format, args...)
}

func debugNpcVisibility(c *Client, phase string, sent int) {
	if !visibilityDebugEnabled || c == nil || c.Char == nil {
		return
	}
	log.Printf("[NPCVisibility] %s viewer=%s map=%d local=%.2f,%.2f sent=%d",
		phase,
		clientDebugLabel(c),
		c.Char.MapID,
		asda2X(c.Char.X, c.Char.MapID),
		asda2Y(c.Char.Y, c.Char.MapID),
		sent,
	)
}

func envNPCVisibilityDisabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ASDA2_DISABLE_NPC_VISIBILITY")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
