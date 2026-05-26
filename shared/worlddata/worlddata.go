package worlddata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"asda2/shared/types"
)

const (
	EnvWorldDataDir = "ASDA2_WORLD_DATA_DIR"

	defaultMonsterRespawnSeconds = 30
	defaultWorldDataDir          = "data/asda2"
)

type monsterTemplateFileRow struct {
	ID           uint16  `json:"id"`
	Name         string  `json:"name"`
	Level        byte    `json:"level"`
	MaxHealth    int32   `json:"maxHealth"`
	MoveMS       int16   `json:"moveMs"`
	WalkSpeed    float64 `json:"walkSpeed,omitempty"`
	RunSpeed     float64 `json:"runSpeed,omitempty"`
	MinDamage    float64 `json:"minDamage,omitempty"`
	MaxDamage    float64 `json:"maxDamage,omitempty"`
	BaseAttackMS int     `json:"baseAttackMs,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

type monsterMapFile struct {
	MapID  *uint16             `json:"mapId,omitempty"`
	Maps   []monsterMapFile    `json:"maps,omitempty"`
	Groups []monsterSpawnGroup `json:"groups,omitempty"`
}

type monsterSpawnGroup struct {
	ID             string              `json:"id,omitempty"`
	MonsterID      uint16              `json:"monsterId,omitempty"`
	EntryID        uint16              `json:"entryId,omitempty"`
	RespawnSeconds int                 `json:"respawnSeconds,omitempty"`
	Channel        *int16              `json:"channel,omitempty"`
	AI             string              `json:"ai,omitempty"`
	AggroRange     float64             `json:"aggroRange,omitempty"`
	LeashRange     float64             `json:"leashRange,omitempty"`
	LootTable      string              `json:"lootTable,omitempty"`
	SpawnDistance  float64             `json:"spawnDistance,omitempty"`
	MovementType   int                 `json:"movementType,omitempty"`
	SpawnPoints    []monsterSpawnPoint `json:"spawnPoints"`
}

type monsterSpawnPoint struct {
	SpawnID         uint32  `json:"spawnId,omitempty"`
	GUID            uint32  `json:"guid,omitempty"`
	X               int16   `json:"x"`
	Y               int16   `json:"y"`
	Z               float64 `json:"z,omitempty"`
	Orientation     float64 `json:"orientation,omitempty"`
	SpawnDistance   float64 `json:"spawnDistance,omitempty"`
	MovementType    int     `json:"movementType,omitempty"`
	CurrentWaypoint uint32  `json:"currentWaypoint,omitempty"`
	MonsterID       uint16  `json:"monsterId,omitempty"`
	EntryID         uint16  `json:"entryId,omitempty"`
	RespawnSeconds  int     `json:"respawnSeconds,omitempty"`
	Channel         *int16  `json:"channel,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
}

type monsterDropFileTable struct {
	MonsterID uint16                `json:"monsterId"`
	Drops     []monsterDropFileItem `json:"drops"`
}

type monsterDropFileItem struct {
	ItemID    int32   `json:"itemId"`
	Chance    float64 `json:"chance"`
	MinAmount int32   `json:"minAmount,omitempty"`
	MaxAmount int32   `json:"maxAmount,omitempty"`
}

type npcTemplateFileRow struct {
	ID              uint16                   `json:"id"`
	Name            string                   `json:"name"`
	Kind            byte                     `json:"kind,omitempty"`
	ClassGroup      byte                     `json:"classGroup,omitempty"`
	IsTrainer       bool                     `json:"isTrainer,omitempty"`
	Interaction     string                   `json:"interaction,omitempty"`
	InteractionKind types.NpcInteractionKind `json:"interactionKind,omitempty"`
	Enabled         *bool                    `json:"enabled,omitempty"`
}

type npcMapFile struct {
	MapID  *uint16         `json:"mapId,omitempty"`
	Maps   []npcMapFile    `json:"maps,omitempty"`
	Groups []npcSpawnGroup `json:"groups,omitempty"`
}

