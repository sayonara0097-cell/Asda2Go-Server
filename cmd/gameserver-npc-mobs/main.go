package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sharedDB "asda2/shared/db"
	"asda2/shared/types"
)

const defaultMonsterMoveMS = 150

type exportTemplate struct {
	ID               uint16   `json:"id"`
	Name             string   `json:"name"`
	Level            byte     `json:"level"`
	MaxHealth        int32    `json:"maxHealth"`
	MoveMS           int16    `json:"moveMs"`
	WalkSpeed        float64  `json:"walkSpeed,omitempty"`
	RunSpeed         float64  `json:"runSpeed,omitempty"`
	MoneyDrop        int64    `json:"moneyDrop,omitempty"`
	PhysicalResist   int64    `json:"physicalResist,omitempty"`
	MagicalResist    int64    `json:"magicalResist,omitempty"`
	MinDamage        float64  `json:"minDamage,omitempty"`
	MaxDamage        float64  `json:"maxDamage,omitempty"`
	BaseAttackMS     int64    `json:"baseAttackMs,omitempty"`
	Rank             int64    `json:"rank,omitempty"`
	Spells           []uint32 `json:"spells,omitempty"`
	Aggressive       bool     `json:"aggressive,omitempty"`
	MovementType     int      `json:"movementType,omitempty"`
	Mana             int64    `json:"mana,omitempty"`
	DamageMultiplier float64  `json:"damageMultiplier,omitempty"`
	Placeholder      bool     `json:"placeholder,omitempty"`
	Enabled          bool     `json:"enabled"`
}

type exportMapFile struct {
	MapID  uint16             `json:"mapId"`
	Groups []exportSpawnGroup `json:"groups"`
}

type exportSpawnGroup struct {
	ID             string             `json:"id"`
	MonsterID      uint16             `json:"monsterId"`
	RespawnSeconds int                `json:"respawnSeconds"`
	Channel        int16              `json:"channel"`
	AI             string             `json:"ai,omitempty"`
	SpawnDistance  float64            `json:"spawnDistance,omitempty"`
	MovementType   int                `json:"movementType,omitempty"`
	SpawnPoints    []exportSpawnPoint `json:"spawnPoints"`
}

type exportSpawnPoint struct {
	SpawnID         uint32  `json:"spawnId"`
	MonsterID       uint16  `json:"monsterId,omitempty"`
	X               int16   `json:"x"`
	Y               int16   `json:"y"`
	Z               float64 `json:"z,omitempty"`
	Orientation     float64 `json:"orientation,omitempty"`
	SpawnDistance   float64 `json:"spawnDistance,omitempty"`
	MovementType    int     `json:"movementType,omitempty"`
	CurrentWaypoint uint32  `json:"currentWaypoint,omitempty"`
	ModelID         uint32  `json:"modelId,omitempty"`
	EquipmentID     uint32  `json:"equipmentId,omitempty"`
	SpawnMask       int     `json:"spawnMask,omitempty"`
	PhaseMask       int     `json:"phaseMask,omitempty"`
	CurrentHealth   int32   `json:"currentHealth,omitempty"`
	CurrentMana     int32   `json:"currentMana,omitempty"`
	DeathState      int     `json:"deathState,omitempty"`
}

type exportDropTable struct {
	MonsterID uint16       `json:"monsterId"`
	Drops     []exportDrop `json:"drops"`
}

type exportDrop struct {
	GUID            uint32  `json:"guid"`
	ItemID          uint32  `json:"itemId"`
	RequiredQuestID uint32  `json:"requiredQuestId,omitempty"`
	Chance          float64 `json:"chance"`
	Type            int     `json:"type,omitempty"`
	MinAmount       int     `json:"minAmount,omitempty"`
	MaxAmount       int     `json:"maxAmount,omitempty"`
	UserGenerated   bool    `json:"userGenerated,omitempty"`
}

type exportMovementPath struct {
	SpawnID uint32                `json:"spawnId"`
	Points  []exportMovementPoint `json:"points"`
}

