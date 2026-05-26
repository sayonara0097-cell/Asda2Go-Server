package worlddata

import (
	"os"
	"path/filepath"
	"testing"

	"asda2/shared/types"
)

func TestLoadMonstersFromRepoData(t *testing.T) {
	root := filepath.Join("..", "..", "data", "asda2")
	templates, spawns, source, ok, err := LoadMonsters(root, 0)
	if err != nil {
		t.Fatalf("LoadMonsters() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadMonsters() did not find static data")
	}
	if source == "" {
		t.Fatal("LoadMonsters() source is empty")
	}
	if len(templates) != 860 {
		t.Fatalf("template count=%d, want 860", len(templates))
	}
	if len(spawns) != 9214 {
		t.Fatalf("spawn count=%d, want 9214", len(spawns))
	}
	if templates[0].MinDamage <= 0 || templates[0].MaxDamage <= 0 || templates[0].BaseAttackMS <= 0 {
		t.Fatalf("first template missing combat stats: %#v", templates[0])
	}
	if templates[0].WalkSpeed <= 0 || templates[0].RunSpeed <= 0 {
		t.Fatalf("first template missing movement speeds: %#v", templates[0])
	}

	templatesByID := make(map[uint16]struct{}, len(templates))
	for _, template := range templates {
		templatesByID[template.EntryID] = struct{}{}
	}

	mapCounts := make(map[uint16]int)
	spawnIDs := make(map[uint32]struct{}, len(spawns))
	spawnsWithDistance := 0
	for _, spawn := range spawns {
		if _, ok := templatesByID[spawn.EntryID]; !ok {
			t.Fatalf("spawn %d uses unknown template %d", spawn.SpawnID, spawn.EntryID)
		}
		if spawn.SpawnID == 0 {
			t.Fatalf("spawn has zero id: map=%d entry=%d x=%d y=%d",
				spawn.MapID, spawn.EntryID, spawn.LocalX, spawn.LocalY)
		}
		if _, exists := spawnIDs[spawn.SpawnID]; exists {
			t.Fatalf("duplicate spawn id %d", spawn.SpawnID)
		}
		spawnIDs[spawn.SpawnID] = struct{}{}
		if spawn.LocalX < 0 || spawn.LocalY < 0 {
			t.Fatalf("spawn %d has negative local coordinates map=%d x=%d y=%d",
				spawn.SpawnID, spawn.MapID, spawn.LocalX, spawn.LocalY)
		}
		if spawn.MapID > 0 && spawn.MapID < 34 && (spawn.LocalX >= 1000 || spawn.LocalY >= 1000) {
			t.Fatalf("monster spawn %d was not converted to local map coordinates map=%d x=%d y=%d",
				spawn.SpawnID, spawn.MapID, spawn.LocalX, spawn.LocalY)
		}
		if spawn.AI == "" {
			t.Fatalf("spawn %d has empty ai", spawn.SpawnID)
		}
		if spawn.SpawnDistance > 0 {
			spawnsWithDistance++
		}
		mapCounts[spawn.MapID]++
	}
	if spawnsWithDistance == 0 {
		t.Fatal("no monster spawns loaded spawn distance data")
	}

	expectedMapCounts := map[uint16]int{
		0: 348, 1: 355, 2: 342, 3: 349, 4: 378, 5: 219, 6: 323,
		7: 307, 8: 285, 9: 295, 10: 332, 11: 293, 12: 340,
		13: 177, 14: 357, 15: 308, 16: 363, 17: 355, 18: 380,
		20: 371, 23: 386, 24: 308, 25: 383, 26: 376, 27: 385,
		34: 899,
	}
	for mapID, want := range expectedMapCounts {
		if got := mapCounts[mapID]; got != want {
			t.Fatalf("map %d spawn count=%d, want %d", mapID, got, want)
		}
	}
	if len(mapCounts) != len(expectedMapCounts) {
		t.Fatalf("map count=%d, want %d: %#v", len(mapCounts), len(expectedMapCounts), mapCounts)
	}

	drops, dropSource, ok, err := LoadMonsterDrops(root)
	if err != nil {
		t.Fatalf("LoadMonsterDrops() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadMonsterDrops() did not find static data")
	}
	if dropSource == "" {
		t.Fatal("LoadMonsterDrops() source is empty")
	}
	if len(drops) != 29419 {
		t.Fatalf("drop count=%d, want 29419", len(drops))
	}
	for _, drop := range drops {
		if drop.EntryID == 0 || drop.ItemID <= 0 || drop.Chance <= 0 {
			t.Fatalf("invalid drop row: %#v", drop)
		}
		if drop.MinAmount <= 0 || drop.MaxAmount < drop.MinAmount {
			t.Fatalf("invalid drop amount range: %#v", drop)
		}
	}
}

