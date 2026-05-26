package main

// world.go — Asda2 map registry + live map runtime
//
// Ported from:
//   WCell.RealmServer/Global/World.cs  → SetupCustomMaps(), map template registry
//   WCell.RealmServer/Global/Map.cs    → Start/Stop, update loop, message queue,
//                                        AddObjectNow, RemoveObjectNow, CallDelayed,
//                                        SendPacketToMap, OnEnter/OnLeave

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// MapType
// ---------------------------------------------------------------------------

type MapType uint8

const (
	MapTypeNormal MapType = 0
)

// MapTemplate mirrors WCell.RealmServer.Global.MapTemplate (Asda2 fields).
type MapTemplate struct {
	ID              uint16
	Name            string
	Type            MapType
	MaxPlayerCount  int
	MinLevel        int
	RepopMapID      uint16
	IsAsda2Fighting bool // true only for BatleField (ID=19)
	// Offset = ID * 1000 (used by C# for coordinate translation)
	Offset float32
}

// ---------------------------------------------------------------------------
// GameMap — runtime instance of one Asda2 map
//
// Mirrors Map.cs:
//   m_characters         → characters map
//   m_messageQueue       → msgQueue channel
//   m_running            → running int32 (atomic)
//   m_updateDelay        → UpdateDelay (120 ms default)
//   MapUpdateCallback    → updateLoop goroutine
//   AddObjectNow         → Enter()
//   RemoveObjectNow      → Leave()
//   CallDelayed          → CallDelayed()
//   AddMessage           → Post()
//   SendPacketToMap      → Broadcast()
// ---------------------------------------------------------------------------

const defaultUpdateDelay = 120 * time.Millisecond // mirrors Map.DefaultUpdateDelay = 120 ms

type mapMsg func()

// GameMap is one live map instance.
type GameMap struct {
	Template *MapTemplate

	mu                sync.RWMutex
	characters        map[uint32]*Client // keyed by client.ID
	knownChars        map[uint32]map[uint32]struct{}
	knownNpcs         map[uint32]map[int16]struct{}
	knownMonsters     map[uint32]map[int16]struct{}
	nextNpcResync     map[uint32]time.Time
	nextNpcServerFull map[uint32]time.Time
	nextMonsterResync map[uint32]time.Time
	npcs              map[int16]*Npc     // keyed by per-map NPC session id
	portals           map[int16]*Portal  // keyed by per-map portal session id
	monsters          map[int16]*Monster // keyed by per-map monster session id
	spawns            map[uint32]*MonsterSpawn
	loots             map[lootKey]*LootItem

	msgQueue chan mapMsg   // mirrors m_messageQueue (LockfreeQueue)
	running  int32         // 1 = running, 0 = stopped (atomic)
	stopCh   chan struct{} // closed when Stop() is called
}

// newGameMap creates a stopped map ready to accept messages.
func newGameMap(t *MapTemplate) *GameMap {
	return &GameMap{
		Template:          t,
		characters:        make(map[uint32]*Client),
		knownChars:        make(map[uint32]map[uint32]struct{}),
		knownNpcs:         make(map[uint32]map[int16]struct{}),
		knownMonsters:     make(map[uint32]map[int16]struct{}),
		nextNpcResync:     make(map[uint32]time.Time),
		nextNpcServerFull: make(map[uint32]time.Time),
		nextMonsterResync: make(map[uint32]time.Time),
		npcs:              make(map[int16]*Npc),
		portals:           make(map[int16]*Portal),
		monsters:          make(map[int16]*Monster),
		spawns:            make(map[uint32]*MonsterSpawn),
		loots:             make(map[lootKey]*LootItem),
		msgQueue:          make(chan mapMsg, 512),
		stopCh:            make(chan struct{}),
	}
}

// ---------------------------------------------------------------------------
// Start / Stop — mirrors Map.Start() / Map.Stop()
// ---------------------------------------------------------------------------

// Start begins the map update goroutine if not already running.
// Mirrors Map.Start() → Task.Factory.StartNewDelayed(m_updateDelay, MapUpdateCallback)
func (m *GameMap) Start() {
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return // already running
	}
	m.mu.Lock()
	m.stopCh = make(chan struct{})
	stopCh := m.stopCh
	m.mu.Unlock()

	log.Printf("[Map] %s (id=%d) started", m.Template.Name, m.Template.ID)
	go m.updateLoop(stopCh)
}

// Stop halts the update goroutine.
// Mirrors Map.Stop()
func (m *GameMap) Stop() {
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return
	}
	m.mu.Lock()
	close(m.stopCh)
	m.mu.Unlock()
	log.Printf("[Map] %s (id=%d) stopped", m.Template.Name, m.Template.ID)
}

// IsRunning mirrors Map.IsRunning
func (m *GameMap) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}