type exportMovementPoint struct {
	Point       uint32  `json:"point"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           float64 `json:"z,omitempty"`
	WaitMS      uint32  `json:"waitMs,omitempty"`
	Orientation float64 `json:"orientation,omitempty"`
}

type exportNpcTemplate struct {
	ID              uint16                   `json:"id"`
	Name            string                   `json:"name"`
	Kind            byte                     `json:"kind,omitempty"`
	ClassGroup      byte                     `json:"classGroup,omitempty"`
	IsTrainer       bool                     `json:"isTrainer,omitempty"`
	InteractionKind types.NpcInteractionKind `json:"interactionKind,omitempty"`
	Enabled         bool                     `json:"enabled"`
}

type exportNpcMapFile struct {
	MapID  uint16                `json:"mapId"`
	Groups []exportNpcSpawnGroup `json:"groups"`
}

type exportNpcSpawnGroup struct {
	ID          string                `json:"id"`
	NpcID       uint16                `json:"npcId"`
	Channel     int16                 `json:"channel"`
	SpawnPoints []exportNpcSpawnPoint `json:"spawnPoints"`
}

type exportNpcSpawnPoint struct {
	SpawnID      uint32  `json:"spawnId"`
	NpcID        uint16  `json:"npcId,omitempty"`
	X            int16   `json:"x"`
	Y            int16   `json:"y"`
	Z            float64 `json:"z,omitempty"`
	Orientation  float64 `json:"orientation,omitempty"`
	State        int     `json:"state,omitempty"`
	AnimProgress int     `json:"animProgress,omitempty"`
	PhaseMask    int     `json:"phaseMask,omitempty"`
	RespawnSecs  int     `json:"respawnSeconds,omitempty"`
}

type exportNpcVendorTable struct {
	VendorID uint16                `json:"vendorId"`
	Items    []exportNpcVendorItem `json:"items"`
}

type exportNpcVendorItem struct {
	ItemID    int `json:"itemId"`
	SortOrder int `json:"sortOrder,omitempty"`
}

type spawnGroupKey struct {
	MapID          uint16
	MonsterID      uint16
	Channel        int16
	RespawnSeconds int
	MovementType   int
	SpawnDistance  string
}

func main() {
	cfg := sharedDB.DefaultDB
	outRoot := flag.String("out", filepath.Join("data", "asda2"), "output static NPC/mob data root")
	flag.StringVar(&cfg.Host, "host", cfg.Host, "MySQL host")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "MySQL port")
	flag.StringVar(&cfg.Database, "db", cfg.Database, "MySQL database")
	flag.StringVar(&cfg.User, "user", cfg.User, "MySQL user")
	flag.StringVar(&cfg.Password, "password", cfg.Password, "MySQL password")
	flag.Parse()

	log.SetFlags(0)
	if err := sharedDB.Init(cfg); err != nil {
		log.Fatalf("connect db: %v", err)
	}

	templates, metaByID, err := loadTemplates()
	if err != nil {
		log.Fatalf("load templates: %v", err)
	}
	if err := addMissingTemplates(&templates, metaByID); err != nil {
		log.Fatalf("load missing templates: %v", err)
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].ID < templates[j].ID })

	mapFiles, totalSpawns, err := loadSpawns(metaByID)
	if err != nil {
		log.Fatalf("load spawns: %v", err)
	}
	drops, err := loadDrops()
	if err != nil {
		log.Fatalf("load drops: %v", err)
	}
	paths, err := loadMovementPaths()
	if err != nil {
		log.Fatalf("load movement paths: %v", err)
	}
	npcTemplates, err := loadNpcTemplates()
	if err != nil {
		log.Fatalf("load npc templates: %v", err)
	}
	npcMapFiles, totalNpcSpawns, err := loadNpcSpawns()
	if err != nil {
		log.Fatalf("load npc spawns: %v", err)
	}
	npcVendorItems, totalNpcVendorItems, err := loadNpcVendorItems()
	if err != nil {
		log.Fatalf("load npc vendor items: %v", err)
	}

	monsterRoot := filepath.Join(*outRoot, "monsters")
	if err := os.MkdirAll(filepath.Join(monsterRoot, "maps"), 0o755); err != nil {
		log.Fatalf("create output dirs: %v", err)
	}
	if err := clearJSONFiles(filepath.Join(monsterRoot, "maps")); err != nil {
		log.Fatalf("clear map json files: %v", err)
	}
	if err := writeJSON(filepath.Join(monsterRoot, "templates.json"), templates); err != nil {
		log.Fatalf("write templates: %v", err)
	}
	if err := writeMapFiles(filepath.Join(monsterRoot, "maps"), mapFiles); err != nil {
		log.Fatalf("write maps: %v", err)
	}
	if err := writeJSON(filepath.Join(monsterRoot, "drops.json"), drops); err != nil {
		log.Fatalf("write drops: %v", err)
	}
	if err := writeJSON(filepath.Join(monsterRoot, "movement_paths.json"), paths); err != nil {
		log.Fatalf("write movement paths: %v", err)
	}

	npcRoot := filepath.Join(*outRoot, "npcs")
	if err := os.MkdirAll(filepath.Join(npcRoot, "maps"), 0o755); err != nil {
		log.Fatalf("create npc output dirs: %v", err)
	}
	if err := clearJSONFiles(filepath.Join(npcRoot, "maps")); err != nil {
		log.Fatalf("clear npc map json files: %v", err)
	}
	if err := writeJSON(filepath.Join(npcRoot, "templates.json"), npcTemplates); err != nil {
		log.Fatalf("write npc templates: %v", err)
	}
	if err := writeNpcMapFiles(filepath.Join(npcRoot, "maps"), npcMapFiles); err != nil {
		log.Fatalf("write npc maps: %v", err)
	}
	if totalNpcVendorItems > 0 {
		if err := writeJSON(filepath.Join(npcRoot, "vendors.json"), npcVendorItems); err != nil {
			log.Fatalf("write npc vendors: %v", err)
		}
	}

	log.Printf("exported mob templates=%d spawns=%d maps=%d drops=%d movementPaths=%d; npc templates=%d spawns=%d maps=%d vendorItems=%d to %s",
		len(templates), totalSpawns, len(mapFiles), countDrops(drops), len(paths),
		len(npcTemplates), totalNpcSpawns, len(npcMapFiles), totalNpcVendorItems, *outRoot)
}

type templateMeta struct {
	Aggressive   bool
	MovementType int
}

func loadTemplates() ([]exportTemplate, map[uint16]templateMeta, error) {
	rows, err := sharedDB.DB.Query(`
SELECT entry, Name, Level, Health, PhisicalResist, MagicalResist,
       speed_walk, speed_run, MoneyDrop, mindmg, maxdmg, baseattacktime,
       rank, spell1, spell2, spell3, spell4, IsAgressive, MovementType,
       Mana, dmg_multiplier
  FROM creature_template
 ORDER BY entry ASC`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var out []exportTemplate
	meta := make(map[uint16]templateMeta)
	for rows.Next() {
		var (
			entry, level                                             uint64
			health, physical, magical, money, baseAttack, rank, mana int64
			walkSpeed, runSpeed, minDmg, maxDmg, dmgMultiplier       float64
			spell1, spell2, spell3, spell4                           uint64
			aggressive, movementType                                 int
			name                                                     string
		)
		if err := rows.Scan(
			&entry, &name, &level, &health, &physical, &magical,
			&walkSpeed, &runSpeed, &money, &minDmg, &maxDmg, &baseAttack,
			&rank, &spell1, &spell2, &spell3, &spell4, &aggressive, &movementType,
			&mana, &dmgMultiplier,
		); err != nil {
			return nil, nil, err
		}

		id := clampUint16(entry)
		row := exportTemplate{
			ID:               id,
			Name:             strings.TrimSpace(name),
			Level:            byte(clampUint8(level)),
			MaxHealth:        clampInt32(health),
			MoveMS:           defaultMonsterMoveMS,
			WalkSpeed:        walkSpeed,
			RunSpeed:         runSpeed,
			MoneyDrop:        money,
			PhysicalResist:   physical,
			MagicalResist:    magical,
			MinDamage:        round2(minDmg),
			MaxDamage:        round2(maxDmg),
			BaseAttackMS:     baseAttack,
			Rank:             rank,
			Spells:           nonZeroSpells(spell1, spell2, spell3, spell4),
			Aggressive:       aggressive != 0,
			MovementType:     movementType,
			Mana:             mana,
			DamageMultiplier: dmgMultiplier,
			Enabled:          true,
		}
		out = append(out, row)
		meta[id] = templateMeta{Aggressive: row.Aggressive, MovementType: row.MovementType}
	}
	return out, meta, rows.Err()
}

func addMissingTemplates(templates *[]exportTemplate, meta map[uint16]templateMeta) error {
	rows, err := sharedDB.DB.Query(`
SELECT c.id, COALESCE(MAX(c.curhealth), 100), COALESCE(MAX(c.MovementType), 0)
  FROM creature c
  LEFT JOIN creature_template t ON t.entry = c.id
 WHERE t.entry IS NULL
 GROUP BY c.id
 ORDER BY c.id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var entry uint64
		var health int64
		var movementType int
		if err := rows.Scan(&entry, &health, &movementType); err != nil {
			return err
		}
		id := clampUint16(entry)
		*templates = append(*templates, exportTemplate{
			ID:           id,
			Name:         fmt.Sprintf("Creature %d", id),
			Level:        1,
			MaxHealth:    clampInt32(health),
			MoveMS:       defaultMonsterMoveMS,
			MovementType: movementType,
			Placeholder:  true,
			Enabled:      true,
		})
		meta[id] = templateMeta{MovementType: movementType}
	}
	return rows.Err()
}