type npcSpawnGroup struct {
	ID          string          `json:"id,omitempty"`
	NpcID       uint16          `json:"npcId,omitempty"`
	EntryID     uint16          `json:"entryId,omitempty"`
	Channel     *int16          `json:"channel,omitempty"`
	SpawnPoints []npcSpawnPoint `json:"spawnPoints"`
}

type npcSpawnPoint struct {
	SpawnID uint32 `json:"spawnId,omitempty"`
	GUID    uint32 `json:"guid,omitempty"`
	X       int16  `json:"x"`
	Y       int16  `json:"y"`
	NpcID   uint16 `json:"npcId,omitempty"`
	EntryID uint16 `json:"entryId,omitempty"`
	Channel *int16 `json:"channel,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

type npcVendorFileTable struct {
	VendorID      uint16              `json:"vendorId,omitempty"`
	VendorEntryID uint16              `json:"vendorEntryId,omitempty"`
	NpcID         uint16              `json:"npcId,omitempty"`
	ItemID        int                 `json:"itemId,omitempty"`
	SortOrder     int                 `json:"sortOrder,omitempty"`
	Enabled       *bool               `json:"enabled,omitempty"`
	Items         []npcVendorFileItem `json:"items,omitempty"`
}

type npcVendorFileItem struct {
	ItemID    int   `json:"itemId"`
	SortOrder int   `json:"sortOrder,omitempty"`
	Enabled   *bool `json:"enabled,omitempty"`
}

func LoadMonsters(root string, channel byte) ([]types.MonsterTemplateRow, []types.MonsterSpawnRow, string, bool, error) {
	resolved, ok := ResolveRoot(root)
	if !ok {
		return nil, nil, "", false, nil
	}

	monsterRoot := filepath.Join(resolved, "monsters")
	templatePath := filepath.Join(monsterRoot, "templates.json")
	mapDir := filepath.Join(monsterRoot, "maps")
	if !fileExists(templatePath) || !dirExists(mapDir) {
		return nil, nil, "", false, nil
	}

	templates, err := loadMonsterTemplates(templatePath)
	if err != nil {
		return nil, nil, "", true, err
	}
	spawns, err := loadMonsterSpawns(mapDir, channel)
	if err != nil {
		return nil, nil, "", true, err
	}
	return templates, spawns, "static:" + resolved, true, nil
}

func LoadMonsterDrops(root string) ([]types.MonsterDropRow, string, bool, error) {
	resolved, ok := ResolveRoot(root)
	if !ok {
		return nil, "", false, nil
	}

	path := filepath.Join(resolved, "monsters", "drops.json")
	if !fileExists(path) {
		return nil, "", false, nil
	}

	drops, err := loadMonsterDrops(path)
	if err != nil {
		return nil, "", true, err
	}
	return drops, "static:" + resolved, true, nil
}

func LoadNpcs(root string, channel byte) ([]types.NpcTemplateRow, []types.NpcSpawnRow, string, bool, error) {
	resolved, ok := ResolveRoot(root)
	if !ok {
		return nil, nil, "", false, nil
	}

	npcRoot := filepath.Join(resolved, "npcs")
	templatePath := filepath.Join(npcRoot, "templates.json")
	mapDir := filepath.Join(npcRoot, "maps")
	if !fileExists(templatePath) || !dirExists(mapDir) {
		return nil, nil, "", false, nil
	}

	templates, err := loadNpcTemplates(templatePath)
	if err != nil {
		return nil, nil, "", true, err
	}
	spawns, err := loadNpcSpawns(mapDir, channel)
	if err != nil {
		return nil, nil, "", true, err
	}
	return templates, spawns, "static:" + resolved, true, nil
}

func LoadNpcVendorItems(root string) ([]types.NpcVendorItemRow, string, bool, error) {
	resolved, ok := ResolveRoot(root)
	if !ok {
		return nil, "", false, nil
	}

	path := filepath.Join(resolved, "npcs", "vendors.json")
	if !fileExists(path) {
		return nil, "", false, nil
	}

	rows, err := loadNpcVendorItems(path)
	if err != nil {
		return nil, "", true, err
	}
	return rows, "static:" + resolved, true, nil
}

func ResolveRoot(explicit string) (string, bool) {
	candidates := make([]string, 0, 5)
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, strings.TrimSpace(explicit))
	}
	if value := strings.TrimSpace(os.Getenv(EnvWorldDataDir)); value != "" {
		candidates = append(candidates, value)
	}
	candidates = append(candidates, defaultWorldDataDir)

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, defaultWorldDataDir),
			filepath.Join(exeDir, "..", defaultWorldDataDir),
		)
	}

	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if dirExists(abs) {
			return abs, true
		}
	}
	return "", false
}

func loadMonsterTemplates(path string) ([]types.MonsterTemplateRow, error) {
	var rows []monsterTemplateFileRow
	if err := readJSON(path, &rows); err != nil {
		return nil, fmt.Errorf("load monster templates %s: %w", path, err)
	}

	out := make([]types.MonsterTemplateRow, 0, len(rows))
	for _, row := range rows {
		if !enabled(row.Enabled) || row.ID == 0 {
			continue
		}
		out = append(out, types.MonsterTemplateRow{
			EntryID:      row.ID,
			Name:         strings.TrimSpace(row.Name),
			Level:        row.Level,
			MaxHealth:    row.MaxHealth,
			MoveMS:       row.MoveMS,
			WalkSpeed:    row.WalkSpeed,
			RunSpeed:     row.RunSpeed,
			MinDamage:    row.MinDamage,
			MaxDamage:    row.MaxDamage,
			BaseAttackMS: row.BaseAttackMS,
		})
	}
	return out, nil
}

func loadMonsterSpawns(dir string, channel byte) ([]types.MonsterSpawnRow, error) {
	files, err := jsonFiles(dir)
	if err != nil {
		return nil, err
	}

	var out []types.MonsterSpawnRow
	spawnID := uint32(1)
	for _, path := range files {
		var file monsterMapFile
		if err := readJSON(path, &file); err != nil {
			return nil, fmt.Errorf("load monster map %s: %w", path, err)
		}
		for _, mapFile := range flattenMonsterMaps(file) {
			if mapFile.MapID == nil {
				return nil, fmt.Errorf("monster map file %s is missing mapId", path)
			}
			for _, group := range mapFile.Groups {
				groupChannel := int16(-1)
				if group.Channel != nil {
					groupChannel = *group.Channel
				}
				groupRespawn := group.RespawnSeconds
				if groupRespawn <= 0 {
					groupRespawn = defaultMonsterRespawnSeconds
				}
				groupSpawnDistance := group.SpawnDistance
				groupMovementType := group.MovementType
				groupEntryID := firstUint16(group.MonsterID, group.EntryID)
				for _, point := range group.SpawnPoints {
					if !enabled(point.Enabled) {
						continue
					}
					rowChannel := groupChannel
					if point.Channel != nil {
						rowChannel = *point.Channel
					}
					if !channelMatches(rowChannel, channel) {
						continue
					}
					respawn := groupRespawn
					if point.RespawnSeconds > 0 {
						respawn = point.RespawnSeconds
					}
					spawnDistance := groupSpawnDistance
					if point.SpawnDistance > 0 {
						spawnDistance = point.SpawnDistance
					}
					movementType := groupMovementType
					if point.MovementType > 0 {
						movementType = point.MovementType
					}
					entryID := firstUint16(point.MonsterID, point.EntryID, groupEntryID)
					if entryID == 0 {
						return nil, fmt.Errorf("monster group %q in %s has spawn without monsterId", group.ID, path)
					}
					rowSpawnID := firstUint32(point.SpawnID, point.GUID)
					if rowSpawnID == 0 {
						rowSpawnID = spawnID
					}
					localX := monsterLocalCoord(point.X, *mapFile.MapID)
					localY := monsterLocalCoord(point.Y, *mapFile.MapID)
					out = append(out, types.MonsterSpawnRow{
						SpawnID:        rowSpawnID,
						EntryID:        entryID,
						MapID:          *mapFile.MapID,
						LocalX:         localX,
						LocalY:         localY,
						RespawnSeconds: respawn,
						Channel:        rowChannel,
						AI:             strings.TrimSpace(group.AI),
						AggroRange:     group.AggroRange,
						LeashRange:     group.LeashRange,
						SpawnDistance:  spawnDistance,
						MovementType:   movementType,
						IsEnabled:      true,
					})
					if rowSpawnID >= spawnID {
						spawnID = rowSpawnID + 1
					} else {
						spawnID++
					}
				}
			}
		}
	}
	return out, nil
}

func monsterLocalCoord(value int16, mapID uint16) int16 {
	offset := int32(mapID) * 1000
	v := int32(value)
	if offset <= 0 || v < offset || v >= offset+1000 {
		return value
	}
	return int16(v - offset)
}

func loadMonsterDrops(path string) ([]types.MonsterDropRow, error) {
	var tables []monsterDropFileTable
	if err := readJSON(path, &tables); err != nil {
		return nil, fmt.Errorf("load monster drops %s: %w", path, err)
	}

	var out []types.MonsterDropRow
	for _, table := range tables {
		if table.MonsterID == 0 {
			continue
		}
		for _, drop := range table.Drops {
			if drop.ItemID <= 0 || drop.Chance <= 0 {
				continue
			}
			minAmount := drop.MinAmount
			if minAmount <= 0 {
				minAmount = 1
			}
			maxAmount := drop.MaxAmount
			if maxAmount < minAmount {
				maxAmount = minAmount
			}
			out = append(out, types.MonsterDropRow{
				EntryID:   table.MonsterID,
				ItemID:    drop.ItemID,
				Chance:    drop.Chance,
				MinAmount: minAmount,
				MaxAmount: maxAmount,
			})
		}
	}
	return out, nil
}

func loadNpcTemplates(path string) ([]types.NpcTemplateRow, error) {
	var rows []npcTemplateFileRow
	if err := readJSON(path, &rows); err != nil {
		return nil, fmt.Errorf("load npc templates %s: %w", path, err)
	}

	out := make([]types.NpcTemplateRow, 0, len(rows))
	for _, row := range rows {
		if !enabled(row.Enabled) || row.ID == 0 {
			continue
		}
		interactionKind := row.InteractionKind
		if parsed := types.ParseNpcInteractionKind(row.Interaction); parsed != types.NpcInteractionNone {
			interactionKind = parsed
		}
		out = append(out, types.NormalizeNpcTemplate(types.NpcTemplateRow{
			EntryID:         row.ID,
			Name:            strings.TrimSpace(row.Name),
			Kind:            row.Kind,
			ClassGroup:      row.ClassGroup,
			IsTrainer:       row.IsTrainer,
			InteractionKind: interactionKind,
		}))
	}
	return out, nil
}

func loadNpcVendorItems(path string) ([]types.NpcVendorItemRow, error) {
	var tables []npcVendorFileTable
	if err := readJSON(path, &tables); err != nil {
		return nil, fmt.Errorf("load npc vendors %s: %w", path, err)
	}

	var out []types.NpcVendorItemRow
	for _, table := range tables {
		if !enabled(table.Enabled) {
			continue
		}
		vendorID := firstUint16(table.VendorID, table.VendorEntryID, table.NpcID)
		if table.ItemID > 0 {
			appendNpcVendorItem(&out, vendorID, table.ItemID, table.SortOrder)
		}
		for i, item := range table.Items {
			if !enabled(item.Enabled) {
				continue
			}
			sortOrder := item.SortOrder
			if sortOrder <= 0 {
				sortOrder = table.SortOrder
			}
			if sortOrder <= 0 {
				sortOrder = i + 1
			}
			appendNpcVendorItem(&out, vendorID, item.ItemID, sortOrder)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].VendorEntryID != out[j].VendorEntryID {
			return out[i].VendorEntryID < out[j].VendorEntryID
		}
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ItemID < out[j].ItemID
	})
	return out, nil
}

func appendNpcVendorItem(out *[]types.NpcVendorItemRow, vendorID uint16, itemID int, sortOrder int) {
	if itemID <= 0 {
		return
	}
	*out = append(*out, types.NpcVendorItemRow{
		VendorEntryID: vendorID,
		ItemID:        itemID,
		SortOrder:     sortOrder,
		IsEnabled:     true,
	})
}

func loadNpcSpawns(dir string, channel byte) ([]types.NpcSpawnRow, error) {
	files, err := jsonFiles(dir)
	if err != nil {
		return nil, err
	}

	var out []types.NpcSpawnRow
	spawnID := uint32(1)
	for _, path := range files {
		var file npcMapFile
		if err := readJSON(path, &file); err != nil {
			return nil, fmt.Errorf("load npc map %s: %w", path, err)
		}
		for _, mapFile := range flattenNpcMaps(file) {
			if mapFile.MapID == nil {
				return nil, fmt.Errorf("npc map file %s is missing mapId", path)
			}
			for _, group := range mapFile.Groups {
				groupChannel := int16(-1)
				if group.Channel != nil {
					groupChannel = *group.Channel
				}
				groupEntryID := firstUint16(group.NpcID, group.EntryID)
				for _, point := range group.SpawnPoints {
					if !enabled(point.Enabled) {
						continue
					}
					rowChannel := groupChannel
					if point.Channel != nil {
						rowChannel = *point.Channel
					}
					if !channelMatches(rowChannel, channel) {
						continue
					}
					entryID := firstUint16(point.NpcID, point.EntryID, groupEntryID)
					if entryID == 0 {
						return nil, fmt.Errorf("npc group %q in %s has spawn without npcId", group.ID, path)
					}
					rowSpawnID := firstUint32(point.SpawnID, point.GUID)
					if rowSpawnID == 0 {
						rowSpawnID = spawnID
					}
					out = append(out, types.NpcSpawnRow{
						SpawnID:   rowSpawnID,
						EntryID:   entryID,
						MapID:     *mapFile.MapID,
						LocalX:    localMapCoord(point.X, *mapFile.MapID),
						LocalY:    localMapCoord(point.Y, *mapFile.MapID),
						Channel:   rowChannel,
						IsEnabled: true,
					})
					if rowSpawnID >= spawnID {
						spawnID = rowSpawnID + 1
					} else {
						spawnID++
					}
				}
			}
		}
	}
	return out, nil
}

func localMapCoord(value int16, mapID uint16) int16 {
	offset := int32(mapID) * 1000
	if offset == 0 {
		return value
	}
	v := int32(value)
	if v >= offset {
		v -= offset
	}
	return int16(v)
}

func flattenMonsterMaps(file monsterMapFile) []monsterMapFile {
	if len(file.Maps) > 0 {
		return file.Maps
	}
	if file.MapID == nil && len(file.Groups) == 0 {
		return nil
	}
	return []monsterMapFile{file}
}

func flattenNpcMaps(file npcMapFile) []npcMapFile {
	if len(file.Maps) > 0 {
		return file.Maps
	}
	if file.MapID == nil && len(file.Groups) == 0 {
		return nil
	}
	return []npcMapFile{file}
}

func jsonFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func readJSON(path string, out interface{}) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(bytes))) == 0 {
		return errors.New("empty json file")
	}
	return json.Unmarshal(bytes, out)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func enabled(value *bool) bool {
	return value == nil || *value
}

func channelMatches(rowChannel int16, channel byte) bool {
	return rowChannel == -1 || rowChannel == int16(channel)
}

func firstUint16(values ...uint16) uint16 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstUint32(values ...uint32) uint32 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