// ---------------------------------------------------------------------------
// updateLoop — mirrors MapUpdateCallback
//
// C# pattern: callback reschedules itself after each tick via
//   Task.Factory.StartNewDelayed(millisecondsDelay, MapUpdateCallback, this)
// We use a ticker goroutine instead, which is idiomatic Go and equivalent.
// ---------------------------------------------------------------------------

func (m *GameMap) updateLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(defaultUpdateDelay)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.tick()
		}
	}
}

// tick drains the message queue then runs per-character environment updates.
// Mirrors the body of MapUpdateCallback.
func (m *GameMap) tick() {
	// 1. Drain message queue (mirrors: while(m_messageQueue.TryDequeue(out message)) message.Execute())
	for {
		select {
		case fn := <-m.msgQueue:
			safeCall(fn)
		default:
			goto drained
		}
	}
drained:

	// 2. Per-character update — mirrors UpdateCharacters() called every
	//    CharacterUpdateEnvironmentTicks ticks (we do it every tick; fine for now).
	m.mu.RLock()
	chars := make([]*Client, 0, len(m.characters))
	for _, c := range m.characters {
		chars = append(chars, c)
	}
	m.mu.RUnlock()

	if len(chars) == 0 {
		m.Stop()
		return
	}

	for _, c := range chars {
		if c.Char != nil {
			updateCharacterEnvironment(c)
		}
	}
	m.updateMonsterAI(chars, time.Now())
}

// safeCall runs fn, recovering from panics so one bad message can't kill the loop.
func safeCall(fn mapMsg) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Map] panic in message: %v", r)
		}
	}()
	fn()
}

// ---------------------------------------------------------------------------
// Post (AddMessage) — mirrors Map.AddMessage(Action) / AddMessage(IMessage)
//
// Starts the map automatically on first message, exactly as C# does:
//   public void AddMessage(IMessage msg) { Start(); m_messageQueue.Enqueue(msg); }
// ---------------------------------------------------------------------------

func (m *GameMap) Post(fn mapMsg) {
	m.Start()
	m.msgQueue <- fn
}

// ---------------------------------------------------------------------------
// CallDelayed — mirrors Map.CallDelayed(int millis, Action action)
//
// C# implementation registers a TimerEntry as an IUpdatable that fires once
// after the given delay, then unregisters itself.
// We spawn a goroutine that sleeps then posts to the map queue.
// ---------------------------------------------------------------------------

func (m *GameMap) CallDelayed(millis int, fn func()) {
	go func() {
		time.Sleep(time.Duration(millis) * time.Millisecond)
		m.Post(fn)
	}()
}

// ---------------------------------------------------------------------------
// Enter / Leave — mirrors AddObjectNow / RemoveObjectNow for Character objects
// ---------------------------------------------------------------------------

// Enter adds a client's character to this map and fires OnEnter.
// Mirrors Map.AddObjectNow → Character branch → OnEnter(chr).
func (m *GameMap) Enter(c *Client) {
	if c.Char == nil {
		return
	}
	m.Start() // auto-start on first player, mirrors AddMessage→Start()

	m.mu.Lock()
	m.characters[c.ID] = c
	m.mu.Unlock()

	log.Printf("[Map] %q entered %s (id=%d)", c.Char.Name, m.Template.Name, m.Template.ID)
	m.onEnter(c)
	m.spawnDueMonsters()
}

// Leave removes a client from this map and fires OnLeave.
// Mirrors Map.RemoveObjectNow → Character branch → OnLeave(chr).
func (m *GameMap) Leave(c *Client) bool {
	if c.Char == nil {
		return false
	}
	sessionID := c.Char.SessionID
	m.mu.Lock()
	_, was := m.characters[c.ID]
	delete(m.characters, c.ID)
	delete(m.knownNpcs, c.ID)
	delete(m.nextNpcResync, c.ID)
	delete(m.nextNpcServerFull, c.ID)
	delete(m.nextMonsterResync, c.ID)
	for _, monster := range m.monsters {
		if monster.TargetSession == sessionID {
			monster.TargetSession = 0
		}
	}
	count := len(m.characters)
	m.mu.Unlock()

	if was {
		log.Printf("[Map] %q left %s (id=%d)", c.Char.Name, m.Template.Name, m.Template.ID)
		m.onLeave(c)
	}

	// Stop map ticker when empty (saves CPU; restarts automatically on next Enter).
	// Mirrors: maps don't literally stop in C# but we can safely do so.
	if count == 0 {
		m.Stop()
	}
	return was
}

// onEnter mirrors Map.OnEnter(Character chr) — currently a no-op in the base class,
// overridden in Battleground etc. We broadcast character visibility to existing players.
func (m *GameMap) onEnter(c *Client) {
	m.RefreshCharacterVisibility(c)
}

