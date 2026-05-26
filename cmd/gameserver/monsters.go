package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"asda2/shared/relay"
	"asda2/shared/worlddata"
)

type MonsterState int32

const (
	MonsterStateDead MonsterState = 0
	MonsterStateOK   MonsterState = 1

	defaultMonsterHealth         int32 = 100
	defaultMonsterMoveMS         int16 = 150
	minVisibleMonsterMoveMS      int16 = 650
	defaultMonsterRespawnSeconds       = 30

	defaultMonsterAggroRange        float64 = 8
	defaultMonsterAttackRange       float64 = 1.5
	defaultMonsterLeashRange        float64 = 18
	defaultMonsterRoamRadius        float64 = 5
	minVisibleMonsterRoamRadius     float64 = 4
	defaultMonsterAttackDamage      int32   = 5
	defaultMonsterWalkSpeed         float64 = 1
	defaultMonsterRunSpeed          float64 = 3.5
	defaultMonsterAttackInterval            = 2 * time.Second
	defaultMonsterMoveInterval              = 1500 * time.Millisecond
	defaultMonsterRoamIntervalMin           = 1500 * time.Millisecond
	defaultMonsterRoamIntervalMax           = 3500 * time.Millisecond
	defaultMonsterSkillCooldown             = 7 * time.Second
	monsterMoveMinMillis            int16   = 800
	monsterMoveMaxMillis            int16   = 5000
	monsterVisibilityRange                  = characterBroadcastRange
	monsterVisibilityResyncInterval         = 15 * time.Second
	npcServerMonsterResyncInterval          = 2 * time.Second

	monsterMoveTypeWalk byte = 2
	monsterMoveTypeRun  byte = 5

	defaultMonsterSessionIDStart int32 = 20000
	maxPositiveSessionID         int32 = 0x7FFF
)

var nextMonsterSessionID int32 = defaultMonsterSessionIDStart
var (
	monsterVisibleStab23 = make([]byte, 2)
	monsterVisibleStab33 = buildMonsterVisibleStab33()
	monsterMoveStab35    = []byte{2, 0, 0xFF, 0xFF, 0, 0, 0, 0}
	monsterStateStab6    = []byte{14, 0}
	monsterDamageStab8   = []byte{0, 0xFF, 0xFF, 0, 0, 0, 0, 62, 239, 246, 57, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
)

var monsterTemplates = struct {
	sync.RWMutex
	byEntry map[uint16]MonsterTemplateRow
}{
	byEntry: make(map[uint16]MonsterTemplateRow),
}

var monsterDrops = struct {
	sync.RWMutex
	byEntry map[uint16][]MonsterDropRow
}{
	byEntry: make(map[uint16][]MonsterDropRow),
}

type Monster struct {
	SessionID      int16
	WorldEntityID  int32
	SpawnID        uint32
	EntryID        uint16
	Level          byte
	MapID          uint16
	LocalX         int16
	LocalY         int16
	HomeX          int16
	HomeY          int16
	Health         int32
	MaxHealth      int32
	MoveMS         int16
	WalkSpeed      float64
	RunSpeed       float64
	RespawnSeconds int
	MinDamage      float64
	MaxDamage      float64
	AttackInterval time.Duration
	AggroRange     float64
	AttackRange    float64
	LeashRange     float64
	RoamRadius     float64
	MovementType   int
	AI             string
	State          MonsterState
	TargetSession  int16
	NextMoveAt     time.Time
	NextAttackAt   time.Time
	NextRoamAt     time.Time
	NextSkillAt    time.Time
	RoamTargetX    int16
	RoamTargetY    int16
	IsMoving       bool
	MoveType       byte
	MoveFromX      float64
	MoveFromY      float64
	MoveDestX      int16
	MoveDestY      int16
	MoveStartedAt  time.Time
	MoveDuration   time.Duration
	NpcServerOwned bool
}

type MonsterSpawn struct {
	SpawnID         uint32
	EntryID         uint16
	MapID           uint16
	LocalX          int16
	LocalY          int16
	RespawnSeconds  int
	Channel         int16
	AI              string
	AggroRange      float64
	LeashRange      float64
	SpawnDistance   float64
	MovementType    int
	ActiveSessionID int16
	RespawnReadyAt  time.Time
}

func initMonsterRuntime(channel byte) error {
	templates, spawns, drops, source, err := loadMonsterRuntimeData(channel)
	if err != nil {
		return err
	}
	setMonsterTemplates(templates)
	setMonsterDrops(drops)
	loaded := World.LoadMonsterSpawns(spawns)
	log.Printf("[Monster] %d spawns loaded for channel %d source=%s", loaded, channel, source)
	log.Printf("[MonsterVisibility] enabled range=%.0f resync=off", monsterVisibilityRange)
	return nil
}

func reloadMonsterRuntime(channel byte) error {
	removed := World.ClearMonsterSpawns()
	templates, spawns, drops, source, err := loadMonsterRuntimeData(channel)
	if err != nil {
		return err
	}
	setMonsterTemplates(templates)
	setMonsterDrops(drops)
	loaded := World.LoadMonsterSpawns(spawns)
	log.Printf("[Monster] reloaded spawns for channel %d source=%s removed=%d loaded=%d", channel, source, removed, loaded)
	return nil
}

func loadMonsterRuntimeData(channel byte) ([]MonsterTemplateRow, []MonsterSpawnRow, []MonsterDropRow, string, error) {
	templates, spawns, source, ok, err := worlddata.LoadMonsters("", channel)
	if err != nil {
		return nil, nil, nil, "", err
	}
	if ok {
		drops, dropSource, dropsOK, err := worlddata.LoadMonsterDrops("")
		if err != nil {
			return nil, nil, nil, "", err
		}
		if dropsOK {
			source = source + " drops=" + dropSource
		}
		return templates, spawns, drops, source, nil
	}

	templates, err = LoadMonsterTemplates()
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("load monster templates: %w", err)
	}
	spawns, err = LoadMonsterSpawns(channel)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("load monster spawns: %w", err)
	}
	return templates, spawns, nil, "db", nil
}

func setMonsterTemplates(rows []MonsterTemplateRow) {
	monsterTemplates.Lock()
	monsterTemplates.byEntry = make(map[uint16]MonsterTemplateRow, len(rows))
	for _, row := range rows {
		monsterTemplates.byEntry[row.EntryID] = row
	}
	monsterTemplates.Unlock()
	log.Printf("[Monster] %d templates loaded", len(rows))
}

func setMonsterDrops(rows []MonsterDropRow) {
	monsterDrops.Lock()
	monsterDrops.byEntry = make(map[uint16][]MonsterDropRow)
	for _, row := range rows {
		monsterDrops.byEntry[row.EntryID] = append(monsterDrops.byEntry[row.EntryID], row)
	}
	monsterCount := len(monsterDrops.byEntry)
	monsterDrops.Unlock()
	log.Printf("[Monster] %d drop rows loaded for %d monsters", len(rows), monsterCount)
}

