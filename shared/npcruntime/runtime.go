package npcruntime

import (
	"sync"
	"sync/atomic"
	"time"

	"asda2/shared/types"
)

const (
	WorldEntityIDStart       int32   = 200000
	DefaultNpcSessionIDStart int32   = 1000
	MaxPositiveSessionID     uint32  = 0x7FFF
	IgnoredNpcEntryID        uint16  = 145
	DefaultVisibilityRange   float64 = 100
	DefaultMonsterRange      float64 = 50
)

type Runtime struct {
	mu       sync.RWMutex
	channel  byte
	nextNPC  int32
	nextMon  int32
	nextWEID int32
	npcs     []*NPC
	monsters []*Monster
	players  map[uint32]Player
}

type Player struct {
	AccountID uint32    `json:"accountId"`
	SessionID int16     `json:"sessionId"`
	Character string    `json:"character"`
	MapID     uint16    `json:"mapId"`
	X         int16     `json:"x"`
	Y         int16     `json:"y"`
	Channel   int16     `json:"channel"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NPC struct {
	SessionID       int16                    `json:"sessionId"`
	WorldEntityID   int32                    `json:"worldEntityId"`
	SpawnID         uint32                   `json:"spawnId"`
	EntryID         uint16                   `json:"entryId"`
	Name            string                   `json:"name"`
	Kind            byte                     `json:"kind"`
	ClassGroup      byte                     `json:"classGroup"`
	IsTrainer       bool                     `json:"isTrainer"`
	InteractionKind types.NpcInteractionKind `json:"interactionKind"`
	MapID           uint16                   `json:"mapId"`
	LocalX          int16                    `json:"x"`
	LocalY          int16                    `json:"y"`
	Channel         int16                    `json:"channel"`
}

type Monster struct {
	SessionID      int16   `json:"sessionId"`
	WorldEntityID  int32   `json:"worldEntityId"`
	SpawnID        uint32  `json:"spawnId"`
	EntryID        uint16  `json:"entryId"`
	Name           string  `json:"name"`
	Level          byte    `json:"level"`
	MapID          uint16  `json:"mapId"`
	LocalX         int16   `json:"x"`
	LocalY         int16   `json:"y"`
	Health         int32   `json:"health"`
	MaxHealth      int32   `json:"maxHealth"`
	MoveMS         int16   `json:"moveMs"`
	WalkSpeed      float64 `json:"walkSpeed"`
	RunSpeed       float64 `json:"runSpeed"`
	RespawnSeconds int     `json:"respawnSeconds"`
	MinDamage      float64 `json:"minDamage"`
	MaxDamage      float64 `json:"maxDamage"`
	BaseAttackMS   int     `json:"baseAttackMs"`
	AggroRange     float64 `json:"aggroRange"`
	LeashRange     float64 `json:"leashRange"`
	SpawnDistance  float64 `json:"spawnDistance"`
	MovementType   int     `json:"movementType"`
	AI             string  `json:"ai"`
	Channel        int16   `json:"channel"`
}

type Snapshot struct {
	MapID        uint16     `json:"mapId"`
	Channel      int16      `json:"channel"`
	NpcCount     int        `json:"npcCount"`
	MonsterCount int        `json:"monsterCount"`
	Npcs         []*NPC     `json:"npcs"`
	Monsters     []*Monster `json:"monsters"`
}

func New(
	channel byte,
	npcTemplates []types.NpcTemplateRow,
	npcSpawns []types.NpcSpawnRow,
	monsterTemplates []types.MonsterTemplateRow,
	monsterSpawns []types.MonsterSpawnRow,
) *Runtime {
	rt := &Runtime{
		channel:  channel,
		nextNPC:  DefaultNpcSessionIDStart,
		nextMon:  20000,
		nextWEID: WorldEntityIDStart,
		players:  make(map[uint32]Player),
	}
	rt.Load(npcTemplates, npcSpawns, monsterTemplates, monsterSpawns)
	return rt
}

func (rt *Runtime) Load(
	npcTemplates []types.NpcTemplateRow,
	npcSpawns []types.NpcSpawnRow,
	monsterTemplates []types.MonsterTemplateRow,
	monsterSpawns []types.MonsterSpawnRow,
) {
	npcByEntry := make(map[uint16]types.NpcTemplateRow, len(npcTemplates))
	for _, template := range npcTemplates {
		template = types.NormalizeNpcTemplate(template)
		npcByEntry[template.EntryID] = template
	}
	monsterByEntry := make(map[uint16]types.MonsterTemplateRow, len(monsterTemplates))
	for _, template := range monsterTemplates {
		monsterByEntry[template.EntryID] = normalizeMonsterTemplate(template)
	}

	npcs := make([]*NPC, 0, len(npcSpawns))
	for _, spawn := range npcSpawns {
		if !spawn.IsEnabled || spawn.EntryID == IgnoredNpcEntryID || !channelMatches(spawn.Channel, rt.channel) {
			continue
		}
		template, ok := npcByEntry[spawn.EntryID]
		if !ok {
			continue
		}
		npcs = append(npcs, &NPC{
			SessionID:       rt.npcSessionID(spawn.SpawnID),
			WorldEntityID:   atomic.AddInt32(&rt.nextWEID, 1),
			SpawnID:         spawn.SpawnID,
			EntryID:         spawn.EntryID,
			Name:            template.Name,
			Kind:            template.Kind,
			ClassGroup:      template.ClassGroup,
			IsTrainer:       template.IsTrainer,
			InteractionKind: template.InteractionKind,
			MapID:           spawn.MapID,
			LocalX:          spawn.LocalX,
			LocalY:          spawn.LocalY,
			Channel:         spawn.Channel,
		})
	}

	monsters := make([]*Monster, 0, len(monsterSpawns))
	for _, spawn := range monsterSpawns {
		if !spawn.IsEnabled || !channelMatches(spawn.Channel, rt.channel) {
			continue
		}
		template, ok := monsterByEntry[spawn.EntryID]
		if !ok {
			continue
		}
		respawnSeconds := spawn.RespawnSeconds
		if respawnSeconds <= 0 {
			respawnSeconds = 30
		}
		monsters = append(monsters, &Monster{
			SessionID:      rt.monsterSessionID(),
			WorldEntityID:  atomic.AddInt32(&rt.nextWEID, 1),
			SpawnID:        spawn.SpawnID,
			EntryID:        spawn.EntryID,
			Name:           template.Name,
			Level:          template.Level,
			MapID:          spawn.MapID,
			LocalX:         spawn.LocalX,
			LocalY:         spawn.LocalY,
			Health:         template.MaxHealth,
			MaxHealth:      template.MaxHealth,
			MoveMS:         template.MoveMS,
			WalkSpeed:      template.WalkSpeed,
			RunSpeed:       template.RunSpeed,
			RespawnSeconds: respawnSeconds,
			MinDamage:      template.MinDamage,
			MaxDamage:      template.MaxDamage,
			BaseAttackMS:   template.BaseAttackMS,
			AggroRange:     spawn.AggroRange,
			LeashRange:     spawn.LeashRange,
			SpawnDistance:  spawn.SpawnDistance,
			MovementType:   spawn.MovementType,
			AI:             spawn.AI,
			Channel:        spawn.Channel,
		})
	}

	rt.mu.Lock()
	rt.npcs = npcs
	rt.monsters = monsters
	rt.mu.Unlock()
}

func (rt *Runtime) NpcCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.npcs)
}

func (rt *Runtime) MonsterCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.monsters)
}

func (rt *Runtime) Count() int {
	return rt.NpcCount()
}

func (rt *Runtime) PlayerCount() int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return len(rt.players)
}

func (rt *Runtime) SyncPlayer(player Player) bool {
	if int(player.Channel) != int(rt.channel) {
		return false
	}
	if player.AccountID == 0 {
		return false
	}
	player.UpdatedAt = time.Now()
	rt.mu.Lock()
	rt.players[player.AccountID] = player
	rt.mu.Unlock()
	return true
}

func (rt *Runtime) LeavePlayer(accountID uint32, sessionID int16) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if accountID != 0 {
		if _, ok := rt.players[accountID]; ok {
			delete(rt.players, accountID)
			return true
		}
	}
	for key, player := range rt.players {
		if sessionID != 0 && player.SessionID == sessionID {
			delete(rt.players, key)
			return true
		}
	}
	return false
}

func (rt *Runtime) VisibleAt(mapID uint16, x int16, y int16, channel int16, force bool) Snapshot {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	visibleNpcs := make([]*NPC, 0, 16)
	for _, npc := range rt.npcs {
		if npc.MapID != mapID || !channelMatches(npc.Channel, byte(channel)) {
			continue
		}
		if distance2D(float64(npc.LocalX), float64(npc.LocalY), float64(x), float64(y)) < DefaultVisibilityRange*DefaultVisibilityRange {
			visibleNpcs = append(visibleNpcs, npc)
		}
	}

	visibleMonsters := make([]*Monster, 0, 16)
	for _, monster := range rt.monsters {
		if monster.MapID != mapID || !channelMatches(monster.Channel, byte(channel)) {
			continue
		}
		if distance2D(float64(monster.LocalX), float64(monster.LocalY), float64(x), float64(y)) < DefaultMonsterRange*DefaultMonsterRange {
			visibleMonsters = append(visibleMonsters, monster)
		}
	}

	return Snapshot{
		MapID:        mapID,
		Channel:      channel,
		NpcCount:     len(visibleNpcs),
		MonsterCount: len(visibleMonsters),
		Npcs:         visibleNpcs,
		Monsters:     visibleMonsters,
	}
}

func normalizeMonsterTemplate(template types.MonsterTemplateRow) types.MonsterTemplateRow {
	if template.Level == 0 {
		template.Level = 1
	}
	if template.MaxHealth <= 0 {
		template.MaxHealth = 100
	}
	if template.MoveMS <= 0 {
		template.MoveMS = 150
	}
	if template.MoveMS < 650 {
		template.MoveMS = 650
	}
	if template.WalkSpeed <= 0 {
		template.WalkSpeed = 1
	}
	if template.RunSpeed <= 0 {
		template.RunSpeed = 3.5
	}
	if template.MinDamage <= 0 {
		template.MinDamage = 5
	}
	if template.MaxDamage < template.MinDamage {
		template.MaxDamage = template.MinDamage
	}
	if template.BaseAttackMS <= 0 {
		template.BaseAttackMS = 2000
	}
	return template
}

func (rt *Runtime) monsterSessionID() int16 {
	for {
		id := atomic.AddInt32(&rt.nextMon, 1)
		if id > 0 && id < int32(MaxPositiveSessionID) {
			return int16(id)
		}
		atomic.CompareAndSwapInt32(&rt.nextMon, id, 20000)
	}
}

func (rt *Runtime) npcSessionID(spawnID uint32) int16 {
	if spawnID > 0 && spawnID < MaxPositiveSessionID {
		return int16(spawnID)
	}
	id := int16(atomic.AddInt32(&rt.nextNPC, 1) & 0x7FFF)
	if id == 0 {
		return rt.npcSessionID(0)
	}
	return id
}

func channelMatches(rowChannel int16, channel byte) bool {
	return rowChannel < 0 || rowChannel == int16(channel)
}

func distance2D(ax, ay, bx, by float64) float64 {
	dx := ax - bx
	dy := ay - by
	return dx*dx + dy*dy
}