func loadSpawns(meta map[uint16]templateMeta) (map[uint16]exportMapFile, int, error) {
	rows, err := sharedDB.DB.Query(`
SELECT guid, id, map, spawnMask, phaseMask, modelid, equipment_id,
       position_x, position_y, position_z, orientation,
       spawntimesecs, spawndist, currentwaypoint,
       curhealth, curmana, DeathState, MovementType, channel
  FROM creature
 ORDER BY map ASC, channel ASC, id ASC, guid ASC`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	files := make(map[uint16]exportMapFile)
	groupByKey := make(map[spawnGroupKey]*exportSpawnGroup)
	total := 0

	for rows.Next() {
		var (
			guid, entry, mapID, modelID, equipmentID, currentWaypoint uint64
			spawnMask, phaseMask, deathState, movementType            int
			x, y, z, orientation, spawnDistance                       float64
			respawn                                                   int
			currentHealth, currentMana                                int64
			channel                                                   int16
		)
		if err := rows.Scan(
			&guid, &entry, &mapID, &spawnMask, &phaseMask, &modelID, &equipmentID,
			&x, &y, &z, &orientation,
			&respawn, &spawnDistance, &currentWaypoint,
			&currentHealth, &currentMana, &deathState, &movementType, &channel,
		); err != nil {
			return nil, 0, err
		}

		mID := clampUint16(mapID)
		monsterID := clampUint16(entry)
		if respawn <= 0 {
			respawn = 30
		}
		key := spawnGroupKey{
			MapID:          mID,
			MonsterID:      monsterID,
			Channel:        channel,
			RespawnSeconds: respawn,
			MovementType:   movementType,
			SpawnDistance:  normalizedDistance(spawnDistance),
		}
		group := groupByKey[key]
		if group == nil {
			file := files[mID]
			file.MapID = mID
			files[mID] = file

			group = &exportSpawnGroup{
				ID:             groupID(key),
				MonsterID:      monsterID,
				RespawnSeconds: respawn,
				Channel:        channel,
				AI:             aiFor(meta[monsterID], movementType),
				SpawnDistance:  round2(spawnDistance),
				MovementType:   movementType,
			}
			groupByKey[key] = group
		}

		group.SpawnPoints = append(group.SpawnPoints, exportSpawnPoint{
			SpawnID:         clampUint32(guid),
			MonsterID:       monsterID,
			X:               roundInt16(x),
			Y:               roundInt16(y),
			Z:               round2(z),
			Orientation:     round4(orientation),
			SpawnDistance:   round2(spawnDistance),
			MovementType:    movementType,
			CurrentWaypoint: clampUint32(currentWaypoint),
			ModelID:         clampUint32(modelID),
			EquipmentID:     clampUint32(equipmentID),
			SpawnMask:       spawnMask,
			PhaseMask:       phaseMask,
			CurrentHealth:   clampInt32(currentHealth),
			CurrentMana:     clampInt32(currentMana),
			DeathState:      deathState,
		})
		total++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	for key, group := range groupByKey {
		sort.Slice(group.SpawnPoints, func(i, j int) bool {
			return group.SpawnPoints[i].SpawnID < group.SpawnPoints[j].SpawnID
		})
		file := files[key.MapID]
		file.Groups = append(file.Groups, *group)
		files[key.MapID] = file
	}
	for mapID, file := range files {
		sort.Slice(file.Groups, func(i, j int) bool {
			return file.Groups[i].ID < file.Groups[j].ID
		})
		files[mapID] = file
	}
	return files, total, nil
}

func loadDrops() ([]exportDropTable, error) {
	rows, err := sharedDB.DB.Query(`
SELECT Guid, MonstrId, ItemId, RequiredQuestId, Chance, Type, MinAmount, MaxAmount, IsUserGenerated
  FROM asda2itemdroptemplate
 ORDER BY MonstrId ASC, Guid ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byMonster := make(map[uint16][]exportDrop)
	for rows.Next() {
		var guid, monsterID, itemID uint64
		var requiredQuestID int64
		var chance float64
		var typ, minAmount, maxAmount, userGenerated int
		if err := rows.Scan(&guid, &monsterID, &itemID, &requiredQuestID, &chance, &typ, &minAmount, &maxAmount, &userGenerated); err != nil {
			return nil, err
		}
		id := clampUint16(monsterID)
		byMonster[id] = append(byMonster[id], exportDrop{
			GUID:            clampUint32(guid),
			ItemID:          clampUint32(itemID),
			RequiredQuestID: clampPositiveUint32(requiredQuestID),
			Chance:          chance,
			Type:            typ,
			MinAmount:       minAmount,
			MaxAmount:       maxAmount,
			UserGenerated:   userGenerated != 0,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	keys := make([]int, 0, len(byMonster))
	for id := range byMonster {
		keys = append(keys, int(id))
	}
	sort.Ints(keys)
	out := make([]exportDropTable, 0, len(keys))
	for _, id := range keys {
		out = append(out, exportDropTable{
			MonsterID: uint16(id),
			Drops:     byMonster[uint16(id)],
		})
	}
	return out, nil
}

func loadMovementPaths() ([]exportMovementPath, error) {
	rows, err := sharedDB.DB.Query(`
SELECT id, point, position_x, position_y, position_z, waittime, orientation
  FROM creature_movement
 ORDER BY id ASC, point ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySpawn := make(map[uint32][]exportMovementPoint)
	for rows.Next() {
		var id, point, wait uint64
		var x, y, z, orientation float64
		if err := rows.Scan(&id, &point, &x, &y, &z, &wait, &orientation); err != nil {
			return nil, err
		}
		spawnID := clampUint32(id)
		bySpawn[spawnID] = append(bySpawn[spawnID], exportMovementPoint{
			Point:       clampUint32(point),
			X:           round2(x),
			Y:           round2(y),
			Z:           round2(z),
			WaitMS:      clampUint32(wait),
			Orientation: round4(orientation),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	keys := make([]int, 0, len(bySpawn))
	for id := range bySpawn {
		keys = append(keys, int(id))
	}
	sort.Ints(keys)
	out := make([]exportMovementPath, 0, len(keys))
	for _, id := range keys {
		points := bySpawn[uint32(id)]
		sort.Slice(points, func(i, j int) bool { return points[i].Point < points[j].Point })
		out = append(out, exportMovementPath{
			SpawnID: uint32(id),
			Points:  points,
		})
	}
	return out, nil
}

func loadNpcTemplates() ([]exportNpcTemplate, error) {
	rows, err := sharedDB.DB.Query(`
SELECT entry, name, type
  FROM gameobject_template
 ORDER BY entry ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []exportNpcTemplate
	for rows.Next() {
		var entry uint64
		var name string
		var kind int
		if err := rows.Scan(&entry, &name, &kind); err != nil {
			return nil, err
		}
		template := types.NormalizeNpcTemplate(types.NpcTemplateRow{
			EntryID: clampUint16(entry),
			Name:    strings.TrimSpace(name),
			Kind:    byte(clampUint8(uint64(kind))),
		})
		out = append(out, exportNpcTemplate{
			ID:              template.EntryID,
			Name:            template.Name,
			Kind:            template.Kind,
			ClassGroup:      template.ClassGroup,
			IsTrainer:       template.IsTrainer,
			InteractionKind: template.InteractionKind,
			Enabled:         true,
		})
	}
	return out, rows.Err()
}

func loadNpcSpawns() (map[uint16]exportNpcMapFile, int, error) {
	rows, err := sharedDB.DB.Query(`
SELECT guid, id, map, position_x, position_y, position_z, orientation,
       phaseMask, state, animprogress, spawntimesecs
  FROM gameobject
 ORDER BY map ASC, id ASC, guid ASC`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	files := make(map[uint16]exportNpcMapFile)
	groupByKey := make(map[npcSpawnGroupKey]*exportNpcSpawnGroup)
	total := 0

	for rows.Next() {
		var guid, entry uint64
		var mapID int64
		var x, y, z, orientation float64
		var phaseMask, state, animProgress, respawn int
		if err := rows.Scan(
			&guid, &entry, &mapID, &x, &y, &z, &orientation,
			&phaseMask, &state, &animProgress, &respawn,
		); err != nil {
			return nil, 0, err
		}
		if entry == 0 || mapID < 0 {
			continue
		}

		mID := clampUint16(uint64(mapID))
		npcID := clampUint16(entry)
		key := npcSpawnGroupKey{MapID: mID, NpcID: npcID, Channel: -1}
		group := groupByKey[key]
		if group == nil {
			file := files[mID]
			file.MapID = mID
			files[mID] = file

			group = &exportNpcSpawnGroup{
				ID:      npcGroupID(key),
				NpcID:   npcID,
				Channel: -1,
			}
			groupByKey[key] = group
		}

		group.SpawnPoints = append(group.SpawnPoints, exportNpcSpawnPoint{
			SpawnID:      clampUint32(guid),
			NpcID:        npcID,
			X:            roundLocalCoord(x, mID),
			Y:            roundLocalCoord(y, mID),
			Z:            round2(z),
			Orientation:  round4(orientation),
			State:        state,
			AnimProgress: animProgress,
			PhaseMask:    phaseMask,
			RespawnSecs:  respawn,
		})
		total++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	for key, group := range groupByKey {
		sort.Slice(group.SpawnPoints, func(i, j int) bool {
			return group.SpawnPoints[i].SpawnID < group.SpawnPoints[j].SpawnID
		})
		file := files[key.MapID]
		file.Groups = append(file.Groups, *group)
		files[key.MapID] = file
	}
	for mapID, file := range files {
		sort.Slice(file.Groups, func(i, j int) bool {
			return file.Groups[i].ID < file.Groups[j].ID
		})
		files[mapID] = file
	}
	return files, total, nil
}

func loadNpcVendorItems() ([]exportNpcVendorTable, int, error) {
	for _, name := range []string{"Asda2NpcVendorItem", "RegularShopRecord", "regularshoprecord", "regular_shop_record"} {
		table, ok, err := firstExistingTable(name)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			continue
		}
		rows, total, err := loadNpcVendorItemsFromTable(table)
		if err != nil {
			return nil, 0, err
		}
		if total > 0 {
			return rows, total, nil
		}
	}
	return nil, 0, nil
}

func loadNpcVendorItemsFromTable(table string) ([]exportNpcVendorTable, int, error) {
	query := fmt.Sprintf(`
SELECT NpcId, ItemId, Id
  FROM %s
 ORDER BY NpcId ASC, Id ASC, ItemId ASC`, quoteIdent(table))
	if strings.EqualFold(table, "Asda2NpcVendorItem") {
		query = fmt.Sprintf(`
SELECT VendorEntryId, ItemId, SortOrder
  FROM %s
 WHERE IsEnabled = 1
 ORDER BY VendorEntryId ASC, SortOrder ASC, ItemId ASC`, quoteIdent(table))
	}

	rows, err := sharedDB.DB.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	byVendor := make(map[uint16][]exportNpcVendorItem)
	total := 0
	for rows.Next() {
		var vendorID, itemID, sortOrder uint64
		if err := rows.Scan(&vendorID, &itemID, &sortOrder); err != nil {
			return nil, 0, err
		}
		if itemID == 0 {
			continue
		}
		id := clampUint16(vendorID)
		byVendor[id] = append(byVendor[id], exportNpcVendorItem{
			ItemID:    int(clampUint32(itemID)),
			SortOrder: int(clampUint32(sortOrder)),
		})
		total++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	ids := make([]int, 0, len(byVendor))
	for id := range byVendor {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	out := make([]exportNpcVendorTable, 0, len(ids))
	for _, id := range ids {
		items := byVendor[uint16(id)]
		sort.Slice(items, func(i, j int) bool {
			if items[i].SortOrder != items[j].SortOrder {
				return items[i].SortOrder < items[j].SortOrder
			}
			return items[i].ItemID < items[j].ItemID
		})
		out = append(out, exportNpcVendorTable{
			VendorID: uint16(id),
			Items:    items,
		})
	}
	return out, total, nil
}

func firstExistingTable(names ...string) (string, bool, error) {
	for _, name := range names {
		var table string
		err := sharedDB.DB.QueryRow(`
SELECT TABLE_NAME
  FROM information_schema.TABLES
 WHERE TABLE_SCHEMA = DATABASE()
   AND LOWER(TABLE_NAME) = LOWER(?)
 LIMIT 1`, name).Scan(&table)
		if err == nil {
			return table, true, nil
		}
		if err != sql.ErrNoRows {
			return "", false, err
		}
	}
	return "", false, nil
}

func quoteIdent(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

type npcSpawnGroupKey struct {
	MapID   uint16
	NpcID   uint16
	Channel int16
}

func writeMapFiles(dir string, files map[uint16]exportMapFile) error {
	ids := make([]int, 0, len(files))
	for id := range files {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, id := range ids {
		path := filepath.Join(dir, fmt.Sprintf("map_%03d.json", id))
		if err := writeJSON(path, files[uint16(id)]); err != nil {
			return err
		}
	}
	return nil
}

func writeNpcMapFiles(dir string, files map[uint16]exportNpcMapFile) error {
	ids := make([]int, 0, len(files))
	for id := range files {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, id := range ids {
		path := filepath.Join(dir, fmt.Sprintf("map_%03d.json", id))
		if err := writeJSON(path, files[uint16(id)]); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	return os.WriteFile(path, bytes, 0o644)
}

func clearJSONFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func nonZeroSpells(values ...uint64) []uint32 {
	var out []uint32
	for _, value := range values {
		if value != 0 {
			out = append(out, clampUint32(value))
		}
	}
	return out
}

func aiFor(meta templateMeta, movementType int) string {
	if movementType == 0 && meta.MovementType != 0 {
		movementType = meta.MovementType
	}
	if movementType == 0 {
		if meta.Aggressive {
			return "aggressive_stationary"
		}
		return "stationary"
	}
	if meta.Aggressive {
		return "aggressive_roam"
	}
	return "passive_roam"
}

func groupID(key spawnGroupKey) string {
	channel := fmt.Sprintf("%d", key.Channel)
	if key.Channel < 0 {
		channel = "all"
	}
	distance := strings.ReplaceAll(key.SpawnDistance, ".", "_")
	return fmt.Sprintf("map_%03d_monster_%d_ch_%s_respawn_%d_move_%d_dist_%s",
		key.MapID, key.MonsterID, channel, key.RespawnSeconds, key.MovementType, distance)
}

func npcGroupID(key npcSpawnGroupKey) string {
	channel := fmt.Sprintf("%d", key.Channel)
	if key.Channel < 0 {
		channel = "all"
	}
	return fmt.Sprintf("map_%03d_npc_%d_ch_%s", key.MapID, key.NpcID, channel)
}

func normalizedDistance(value float64) string {
	return fmt.Sprintf("%.2f", round2(value))
}

func countDrops(tables []exportDropTable) int {
	total := 0
	for _, table := range tables {
		total += len(table.Drops)
	}
	return total
}

func roundInt16(value float64) int16 {
	rounded := math.Round(value)
	if rounded > math.MaxInt16 {
		return math.MaxInt16
	}
	if rounded < math.MinInt16 {
		return math.MinInt16
	}
	return int16(rounded)
}

func roundLocalCoord(value float64, mapID uint16) int16 {
	offset := float64(mapID) * 1000
	if offset > 0 && value >= offset {
		value -= offset
	}
	return roundInt16(value)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func clampInt32(value int64) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func clampUint8(value uint64) uint64 {
	if value > math.MaxUint8 {
		return math.MaxUint8
	}
	return value
}

func clampUint16(value uint64) uint16 {
	if value > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(value)
}

func clampUint32(value uint64) uint32 {
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value)
}

func clampPositiveUint32(value int64) uint32 {
	if value <= 0 {
		return 0
	}
	if value > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(value)
}