func monsterDefaults(entryID uint16) MonsterTemplateRow {
	monsterTemplates.RLock()
	template, ok := monsterTemplates.byEntry[entryID]
	monsterTemplates.RUnlock()
	if !ok {
		return MonsterTemplateRow{
			EntryID:      entryID,
			Level:        1,
			MaxHealth:    defaultMonsterHealth,
			MoveMS:       defaultMonsterMoveMS,
			WalkSpeed:    defaultMonsterWalkSpeed,
			RunSpeed:     defaultMonsterRunSpeed,
			MinDamage:    float64(defaultMonsterAttackDamage),
			MaxDamage:    float64(defaultMonsterAttackDamage),
			BaseAttackMS: int(defaultMonsterAttackInterval / time.Millisecond),
		}
	}
	if template.Level == 0 {
		template.Level = 1
	}
	if template.MaxHealth <= 0 {
		template.MaxHealth = defaultMonsterHealth
	}
	if template.MoveMS <= 0 {
		template.MoveMS = defaultMonsterMoveMS
	}
	if template.MoveMS < minVisibleMonsterMoveMS {
		template.MoveMS = minVisibleMonsterMoveMS
	}
	if template.WalkSpeed <= 0 {
		template.WalkSpeed = defaultMonsterWalkSpeed
	}
	if template.RunSpeed <= 0 {
		template.RunSpeed = defaultMonsterRunSpeed
	}
	if template.MinDamage <= 0 {
		template.MinDamage = float64(defaultMonsterAttackDamage)
	}
	if template.MaxDamage < template.MinDamage {
		template.MaxDamage = template.MinDamage
	}
	if template.BaseAttackMS <= 0 {
		template.BaseAttackMS = int(defaultMonsterAttackInterval / time.Millisecond)
	}
	return template
}

func newMonster(entryID uint16, mapID uint16, localX int16, localY int16) *Monster {
	template := monsterDefaults(entryID)
	return &Monster{
		SessionID:      allocMonsterSessionID(),
		WorldEntityID:  allocMonsterWorldEntityID(),
		EntryID:        entryID,
		Level:          template.Level,
		MapID:          mapID,
		LocalX:         localX,
		LocalY:         localY,
		HomeX:          localX,
		HomeY:          localY,
		Health:         template.MaxHealth,
		MaxHealth:      template.MaxHealth,
		MoveMS:         template.MoveMS,
		WalkSpeed:      template.WalkSpeed,
		RunSpeed:       template.RunSpeed,
		RespawnSeconds: 0,
		MinDamage:      template.MinDamage,
		MaxDamage:      template.MaxDamage,
		AttackInterval: time.Duration(template.BaseAttackMS) * time.Millisecond,
		AggroRange:     defaultMonsterAggroRange,
		AttackRange:    defaultMonsterAttackRange,
		LeashRange:     defaultMonsterLeashRange,
		RoamRadius:     defaultMonsterRoamRadius,
		MoveType:       monsterMoveTypeWalk,
		State:          MonsterStateOK,
		NextRoamAt:     time.Now().Add(randomRoamInterval()),
		NextSkillAt:    time.Now().Add(defaultMonsterSkillCooldown),
	}
}

func newMonsterFromSpawn(spawn *MonsterSpawn) *Monster {
	monster := newMonster(spawn.EntryID, spawn.MapID, spawn.LocalX, spawn.LocalY)
	monster.SpawnID = spawn.SpawnID
	monster.RespawnSeconds = spawn.RespawnSeconds
	monster.AI = strings.TrimSpace(spawn.AI)
	if spawn.AggroRange > 0 {
		monster.AggroRange = spawn.AggroRange
	}
	if spawn.LeashRange > 0 {
		monster.LeashRange = spawn.LeashRange
	}
	if spawn.SpawnDistance > 0 {
		monster.RoamRadius = spawn.SpawnDistance
	}
	if monster.RoamRadius < minVisibleMonsterRoamRadius {
		monster.RoamRadius = minVisibleMonsterRoamRadius
	}
	if spawn.MovementType > 0 {
		monster.MovementType = spawn.MovementType
	}
	if monster.LeashRange < monster.RoamRadius+defaultMonsterAttackRange {
		monster.LeashRange = monster.RoamRadius + defaultMonsterAttackRange
	}
	return monster
}

func newMonsterSpawn(row MonsterSpawnRow) *MonsterSpawn {
	respawnSeconds := row.RespawnSeconds
	if respawnSeconds <= 0 {
		respawnSeconds = defaultMonsterRespawnSeconds
	}
	return &MonsterSpawn{
		SpawnID:        row.SpawnID,
		EntryID:        row.EntryID,
		MapID:          row.MapID,
		LocalX:         row.LocalX,
		LocalY:         row.LocalY,
		RespawnSeconds: respawnSeconds,
		Channel:        row.Channel,
		AI:             row.AI,
		AggroRange:     row.AggroRange,
		LeashRange:     row.LeashRange,
		SpawnDistance:  row.SpawnDistance,
		MovementType:   row.MovementType,
	}
}

func allocMonsterSessionID() int16 {
	for {
		id := atomic.AddInt32(&nextMonsterSessionID, 1)
		if id > 0 && id < maxPositiveSessionID {
			return int16(id)
		}
		atomic.CompareAndSwapInt32(&nextMonsterSessionID, id, defaultMonsterSessionIDStart)
	}
}

func allocMonsterWorldEntityID() int32 {
	return allocWorldEntityID()
}

func (w *worldMgr) LoadMonsterSpawns(rows []MonsterSpawnRow) int {
	loaded := 0
	for _, row := range rows {
		gm := w.GetMap(row.MapID)
		if gm == nil {
			log.Printf("[Monster] ignoring spawn=%d entry=%d: map %d is not registered", row.SpawnID, row.EntryID, row.MapID)
			continue
		}
		spawn := newMonsterSpawn(row)
		gm.RegisterMonsterSpawn(spawn)
		if _, spawned, err := gm.SpawnMonster(spawn); err != nil {
			log.Printf("[Monster] spawn=%d entry=%d failed: %v", spawn.SpawnID, spawn.EntryID, err)
			continue
		} else if spawned {
			loaded++
		}
	}
	return loaded
}

func (w *worldMgr) ClearMonsterSpawns() int {
	removed := 0
	for _, gm := range w.Maps() {
		removed += gm.ClearMonsterSpawns()
	}
	return removed
}

func (m *GameMap) RegisterMonsterSpawn(spawn *MonsterSpawn) {
	if spawn == nil {
		return
	}
	m.mu.Lock()
	m.spawns[spawn.SpawnID] = spawn
	m.mu.Unlock()
}

func (m *GameMap) SpawnMonster(spawn *MonsterSpawn) (*Monster, bool, error) {
	if spawn == nil {
		return nil, false, fmt.Errorf("spawn is nil")
	}
	if spawn.MapID != m.Template.ID {
		return nil, false, fmt.Errorf("spawn map %d does not match runtime map %d", spawn.MapID, m.Template.ID)
	}

	monster := newMonsterFromSpawn(spawn)
	m.mu.Lock()
	if spawn.ActiveSessionID != 0 {
		if active, ok := m.monsters[spawn.ActiveSessionID]; ok {
			m.mu.Unlock()
			return active, false, nil
		}
	}
	m.monsters[monster.SessionID] = monster
	spawn.ActiveSessionID = monster.SessionID
	spawn.RespawnReadyAt = time.Time{}
	m.mu.Unlock()

	if m.PlayerCount() > 0 {
		m.Start()
		m.broadcastMonsterVisible(monster)
	}
	debugMonsterSpawnf("spawned entry=%d session=%d spawn=%d map=%d x=%d y=%d",
		monster.EntryID, monster.SessionID, monster.SpawnID, monster.MapID, monster.LocalX, monster.LocalY)
	return monster, true, nil
}

