package types

// MonsterTemplateRow is the Asda2-only subset needed by the current monster
// runtime. More combat/loot/skill fields should be added only when a system
// actually consumes them.
type MonsterTemplateRow struct {
	EntryID      uint16
	Name         string
	Level        byte
	MaxHealth    int32
	MoveMS       int16
	WalkSpeed    float64
	RunSpeed     float64
	MinDamage    float64
	MaxDamage    float64
	BaseAttackMS int
}

// MonsterSpawnRow describes one persistent monster spawn point.
type MonsterSpawnRow struct {
	SpawnID        uint32
	EntryID        uint16
	MapID          uint16
	LocalX         int16
	LocalY         int16
	RespawnSeconds int
	Channel        int16 // -1 means all channels
	AI             string
	AggroRange     float64
	LeashRange     float64
	SpawnDistance  float64
	MovementType   int
	IsEnabled      bool
}

// MonsterDropRow describes one static drop chance loaded from worlddata.
type MonsterDropRow struct {
	EntryID   uint16
	ItemID    int32
	Chance    float64
	MinAmount int32
	MaxAmount int32
}