func TestLoadNpcVendorItemsFromStaticData(t *testing.T) {
	root := t.TempDir()
	npcRoot := filepath.Join(root, "npcs")
	if err := os.MkdirAll(npcRoot, 0o755); err != nil {
		t.Fatalf("create npc data dir: %v", err)
	}
	data := `[
  {"vendorId": 3, "items": [
    {"itemId": 20001},
    {"itemId": 20002, "sortOrder": 5},
    {"itemId": 99999, "enabled": false}
  ]},
  {"npcId": 4, "itemId": 30001, "sortOrder": 2}
]`
	if err := os.WriteFile(filepath.Join(npcRoot, "vendors.json"), []byte(data), 0o644); err != nil {
		t.Fatalf("write vendors.json: %v", err)
	}

	rows, source, ok, err := LoadNpcVendorItems(root)
	if err != nil {
		t.Fatalf("LoadNpcVendorItems() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadNpcVendorItems() did not find static data")
	}
	if source == "" {
		t.Fatal("LoadNpcVendorItems() source is empty")
	}
	if len(rows) != 3 {
		t.Fatalf("vendor item count=%d, want 3", len(rows))
	}
	if rows[0].VendorEntryID != 3 || rows[0].ItemID != 20001 || rows[0].SortOrder != 1 {
		t.Fatalf("first vendor item mismatch: %#v", rows[0])
	}
	if rows[1].VendorEntryID != 3 || rows[1].ItemID != 20002 || rows[1].SortOrder != 5 {
		t.Fatalf("second vendor item mismatch: %#v", rows[1])
	}
	if rows[2].VendorEntryID != 4 || rows[2].ItemID != 30001 || rows[2].SortOrder != 2 {
		t.Fatalf("third vendor item mismatch: %#v", rows[2])
	}
}

func TestLoadNpcsFromRepoData(t *testing.T) {
	root := filepath.Join("..", "..", "data", "asda2")
	templates, spawns, source, ok, err := LoadNpcs(root, 0)
	if err != nil {
		t.Fatalf("LoadNpcs() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadNpcs() did not find static data")
	}
	if source == "" {
		t.Fatal("LoadNpcs() source is empty")
	}
	if len(templates) != 336 {
		t.Fatalf("npc template count=%d, want 336", len(templates))
	}
	if len(spawns) != 310 {
		t.Fatalf("npc spawn count=%d, want 310", len(spawns))
	}

	templatesByID := make(map[uint16]types.NpcTemplateRow, len(templates))
	for _, template := range templates {
		if template.EntryID == 0 || template.Name == "" {
			t.Fatalf("invalid npc template: %#v", template)
		}
		templatesByID[template.EntryID] = template
	}
	if got := templatesByID[12]; !got.IsTrainer ||
		got.InteractionKind != types.NpcInteractionTrainer ||
		got.ClassGroup != types.NpcClassGroupWarrior {
		t.Fatalf("warrior trainer metadata was not derived: %#v", got)
	}
	if got := templatesByID[3]; got.InteractionKind != types.NpcInteractionVendor {
		t.Fatalf("shop metadata was not derived: %#v", got)
	}
	if got := templatesByID[2]; got.InteractionKind != types.NpcInteractionQuest {
		t.Fatalf("bulletin board metadata was not derived: %#v", got)
	}

	mapCounts := make(map[uint16]int)
	spawnIDs := make(map[uint32]struct{}, len(spawns))
	for _, spawn := range spawns {
		if _, ok := templatesByID[spawn.EntryID]; !ok {
			t.Fatalf("npc spawn %d uses unknown template %d", spawn.SpawnID, spawn.EntryID)
		}
		if spawn.SpawnID == 0 {
			t.Fatalf("npc spawn has zero id: map=%d entry=%d x=%d y=%d",
				spawn.MapID, spawn.EntryID, spawn.LocalX, spawn.LocalY)
		}
		if _, exists := spawnIDs[spawn.SpawnID]; exists {
			t.Fatalf("duplicate npc spawn id %d", spawn.SpawnID)
		}
		spawnIDs[spawn.SpawnID] = struct{}{}
		if spawn.LocalX < 0 || spawn.LocalY < 0 {
			t.Fatalf("npc spawn %d has negative local coordinates map=%d x=%d y=%d",
				spawn.SpawnID, spawn.MapID, spawn.LocalX, spawn.LocalY)
		}
		if spawn.MapID > 0 && (spawn.LocalX >= 1000 || spawn.LocalY >= 1000) {
			t.Fatalf("npc spawn %d was not converted to local map coordinates map=%d x=%d y=%d",
				spawn.SpawnID, spawn.MapID, spawn.LocalX, spawn.LocalY)
		}
		mapCounts[spawn.MapID]++
	}

	expectedMapCounts := map[uint16]int{
		0: 42, 1: 20, 2: 16, 3: 48, 4: 1, 5: 37, 6: 17, 7: 30,
		8: 4, 9: 7, 10: 10, 11: 4, 12: 1, 13: 9, 14: 1, 15: 1,
		16: 10, 17: 1, 18: 1, 19: 7, 20: 4, 21: 1, 22: 2, 23: 14,
		24: 1, 25: 9, 26: 10, 27: 1, 29: 1,
	}
	for mapID, want := range expectedMapCounts {
		if got := mapCounts[mapID]; got != want {
			t.Fatalf("npc map %d spawn count=%d, want %d", mapID, got, want)
		}
	}
	if len(mapCounts) != len(expectedMapCounts) {
		t.Fatalf("npc map count=%d, want %d: %#v", len(mapCounts), len(expectedMapCounts), mapCounts)
	}
}