func debugMonsterSpawnf(format string, args ...any) {
	if !visibilityDebugEnabled {
		return
	}
	log.Printf("[Monster] "+format, args...)
}

func (m *GameMap) ClearMonsterSpawns() int {
	var removed []*Monster
	m.mu.Lock()
	for sessionID, monster := range m.monsters {
		if monster.SpawnID == 0 {
			continue
		}
		delete(m.monsters, sessionID)
		m.forgetMonsterLocked(sessionID)
		monster.State = MonsterStateDead
		monster.Health = 0
		removed = append(removed, monster)
	}
	m.spawns = make(map[uint32]*MonsterSpawn)
	m.mu.Unlock()

	for _, monster := range removed {
		m.broadcastMonsterState(monster)
	}
	return len(removed)
}

func (m *GameMap) AddMonster(monster *Monster) {
	if monster == nil {
		return
	}
	m.mu.Lock()
	m.monsters[monster.SessionID] = monster
	if monster.SpawnID != 0 {
		if spawn := m.spawns[monster.SpawnID]; spawn != nil {
			spawn.ActiveSessionID = monster.SessionID
		}
	}
	m.mu.Unlock()
	if m.PlayerCount() > 0 {
		m.Start()
		m.broadcastMonsterVisible(monster)
	}
}

func (m *GameMap) RemoveMonster(sessionID int16) (*Monster, bool) {
	m.mu.Lock()
	monster, ok := m.monsters[sessionID]
	if ok {
		delete(m.monsters, sessionID)
		m.forgetMonsterLocked(sessionID)
		if monster.SpawnID != 0 {
			if spawn := m.spawns[monster.SpawnID]; spawn != nil && spawn.ActiveSessionID == sessionID {
				spawn.ActiveSessionID = 0
			}
		}
	}
	m.mu.Unlock()
	return monster, ok
}

func (m *GameMap) KillMonster(monster *Monster) {
	if monster == nil {
		return
	}
	monster.State = MonsterStateDead
	monster.Health = 0
	m.broadcastMonsterState(monster)

	removed, ok := m.RemoveMonster(monster.SessionID)
	if !ok {
		return
	}
	if removed.SpawnID != 0 {
		m.scheduleMonsterRespawn(removed.SpawnID)
	}
}

func (m *GameMap) ClearMonsterTargetsFor(sessionID int16) {
	if sessionID <= 0 {
		return
	}
	m.mu.Lock()
	for _, monster := range m.monsters {
		if monster.TargetSession == sessionID {
			monster.TargetSession = 0
		}
	}
	m.mu.Unlock()
}

func (m *GameMap) scheduleMonsterRespawn(spawnID uint32) {
	m.mu.RLock()
	spawn := m.spawns[spawnID]
	m.mu.RUnlock()
	if spawn == nil || spawn.RespawnSeconds <= 0 {
		return
	}

	log.Printf("[Monster] spawn=%d entry=%d respawn scheduled in %ds", spawn.SpawnID, spawn.EntryID, spawn.RespawnSeconds)
	go func(expected *MonsterSpawn) {
		time.Sleep(time.Duration(expected.RespawnSeconds) * time.Second)

		m.mu.Lock()
		current := m.spawns[spawnID]
		if current == nil || current != expected || current.ActiveSessionID != 0 {
			m.mu.Unlock()
			return
		}
		if len(m.characters) == 0 {
			current.RespawnReadyAt = time.Now()
			m.mu.Unlock()
			log.Printf("[Monster] spawn=%d entry=%d respawn deferred until map has players", current.SpawnID, current.EntryID)
			return
		}
		current.RespawnReadyAt = time.Time{}
		m.mu.Unlock()

		if _, _, err := m.SpawnMonster(current); err != nil {
			log.Printf("[Monster] spawn=%d respawn failed: %v", spawnID, err)
		}
	}(spawn)
}

func (m *GameMap) spawnDueMonsters() {
	now := time.Now()
	var due []*MonsterSpawn

	m.mu.Lock()
	for _, spawn := range m.spawns {
		if spawn == nil || spawn.ActiveSessionID != 0 || spawn.RespawnReadyAt.IsZero() {
			continue
		}
		if spawn.RespawnReadyAt.After(now) {
			continue
		}
		spawn.RespawnReadyAt = time.Time{}
		due = append(due, spawn)
	}
	m.mu.Unlock()

	for _, spawn := range due {
		if _, _, err := m.SpawnMonster(spawn); err != nil {
			log.Printf("[Monster] spawn=%d deferred respawn failed: %v", spawn.SpawnID, err)
		}
	}
}

func (m *GameMap) Monsters() []*Monster {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Monster, 0, len(m.monsters))
	for _, monster := range m.monsters {
		out = append(out, monster)
	}
	return out
}

func (m *GameMap) FindMonster(selector string) (*Monster, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, false
	}
	id, err := strconv.ParseUint(selector, 10, 32)
	if err != nil {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if id <= 0x7FFF {
		if monster, ok := m.monsters[int16(id)]; ok {
			return monster, true
		}
	}
	for _, monster := range m.monsters {
		if uint64(monster.EntryID) == id || uint64(monster.SpawnID) == id {
			return monster, true
		}
	}
	return nil, false
}

func (m *GameMap) FindMonsterBySessionID(sessionID int16) (*Monster, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	monster, ok := m.monsters[sessionID]
	return monster, ok
}

func (m *GameMap) FindMonsterByEntryID(entryID uint16) (*Monster, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, monster := range m.monsters {
		if monster.EntryID == entryID {
			return monster, true
		}
	}
	return nil, false
}

func (m *GameMap) FindMonsterBySpawnID(spawnID uint32) (*Monster, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, monster := range m.monsters {
		if monster.SpawnID == spawnID {
			return monster, true
		}
	}
	return nil, false
}