// onLeave mirrors Map.OnLeave(Character chr).
// ForgetCharacter sends delete packets only to clients that currently know this character.
func (m *GameMap) onLeave(c *Client) {
	m.ForgetCharacter(c)
}

// ---------------------------------------------------------------------------
// Broadcast / SendPacketToMap — mirrors Map.SendPacketToMap(RealmPacketOut packet)
// ---------------------------------------------------------------------------

// Broadcast sends a packet to every client currently on this map.
// Mirrors Map.SendPacketToMap.
func (m *GameMap) Broadcast(p *PacketOut) {
	m.mu.RLock()
	chars := make([]*Client, 0, len(m.characters))
	for _, c := range m.characters {
		chars = append(chars, c)
	}
	m.mu.RUnlock()
	for _, c := range chars {
		c.Send(p)
	}
}

// Characters returns a snapshot of all clients on this map.
func (m *GameMap) Characters() []*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Client, 0, len(m.characters))
	for _, c := range m.characters {
		out = append(out, c)
	}
	return out
}

// PlayerCount mirrors Map.PlayerCount
func (m *GameMap) PlayerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.characters)
}

// ---------------------------------------------------------------------------
// updateCharacterEnvironment — stub for Map.UpdateCharacters → character.UpdateEnvironment
// Movement packets originate from handlers. Keep this aligned with the older
// AsdaGo movement behavior so jumps stay fully client-driven.
// ---------------------------------------------------------------------------

func updateCharacterEnvironment(c *Client) {
	now := time.Now()
	if npcServerClient != nil {
		World.TickNpcServerVisibility(c, now)
		return
	}
	World.TickNpcVisibility(c, now)
	World.TickMonsterVisibility(c, now)
}

// ---------------------------------------------------------------------------
// World singleton — holds all map templates and live instances
// ---------------------------------------------------------------------------

var World = &worldMgr{
	templates: make(map[uint16]*MapTemplate),
	maps:      make(map[uint16]*GameMap),
}

type worldMgr struct {
	mu        sync.RWMutex
	templates map[uint16]*MapTemplate
	maps      map[uint16]*GameMap
}

// GetMap returns the live GameMap for the given ID, or nil.
func (w *worldMgr) GetMap(mapID uint16) *GameMap {
	mapID = normalizeAsda2MapID(mapID)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.maps[mapID]
}

// GetTemplate returns the MapTemplate for the given ID, or nil.
func (w *worldMgr) GetTemplate(mapID uint16) *MapTemplate {
	mapID = normalizeAsda2MapID(mapID)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.templates[mapID]
}

// EnterMap looks up the correct GameMap for the client's character and calls Enter.
// Mirrors World.GetMap(record) → map.AddObjectNow(character)
func (w *worldMgr) EnterMap(c *Client) *GameMap {
	if c.Char == nil {
		return nil
	}
	c.Char.MapID = normalizeAsda2MapID(c.Char.MapID)
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		log.Printf("[World] EnterMap: unknown mapID %d for %q", c.Char.MapID, c.Char.Name)
		return nil
	}
	gm.Enter(c)
	syncNpcServerPlayer(c)
	return gm
}

// LeaveMap removes the client from their current map.
func (w *worldMgr) LeaveMap(c *Client) {
	if c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm != nil {
		if gm.Leave(c) {
			leaveNpcServerPlayer(c)
		}
	}
}

// CharactersOnMap returns a snapshot of all clients on a given map.
// Used by SendToArea in client.go.
func (w *worldMgr) CharactersOnMap(mapID uint16) []*Client {
	gm := w.GetMap(mapID)
	if gm == nil {
		return nil
	}
	return gm.Characters()
}

func (w *worldMgr) Maps() []*GameMap {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*GameMap, 0, len(w.maps))
	for _, gm := range w.maps {
		out = append(out, gm)
	}
	return out
}

func (w *worldMgr) register(t *MapTemplate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.templates[t.ID] = t
	w.maps[t.ID] = newGameMap(t)
}

// ---------------------------------------------------------------------------
// initWorld — called from main.go at startup
// ---------------------------------------------------------------------------

func initWorld() {
	setupCustomMaps()
	log.Printf("[World] %d maps registered", len(World.templates))
}

// ---------------------------------------------------------------------------
func setupCustomMaps() {
	def := func(info Asda2MapInfo) {
		World.register(&MapTemplate{
			ID:              info.ID,
			Name:            info.Name,
			Type:            MapTypeNormal,
			MaxPlayerCount:  1000,
			MinLevel:        1,
			RepopMapID:      info.ID,
			IsAsda2Fighting: info.Fighting,
			Offset:          float32(info.ID) * 1000.0, // mirrors: Offset = (float)rgnTemplate.ID * 1000f
		})
	}
	for _, info := range asda2MapInfos {
		def(info)
	}
}