func (m *GameMap) FindMonsterByClientTargetID(targetID uint16) (*Monster, bool) {
	if targetID == 0 {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if monster, ok := m.monsters[int16(targetID)]; ok {
		return monster, true
	}
	for _, monster := range m.monsters {
		if monster.EntryID == targetID ||
			uint16(monster.SpawnID) == targetID ||
			uint16(monster.WorldEntityID) == targetID {
			return monster, true
		}
	}
	return nil, false
}

func (m *GameMap) AggroMonster(monster *Monster, target *Client) {
	if monster == nil || target == nil || target.Char == nil {
		return
	}
	m.mu.Lock()
	if m.monsters[monster.SessionID] == monster && monster.State == MonsterStateOK {
		monster.TargetSession = target.Char.SessionID
	}
	m.mu.Unlock()
}

func (m *GameMap) DamageMonster(attacker *Client, monster *Monster, damage int32) bool {
	return m.damageMonster(attacker, monster, damage, true)
}

func (m *GameMap) DamageMonsterFromSkill(attacker *Client, monster *Monster, damage int32) bool {
	return m.damageMonster(attacker, monster, damage, false)
}

func (m *GameMap) damageMonster(attacker *Client, monster *Monster, damage int32, sendBasicDamage bool) bool {
	if attacker == nil || attacker.Char == nil || monster == nil || damage <= 0 {
		return false
	}

	var killed bool
	m.mu.Lock()
	if m.monsters[monster.SessionID] != monster || monster.State != MonsterStateOK {
		m.mu.Unlock()
		return false
	}
	monster.TargetSession = attacker.Char.SessionID
	monster.Health -= damage
	if monster.Health <= 0 {
		monster.Health = 0
		killed = true
	}
	m.mu.Unlock()

	if sendBasicDamage {
		m.sendMonsterTakesDamage(attacker, monster, damage)
	}
	if killed {
		grantMonsterKillExp(attacker, monster)
		m.dropMonsterLoot(attacker, monster)
		m.KillMonster(monster)
	}
	return killed
}

func (m *GameMap) updateMonsterAI(chars []*Client, now time.Time) {
	if len(chars) == 0 {
		return
	}
	for _, monster := range m.Monsters() {
		if monster.NpcServerOwned {
			continue
		}
		if monster.State != MonsterStateOK || monster.Health <= 0 {
			continue
		}
		arrived := m.advanceMonsterMovement(monster, now)
		target := monsterAITarget(monster, chars)
		if target == nil {
			if arrived {
				sendMonsterMovementComplete(m, monster)
			}
			m.updateMonsterRoam(monster, now)
			continue
		}

		if monsterHomeDistance(monster) > monster.LeashRange {
			m.clearMonsterTarget(monster)
			m.returnMonsterHome(monster, now)
			continue
		}

		if m.monsterUseSkill(monster, target, now) {
			continue
		}
		if monsterCanAttackClient(monster, target) {
			m.monsterAttackPlayer(monster, target, now)
			continue
		}
		destX, destY := monsterChaseDestination(monster, target)
		m.moveMonsterToward(monster, destX, destY, now, 0, monsterMoveTypeRun)
	}
}

func monsterAITarget(monster *Monster, chars []*Client) *Client {
	if monster.TargetSession != 0 {
		for _, c := range chars {
			if isValidMonsterTarget(c) && c.Char.SessionID == monster.TargetSession {
				return c
			}
		}
		monster.TargetSession = 0
	}

	var best *Client
	bestDistance := monster.AggroRange
	for _, c := range chars {
		if !isValidMonsterTarget(c) {
			continue
		}
		distance := monsterDistanceToClient(monster, c)
		if distance <= bestDistance {
			best = c
			bestDistance = distance
		}
	}
	if best != nil {
		monster.TargetSession = best.Char.SessionID
	}
	return best
}

func (m *GameMap) updateMonsterRoam(monster *Monster, now time.Time) {
	if monster == nil || monster.MovementType == 0 || strings.EqualFold(monster.AI, "idle") {
		return
	}
	if monster.RoamRadius <= 0 {
		m.returnMonsterHome(monster, now)
		return
	}
	if monster.RoamTargetX != 0 || monster.RoamTargetY != 0 {
		if monster.LocalX == monster.RoamTargetX && monster.LocalY == monster.RoamTargetY {
			monster.RoamTargetX = 0
			monster.RoamTargetY = 0
			monster.NextRoamAt = now.Add(randomRoamInterval())
			return
		}
		m.moveMonsterToward(monster, monster.RoamTargetX, monster.RoamTargetY, now, 0, monsterMoveTypeWalk)
		return
	}
	if now.Before(monster.NextRoamAt) {
		return
	}
	x, y := randomRoamPoint(monster.HomeX, monster.HomeY, monster.RoamRadius)
	if x == monster.LocalX && y == monster.LocalY {
		monster.NextRoamAt = now.Add(randomRoamInterval())
		return
	}
	monster.RoamTargetX = x
	monster.RoamTargetY = y
	m.moveMonsterToward(monster, x, y, now, 0, monsterMoveTypeWalk)
}

func isValidMonsterTarget(c *Client) bool {
	return c != nil && c.Char != nil && c.Char.HP > 0
}

func (m *GameMap) clearMonsterTarget(monster *Monster) {
	m.mu.Lock()
	if m.monsters[monster.SessionID] == monster {
		monster.TargetSession = 0
	}
	m.mu.Unlock()
}

func (m *GameMap) returnMonsterHome(monster *Monster, now time.Time) {
	if monster.LocalX == monster.HomeX && monster.LocalY == monster.HomeY {
		return
	}
	monster.RoamTargetX = 0
	monster.RoamTargetY = 0
	m.moveMonsterToward(monster, monster.HomeX, monster.HomeY, now, 0, monsterMoveTypeRun)
}

func (m *GameMap) moveMonsterToward(monster *Monster, destX int16, destY int16, now time.Time, maxDistance float64, moveType byte) {
	if now.Before(monster.NextMoveAt) {
		return
	}
	if monster.IsMoving && monster.MoveDestX == destX && monster.MoveDestY == destY && monster.MoveType == moveType {
		return
	}
	fromX := monster.LocalX
	fromY := monster.LocalY
	nextX, nextY := nextMonsterDestination(monster.LocalX, monster.LocalY, destX, destY, maxDistance)
	if nextX == monster.LocalX && nextY == monster.LocalY {
		return
	}
	moveMS := monsterMoveUnitMS(monster, moveType)
	moveDuration := monsterMoveDuration(monster, moveType, fromX, fromY, nextX, nextY)

	m.mu.Lock()
	if m.monsters[monster.SessionID] != monster || monster.State != MonsterStateOK {
		m.mu.Unlock()
		return
	}
	monster.IsMoving = true
	monster.MoveType = moveType
	monster.MoveFromX = float64(fromX)
	monster.MoveFromY = float64(fromY)
	monster.MoveDestX = nextX
	monster.MoveDestY = nextY
	monster.MoveStartedAt = now
	monster.MoveDuration = moveDuration
	monster.NextMoveAt = now.Add(defaultMonsterMoveInterval)
	m.mu.Unlock()

	sendMonsterMoveOrAttack(m, monster, 0, 0, fromX, fromY, nextX, nextY, false, moveType, moveMS)
}

func (m *GameMap) monsterAttackPlayer(monster *Monster, target *Client, now time.Time) {
	if target == nil || target.Char == nil || now.Before(monster.NextAttackAt) {
		return
	}
	if target.Char.HP <= 0 {
		m.clearMonsterTarget(monster)
		return
	}

	m.mu.Lock()
	if m.monsters[monster.SessionID] != monster || monster.State != MonsterStateOK {
		m.mu.Unlock()
		return
	}
	monster.NextAttackAt = now.Add(monster.AttackInterval)
	monster.IsMoving = false
	monster.MoveDestX = monster.LocalX
	monster.MoveDestY = monster.LocalY
	m.mu.Unlock()

	damage := monsterRollDamage(monster)
	if target.Char.HP < damage {
		damage = target.Char.HP
	}
	sendMonsterMoveOrAttack(m, monster, target.Char.SessionID, damage, monster.LocalX, monster.LocalY, monster.LocalX, monster.LocalY, true, monster.MoveType, 0)
	if damageCharacter(target, damage, fmt.Sprintf("monster:%d", monster.EntryID)) {
		m.clearMonsterTarget(monster)
	}
}

func (m *GameMap) monsterUseSkill(monster *Monster, target *Client, now time.Time) bool {
	if monster == nil || target == nil || target.Char == nil || now.Before(monster.NextSkillAt) {
		return false
	}
	if monster.Level < 3 && monster.MaxHealth < defaultMonsterHealth*4 {
		return false
	}
	if !monsterCanAttackClient(monster, target) {
		return false
	}

	damage := int32(math.Round(float64(monsterRollDamage(monster)) * 1.5))
	if damage < defaultMonsterAttackDamage+1 {
		damage = defaultMonsterAttackDamage + 1
	}
	if target.Char.HP < damage {
		damage = target.Char.HP
	}

	m.mu.Lock()
	if m.monsters[monster.SessionID] != monster || monster.State != MonsterStateOK {
		m.mu.Unlock()
		return false
	}
	monster.NextSkillAt = now.Add(defaultMonsterSkillCooldown + time.Duration(rand.Intn(4000))*time.Millisecond)
	monster.NextAttackAt = now.Add(monster.AttackInterval / 2)
	monster.IsMoving = false
	monster.MoveDestX = monster.LocalX
	monster.MoveDestY = monster.LocalY
	m.mu.Unlock()

	sendMonsterMoveOrAttack(m, monster, target.Char.SessionID, damage, monster.LocalX, monster.LocalY, monster.LocalX, monster.LocalY, true, monster.MoveType, 0)
	if damageCharacter(target, damage, fmt.Sprintf("monster-skill:%d", monster.EntryID)) {
		m.clearMonsterTarget(monster)
	}
	log.Printf("[MonsterAI] entry=%d session=%d used skill target=%q damage=%d", monster.EntryID, monster.SessionID, target.Char.Name, damage)
	return true
}

func monsterRollDamage(monster *Monster) int32 {
	if monster == nil {
		return defaultMonsterAttackDamage
	}
	minDamage := monster.MinDamage
	if minDamage <= 0 {
		minDamage = float64(defaultMonsterAttackDamage)
	}
	maxDamage := monster.MaxDamage
	if maxDamage < minDamage {
		maxDamage = minDamage
	}
	if maxDamage == minDamage {
		return int32(math.Round(minDamage))
	}
	return int32(math.Round(minDamage + rand.Float64()*(maxDamage-minDamage)))
}

func randomRoamInterval() time.Duration {
	delta := defaultMonsterRoamIntervalMax - defaultMonsterRoamIntervalMin
	if delta <= 0 {
		return defaultMonsterRoamIntervalMin
	}
	return defaultMonsterRoamIntervalMin + time.Duration(rand.Int63n(int64(delta)))
}

func randomRoamPoint(homeX int16, homeY int16, radius float64) (int16, int16) {
	if radius <= 0 {
		return homeX, homeY
	}
	angle := rand.Float64() * math.Pi * 2
	distance := 1 + rand.Float64()*radius
	return int16(math.Round(float64(homeX) + math.Cos(angle)*distance)),
		int16(math.Round(float64(homeY) + math.Sin(angle)*distance))
}

func nextMonsterDestination(fromX int16, fromY int16, destX int16, destY int16, maxDistance float64) (int16, int16) {
	dx := float64(destX - fromX)
	dy := float64(destY - fromY)
	distance := math.Hypot(dx, dy)
	if distance <= 0.01 {
		return fromX, fromY
	}
	if maxDistance <= 0 || distance <= maxDistance {
		return destX, destY
	}
	return int16(math.Round(float64(fromX) + dx/distance*maxDistance)),
		int16(math.Round(float64(fromY) + dy/distance*maxDistance))
}

func monsterChaseDestination(monster *Monster, target *Client) (int16, int16) {
	if monster == nil || target == nil || target.Char == nil {
		if monster == nil {
			return 0, 0
		}
		return monster.LocalX, monster.LocalY
	}
	targetX := float64(clientLocalX(target))
	targetY := float64(clientLocalY(target))
	dx := targetX - float64(monster.LocalX)
	dy := targetY - float64(monster.LocalY)
	distance := math.Hypot(dx, dy)
	if distance <= 0.01 {
		return int16(math.Round(targetX)), int16(math.Round(targetY))
	}
	desiredDistance := monster.AttackRange * 0.8
	if desiredDistance < 1 {
		desiredDistance = 1
	}
	x := targetX - dx/distance*desiredDistance
	y := targetY - dy/distance*desiredDistance
	return int16(math.Round(x)), int16(math.Round(y))
}

func (m *GameMap) advanceMonsterMovement(monster *Monster, now time.Time) bool {
	if monster == nil || !monster.IsMoving {
		return false
	}
	duration := monster.MoveDuration
	if duration <= 0 {
		duration = time.Duration(monsterMoveMinMillis) * time.Millisecond
	}
	elapsed := now.Sub(monster.MoveStartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	ratio := float64(elapsed) / float64(duration)
	if ratio > 1 {
		ratio = 1
	}
	x := monster.MoveFromX + (float64(monster.MoveDestX)-monster.MoveFromX)*ratio
	y := monster.MoveFromY + (float64(monster.MoveDestY)-monster.MoveFromY)*ratio

	m.mu.Lock()
	if m.monsters[monster.SessionID] != monster {
		m.mu.Unlock()
		return false
	}
	monster.LocalX = int16(math.Round(x))
	monster.LocalY = int16(math.Round(y))
	arrived := false
	if ratio >= 1 {
		monster.LocalX = monster.MoveDestX
		monster.LocalY = monster.MoveDestY
		monster.IsMoving = false
		arrived = true
	}
	m.mu.Unlock()
	return arrived
}

func monsterMoveUnitMS(monster *Monster, moveType byte) int16 {
	speed := monsterMoveSpeed(monster, moveType)
	if speed <= 0 {
		speed = defaultMonsterWalkSpeed
	}
	moveMS := int16(math.Round(1000 / speed))
	if moveMS < 1 {
		return 1
	}
	return moveMS
}

func monsterMoveDuration(monster *Monster, moveType byte, fromX int16, fromY int16, destX int16, destY int16) time.Duration {
	distance := distance2D(float64(fromX), float64(fromY), float64(destX), float64(destY))
	moveMS := int16(math.Round(distance * float64(monsterMoveUnitMS(monster, moveType))))
	if moveMS < monsterMoveMinMillis {
		moveMS = monsterMoveMinMillis
	}
	if moveMS > monsterMoveMaxMillis {
		moveMS = monsterMoveMaxMillis
	}
	return time.Duration(moveMS) * time.Millisecond
}

func monsterMoveSpeed(monster *Monster, moveType byte) float64 {
	if monster == nil {
		if moveType == monsterMoveTypeRun {
			return defaultMonsterRunSpeed
		}
		return defaultMonsterWalkSpeed
	}
	if moveType == monsterMoveTypeRun {
		if monster.RunSpeed > 0 {
			return monster.RunSpeed
		}
		return defaultMonsterRunSpeed
	}
	if monster.WalkSpeed > 0 {
		return monster.WalkSpeed
	}
	return defaultMonsterWalkSpeed
}

func monsterDistanceToClient(monster *Monster, c *Client) float64 {
	return distance2D(float64(monster.LocalX), float64(monster.LocalY), float64(clientLocalX(c)), float64(clientLocalY(c)))
}

func monsterCanAttackClient(monster *Monster, c *Client) bool {
	if monster == nil || c == nil || c.Char == nil {
		return false
	}
	attackRange := monster.AttackRange
	if attackRange <= 0 {
		attackRange = defaultMonsterAttackRange
	}
	dx := math.Abs(float64(monster.LocalX - clientLocalX(c)))
	dy := math.Abs(float64(monster.LocalY - clientLocalY(c)))
	return dx <= attackRange && dy <= attackRange
}

func monsterHomeDistance(monster *Monster) float64 {
	return distance2D(float64(monster.LocalX), float64(monster.LocalY), float64(monster.HomeX), float64(monster.HomeY))
}

func clientLocalX(c *Client) int16 {
	if c == nil || c.Char == nil {
		return 0
	}
	return int16(math.Round(float64(asda2X(c.Char.X, c.Char.MapID))))
}

func clientLocalY(c *Client) int16 {
	if c == nil || c.Char == nil {
		return 0
	}
	return int16(math.Round(float64(asda2Y(c.Char.Y, c.Char.MapID))))
}

func distance2D(ax float64, ay float64, bx float64, by float64) float64 {
	return math.Hypot(ax-bx, ay-by)
}

func (m *GameMap) sendVisibleMonstersTo(c *Client) int {
	return m.RefreshMonsterVisibility(c)
}

func (m *GameMap) broadcastMonsterVisible(monster *Monster) {
	if monster == nil {
		return
	}

	var viewers []*Client
	m.mu.Lock()
	for _, c := range m.characters {
		if !clientCanSeeMonster(c, monster) {
			continue
		}
		known := m.ensureKnownMonsterSetLocked(c.ID)
		if _, wasKnown := known[monster.SessionID]; wasKnown {
			continue
		}
		known[monster.SessionID] = struct{}{}
		viewers = append(viewers, c)
	}
	m.mu.Unlock()

	for _, c := range viewers {
		sendMonsterVisible(c, monster)
	}
}

func (w *worldMgr) RefreshMonsterVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	if gm := w.GetMap(c.Char.MapID); gm != nil {
		gm.RefreshMonsterVisibility(c)
	}
}

func (w *worldMgr) ResyncMonsterVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	if gm := w.GetMap(c.Char.MapID); gm != nil {
		gm.ResyncMonsterVisibility(c)
	}
}

func (w *worldMgr) TickMonsterVisibility(c *Client, now time.Time) {
	if c == nil || c.Char == nil {
		return
	}
	if gm := w.GetMap(c.Char.MapID); gm != nil {
		gm.TickMonsterVisibility(c, now)
	}
}

func (m *GameMap) RefreshMonsterVisibility(c *Client) int {
	if npcServerClient != nil {
		return m.refreshMonsterVisibilityFromServer(c, false)
	}
	monsters := m.refreshMonsterVisibility(c, false)
	if len(monsters) > 0 {
		debugMonsterVisibility(c, "refresh", len(monsters))
	}
	for _, monster := range monsters {
		sendMonsterVisible(c, monster)
	}
	return len(monsters)
}

func (m *GameMap) ResyncMonsterVisibility(c *Client) {
	if npcServerClient != nil {
		m.refreshMonsterVisibilityFromServer(c, true)
		return
	}
	m.RefreshMonsterVisibility(c)
}

func (m *GameMap) TickMonsterVisibility(c *Client, now time.Time) {
	if npcServerClient == nil {
		return
	}
	if c == nil || c.Char == nil {
		return
	}
	if !m.monsterResyncDue(c.ID, now) {
		return
	}
	m.RefreshMonsterVisibility(c)
}

func (m *GameMap) monsterResyncDue(viewerID uint32, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if next := m.nextMonsterResync[viewerID]; !next.IsZero() && now.Before(next) {
		return false
	}
	interval := monsterVisibilityResyncInterval
	if npcServerClient != nil {
		interval = npcServerMonsterResyncInterval
	}
	m.nextMonsterResync[viewerID] = now.Add(interval)
	return true
}

func (m *GameMap) markNextMonsterResync(viewerID uint32, next time.Time) {
	m.mu.Lock()
	m.nextMonsterResync[viewerID] = next
	m.mu.Unlock()
}

func (m *GameMap) refreshMonsterVisibility(c *Client, forceResend bool) []*Monster {
	if c == nil || c.Char == nil {
		return nil
	}

	var visible []*Monster
	m.mu.Lock()
	if _, ok := m.characters[c.ID]; !ok {
		m.mu.Unlock()
		return nil
	}
	known := m.ensureKnownMonsterSetLocked(c.ID)
	for sessionID, monster := range m.monsters {
		canSee := clientCanSeeMonster(c, monster)
		_, wasKnown := known[sessionID]
		if canSee {
			if forceResend || !wasKnown {
				known[sessionID] = struct{}{}
				visible = append(visible, monster)
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

func (m *GameMap) refreshMonsterVisibilityFromServer(c *Client, forceResend bool) int {
	_, monsters, err := npcServerClient.VisibleWorld(c, forceResend)
	if err != nil {
		logNpcServerUnavailable(err)
		m.flushKnownMonstersFor(c)
		return 0
	}

	m.mu.Lock()
	if _, ok := m.characters[c.ID]; !ok {
		m.mu.Unlock()
		return 0
	}
	known := m.ensureKnownMonsterSetLocked(c.ID)
	current := make(map[int16]struct{}, len(monsters))
	visible := make([]*Monster, 0, len(monsters))
	disappeared := make([]*Monster, 0)
	for _, monster := range monsters {
		if monster == nil {
			continue
		}
		current[monster.SessionID] = struct{}{}
		if existing := m.monsters[monster.SessionID]; existing != nil {
			monster = existing
		} else {
			m.monsters[monster.SessionID] = monster
		}
		_, wasKnown := known[monster.SessionID]
		canSee := clientCanSeeMonster(c, monster)
		if canSee && (forceResend || !wasKnown) {
			known[monster.SessionID] = struct{}{}
			visible = append(visible, monster)
			continue
		}
		if !canSee && wasKnown {
			disappeared = append(disappeared, monster)
			delete(known, monster.SessionID)
		}
	}
	for sessionID := range known {
		if _, ok := current[sessionID]; !ok {
			if monster := m.monsters[sessionID]; monster != nil {
				disappeared = append(disappeared, monster)
			}
			delete(known, sessionID)
		}
	}
	m.mu.Unlock()

	for _, monster := range disappeared {
		sendEntityDelete(c, monster.SessionID, monster.WorldEntityID)
	}
	if len(visible) > 0 {
		debugMonsterVisibility(c, "remote", len(visible))
		for _, monster := range visible {
			sendMonsterVisible(c, monster)
		}
	}
	return len(visible)
}

func (m *GameMap) flushKnownMonstersFor(c *Client) {
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
	deleted := make(map[int16]struct{})
	for viewerID, known := range m.knownMonsters {
		viewer := m.characters[viewerID]
		for sessionID := range known {
			if monster := m.monsters[sessionID]; monster != nil && viewer != nil {
				actions = append(actions, deleteAction{
					viewer:        viewer,
					sessionID:     monster.SessionID,
					worldEntityID: monster.WorldEntityID,
				})
			}
			deleted[sessionID] = struct{}{}
			delete(known, sessionID)
		}
	}
	m.monsters = make(map[int16]*Monster)
	for _, viewer := range m.characters {
		if viewer == nil || viewer.Char == nil {
			continue
		}
		if _, ok := deleted[viewer.Char.TargetID]; ok {
			viewer.Char.TargetID = -1
			viewer.Char.IsFighting = false
		}
	}
	m.mu.Unlock()

	for _, action := range actions {
		sendEntityDelete(action.viewer, action.sessionID, action.worldEntityID)
	}
	if len(actions) > 0 {
		log.Printf("[NpcServer] hid %d monster visibility record(s) after remote visibility loss", len(actions))
	}
}

func (m *GameMap) refreshMonsterVisibilityAround(monster *Monster) []*Client {
	if monster == nil {
		return nil
	}

	var newlyVisible []*Client
	m.mu.Lock()
	for viewerID, c := range m.characters {
		known := m.ensureKnownMonsterSetLocked(viewerID)
		_, wasKnown := known[monster.SessionID]
		if clientCanSeeMonster(c, monster) {
			if !wasKnown {
				known[monster.SessionID] = struct{}{}
				newlyVisible = append(newlyVisible, c)
			}
			continue
		}
		if wasKnown {
			delete(known, monster.SessionID)
		}
	}
	m.mu.Unlock()
	return newlyVisible
}

func (m *GameMap) monsterKnownViewers(monster *Monster) []*Client {
	if monster == nil {
		return nil
	}
	var viewers []*Client
	m.mu.RLock()
	for viewerID, c := range m.characters {
		if !clientCanSeeMonster(c, monster) {
			continue
		}
		if m.monsterKnownLocked(viewerID, monster.SessionID) {
			viewers = append(viewers, c)
		}
	}
	m.mu.RUnlock()
	return viewers
}

func (m *GameMap) sendToMonsterKnownViewers(monster *Monster, p *PacketOut) {
	for _, c := range m.monsterKnownViewers(monster) {
		c.Send(p)
	}
}

func (m *GameMap) EnsureMonsterKnown(viewer *Client, monster *Monster) bool {
	if viewer == nil || viewer.Char == nil || monster == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.characters[viewer.ID] == nil || m.monsters[monster.SessionID] != monster {
		return false
	}
	if !clientCanSeeMonster(viewer, monster) {
		return false
	}
	m.ensureKnownMonsterSetLocked(viewer.ID)[monster.SessionID] = struct{}{}
	return true
}

func (m *GameMap) ensureKnownMonsterSetLocked(viewerID uint32) map[int16]struct{} {
	known := m.knownMonsters[viewerID]
	if known == nil {
		known = make(map[int16]struct{})
		m.knownMonsters[viewerID] = known
	}
	return known
}

func (m *GameMap) monsterKnownLocked(viewerID uint32, monsterSessionID int16) bool {
	known := m.knownMonsters[viewerID]
	if known == nil {
		return false
	}
	_, ok := known[monsterSessionID]
	return ok
}

func (m *GameMap) KnowsMonster(viewerID uint32, monsterSessionID int16) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monsterKnownLocked(viewerID, monsterSessionID)
}

func (m *GameMap) forgetMonsterLocked(monsterSessionID int16) {
	for _, known := range m.knownMonsters {
		delete(known, monsterSessionID)
	}
}

func clientCanSeeMonster(c *Client, monster *Monster) bool {
	if c == nil || c.Char == nil || monster == nil {
		return false
	}
	if monster.State != MonsterStateOK || monster.Health <= 0 {
		return false
	}
	if c.Char.MapID != monster.MapID {
		return false
	}
	return monsterDistanceToCharacter(monster, c.Char) < monsterVisibilityRange
}

func monsterDistanceToCharacter(monster *Monster, chr *Character) float64 {
	if monster == nil || chr == nil {
		return math.MaxFloat64
	}
	return distance2D(
		float64(monster.LocalX),
		float64(monster.LocalY),
		float64(asda2X(chr.X, chr.MapID)),
		float64(asda2Y(chr.Y, chr.MapID)),
	)
}

func debugMonsterVisibility(c *Client, phase string, sent int) {
	if c == nil || c.Char == nil {
		return
	}
	log.Printf("[MonsterVisibility] %s viewer=%s map=%d local=%.2f,%.2f sent=%d",
		phase,
		clientDebugLabel(c),
		c.Char.MapID,
		asda2X(c.Char.X, c.Char.MapID),
		asda2Y(c.Char.Y, c.Char.MapID),
		sent,
	)
}

func (m *GameMap) broadcastMonsterState(monster *Monster) {
	for _, c := range m.Characters() {
		sendMonsterStateChanged(c, monster)
	}
}

func sendMonsterVisible(c *Client, monster *Monster) {
	if c == nil || monster == nil {
		return
	}
	moveType := monster.MoveType
	if moveType == 0 {
		moveType = monsterMoveTypeWalk
	}
	moveMS := monsterMoveUnitMS(monster, moveType)
	currentX, currentY, destX, destY := monsterVisibleCoordinates(monster)
	p := NewPacket(MonstVisible)
	p.WriteInt16(monster.SessionID)
	p.WriteInt16(int16(monster.EntryID))
	p.WriteInt32(91)
	p.WriteUint8(2)
	p.WriteInt32(monster.Health)
	p.WriteInt16(moveMS)
	p.WriteInt16(15000)
	p.WriteBytes(monsterVisibleStab23)
	p.WriteInt16(currentX)
	p.WriteInt16(currentY)
	p.WriteInt16(destX)
	p.WriteInt16(destY)
	p.WriteBytes(monsterVisibleStab33)
	p.WriteInt32(91)
	p.WriteInt16(47)
	p.WriteInt16(150)
	p.WriteInt32(0)
	p.WriteInt16(1500)
	p.WriteInt16(1000)
	p.WriteInt16(1000)
	c.Send(p)
}

func monsterVisibleCoordinates(monster *Monster) (currentX int16, currentY int16, destX int16, destY int16) {
	if monster == nil {
		return 0, 0, 0, 0
	}
	currentX = monster.LocalX
	currentY = monster.LocalY
	if monster.IsMoving {
		return currentX, currentY, monster.MoveDestX, monster.MoveDestY
	}
	return currentX, currentY, currentX, currentY
}

func sendMonsterMovementComplete(gm *GameMap, monster *Monster) {
	if gm == nil || monster == nil || monster.State != MonsterStateOK || monster.Health <= 0 {
		return
	}
	moveType := monster.MoveType
	if moveType == 0 {
		moveType = monsterMoveTypeWalk
	}
	sendMonsterMoveOrAttack(gm, monster, 0, 0, monster.LocalX, monster.LocalY, monster.LocalX, monster.LocalY, false, moveType, 1)
}

func sendMonsterStateChanged(c *Client, monster *Monster) {
	if c == nil || monster == nil {
		return
	}
	p := NewPacket(MonstrStateChanged)
	p.WriteBytes(monsterStateStab6)
	p.WriteInt16(monster.SessionID)
	p.WriteInt32(int32(monster.State))
	for i := 0; i < 28; i++ {
		p.WriteInt16(-1)
	}
	for i := 0; i < 28; i++ {
		p.WriteUint8(0)
	}
	p.WriteInt32(monster.Health)
	p.WriteInt16(monster.LocalX)
	p.WriteInt16(monster.LocalY)
	c.Send(p)
}

func sendMonsterMoveOrAttack(gm *GameMap, monster *Monster, targetSession int16, damage int32, fromX int16, fromY int16, destX int16, destY int16, isAttack bool, moveType byte, overrideMoveMS int16) {
	if gm == nil || monster == nil {
		return
	}
	if moveType == 0 {
		moveType = monsterMoveTypeRun
	}
	moveTime := monsterMoveUnitMS(monster, moveType)
	if overrideMoveMS > 0 {
		moveTime = overrideMoveMS
	}
	p := NewPacket(MonstMove)
	p.WriteInt16(targetSession)
	p.WriteInt16(int16(monster.EntryID))
	p.WriteInt16(monster.SessionID)
	if isAttack {
		p.WriteUint8(3)
		p.WriteInt16(fromX)
		p.WriteInt16(fromY)
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteInt16(0)
	} else {
		p.WriteUint8(moveType)
		p.WriteInt16(fromX)
		p.WriteInt16(fromY)
		p.WriteInt16(destX)
		p.WriteInt16(destY)
		p.WriteInt16(moveTime)
		p.WriteInt16(10000)
	}
	p.WriteInt16(10000)
	p.WriteInt32(damage)
	p.WriteInt16(clampInt16(monster.Health))
	if monster.Health <= 0 {
		p.WriteInt16(-1)
	} else {
		p.WriteInt16(0)
	}
	p.WriteBytes(monsterMoveStab35)
	gm.sendToMonsterKnownViewers(monster, p)
	for _, c := range gm.refreshMonsterVisibilityAround(monster) {
		sendMonsterVisible(c, monster)
	}
}

func (m *GameMap) sendMonsterTakesDamage(attacker *Client, monster *Monster, damage int32) {
	if attacker == nil || attacker.Char == nil || monster == nil {
		return
	}
	p := NewPacket(MonstrTakeDmg)
	p.WriteInt16(attacker.Char.SessionID)
	p.WriteInt16(monster.SessionID)
	p.WriteInt32(monster.WorldEntityID)
	p.WriteInt32(damage)
	p.WriteBytes(monsterDamageStab8)
	for _, viewer := range m.monsterDamageRecipients(attacker, monster) {
		viewer.Send(p)
	}
}

func (m *GameMap) monsterDamageRecipients(attacker *Client, monster *Monster) []*Client {
	if m == nil || attacker == nil || attacker.Char == nil {
		return nil
	}
	seen := make(map[uint32]struct{})
	recipients := make([]*Client, 0, 4)
	add := func(c *Client) {
		if c == nil || c.Char == nil {
			return
		}
		if _, ok := seen[c.ID]; ok {
			return
		}
		seen[c.ID] = struct{}{}
		recipients = append(recipients, c)
	}

	m.mu.RLock()
	if m.characters[attacker.ID] != nil {
		add(attacker)
		for viewerID, viewer := range m.characters {
			if viewer == nil || viewer.ID == attacker.ID || viewer.Char == nil {
				continue
			}
			if m.characterKnownLocked(viewerID, attacker.ID) || m.characterKnownLocked(attacker.ID, viewerID) {
				add(viewer)
			}
		}
	}
	m.mu.RUnlock()

	for _, viewer := range m.monsterKnownViewers(monster) {
		add(viewer)
	}
	return recipients
}

func clampInt16(value int32) int16 {
	if value > 32767 {
		return 32767
	}
	if value < -32768 {
		return -32768
	}
	return int16(value)
}

func gmSummonMonster(cmd relay.GMCommand) error {
	entryID, err := parseUint16Arg(cmd.Args, "monsterId")
	if err != nil {
		return err
	}
	mapID, err := parseUint16Arg(cmd.Args, "map")
	if err != nil {
		return err
	}
	localX, err := parseInt16Arg(cmd.Args, "x")
	if err != nil {
		return err
	}
	localY, err := parseInt16Arg(cmd.Args, "y")
	if err != nil {
		return err
	}

	gm := World.GetMap(mapID)
	if gm == nil {
		return fmt.Errorf("map %d is not registered", mapID)
	}
	monster := newMonster(entryID, mapID, localX, localY)
	gm.AddMonster(monster)
	log.Printf("[GM] %s summoned monster entry=%d session=%d map=%d x=%d y=%d", cmd.RequestedBy, entryID, monster.SessionID, mapID, localX, localY)
	return nil
}

func gmSummonMonsterNearPlayer(cmd relay.GMCommand) error {
	entryID, err := parseUint16Arg(cmd.Args, "monsterId")
	if err != nil {
		return err
	}
	targetName := strings.TrimSpace(cmd.Args["character"])
	if targetName == "" {
		return fmt.Errorf("character is required")
	}
	target := getClientByCharacterName(targetName)
	if target == nil || target.Char == nil {
		return fmt.Errorf("character %q is not online on this game server", targetName)
	}
	distance := int16(2)
	if strings.TrimSpace(cmd.Args["distance"]) != "" {
		distance, err = parseInt16Arg(cmd.Args, "distance")
		if err != nil {
			return err
		}
	}

	mapID := target.Char.MapID
	localX := int16(math.Round(float64(asda2X(target.Char.X, mapID)))) + distance
	localY := int16(math.Round(float64(asda2Y(target.Char.Y, mapID))))
	gm := World.GetMap(mapID)
	if gm == nil {
		return fmt.Errorf("map %d is not registered", mapID)
	}
	monster := newMonster(entryID, mapID, localX, localY)
	gm.AddMonster(monster)
	log.Printf("[GM] %s summoned monster entry=%d session=%d near %q map=%d x=%d y=%d",
		cmd.RequestedBy, entryID, monster.SessionID, target.Char.Name, mapID, localX, localY)
	return nil
}

func gmKillMonster(cmd relay.GMCommand) error {
	if value := strings.TrimSpace(cmd.Args["monsterSessionId"]); value != "" {
		id, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			return fmt.Errorf("monsterSessionId: %w", err)
		}
		return killMonsterWhere(cmd, func(gm *GameMap) (*Monster, bool) {
			return gm.FindMonsterBySessionID(int16(id))
		}, value)
	}
	if value := strings.TrimSpace(cmd.Args["spawnId"]); value != "" {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("spawnId: %w", err)
		}
		return killMonsterWhere(cmd, func(gm *GameMap) (*Monster, bool) {
			return gm.FindMonsterBySpawnID(uint32(id))
		}, value)
	}
	if value := strings.TrimSpace(cmd.Args["monsterId"]); value != "" {
		id, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("monsterId: %w", err)
		}
		return killMonsterWhere(cmd, func(gm *GameMap) (*Monster, bool) {
			return gm.FindMonsterByEntryID(uint16(id))
		}, value)
	}
	if value := strings.TrimSpace(cmd.Args["monster"]); value != "" {
		return killMonsterWhere(cmd, func(gm *GameMap) (*Monster, bool) {
			return gm.FindMonster(value)
		}, value)
	}
	return fmt.Errorf("monster, monsterSessionId, monsterId, or spawnId is required")
}

func killMonsterWhere(cmd relay.GMCommand, find func(*GameMap) (*Monster, bool), selector string) error {
	for _, gm := range World.Maps() {
		monster, ok := find(gm)
		if !ok {
			continue
		}
		gm.KillMonster(monster)
		log.Printf("[GM] %s killed monster entry=%d session=%d spawn=%d map=%d",
			cmd.RequestedBy, monster.EntryID, monster.SessionID, monster.SpawnID, monster.MapID)
		return nil
	}
	return fmt.Errorf("monster %q not found", selector)
}

func gmReloadMonsterSpawns(cmd relay.GMCommand) error {
	if err := reloadMonsterRuntime(gameChannel); err != nil {
		return err
	}
	log.Printf("[GM] %s reloaded monster spawns", cmd.RequestedBy)
	return nil
}

func buildMonsterVisibleStab33() []byte {
	out := make([]byte, 120)
	out[0] = 4
	for i := 36; i < 92; i++ {
		out[i] = 0xFF
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
