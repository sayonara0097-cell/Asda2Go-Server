package types

import "strings"

type NpcInteractionKind byte

const (
	NpcInteractionNone NpcInteractionKind = iota
	NpcInteractionTrainer
	NpcInteractionVendor
	NpcInteractionQuest
	NpcInteractionDialogue
)

const (
	NpcClassGroupAll byte = iota
	NpcClassGroupWarrior
	NpcClassGroupArcher
	NpcClassGroupMage
)

// NpcTemplateRow is the Asda2-only subset needed by the current NPC runtime.
// Keep this lean; add fields only when a game system actually consumes them.
type NpcTemplateRow struct {
	EntryID         uint16
	Name            string
	Kind            byte
	ClassGroup      byte
	IsTrainer       bool
	InteractionKind NpcInteractionKind
}

// NpcSpawnRow describes one persistent NPC spawn point.
type NpcSpawnRow struct {
	SpawnID   uint32
	EntryID   uint16
	MapID     uint16
	LocalX    int16
	LocalY    int16
	Channel   int16 // -1 means all channels
	IsEnabled bool
}

// NpcVendorItemRow mirrors WCell's RegularShopRecord for Asda2 shops.
// VendorEntryID is the NPC template id (RegularShopRecord.NpcId).
type NpcVendorItemRow struct {
	VendorEntryID uint16
	ItemID        int
	SortOrder     int
	IsEnabled     bool
}

// PortalRow mirrors the Asda2Portal table used by WCell's walk-in portals.
// Coordinates are local map coordinates; runtime code converts them through
// MapOffset when teleporting.
type PortalRow struct {
	ID      uint32
	FromX   int16
	FromY   int16
	FromMap uint16
	ToX     int16
	ToY     int16
	ToMap   uint16
}

func NormalizeNpcTemplate(row NpcTemplateRow) NpcTemplateRow {
	row.Name = strings.TrimSpace(row.Name)
	name := strings.ToLower(row.Name)

	if row.IsTrainer || row.InteractionKind == NpcInteractionTrainer || strings.Contains(name, "trainer") {
		row.IsTrainer = true
		row.InteractionKind = NpcInteractionTrainer
		if row.ClassGroup == NpcClassGroupAll {
			row.ClassGroup = NpcTrainerClassGroupFromName(row.Name)
		}
		return row
	}

	if row.InteractionKind != NpcInteractionNone {
		return row
	}

	switch {
	case strings.Contains(name, "shop"), strings.Contains(name, "merchant"):
		row.InteractionKind = NpcInteractionVendor
	case strings.Contains(name, "bulletin"), strings.Contains(name, "quest"):
		row.InteractionKind = NpcInteractionQuest
	default:
		row.InteractionKind = NpcInteractionDialogue
	}
	return row
}

func NpcTrainerClassGroupFromName(name string) byte {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "warrior"):
		return NpcClassGroupWarrior
	case strings.Contains(lower, "archer"):
		return NpcClassGroupArcher
	case strings.Contains(lower, "mage"):
		return NpcClassGroupMage
	default:
		return NpcClassGroupAll
	}
}

func ParseNpcInteractionKind(value string) NpcInteractionKind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trainer":
		return NpcInteractionTrainer
	case "vendor", "shop":
		return NpcInteractionVendor
	case "quest", "questgiver", "quest_giver", "bulletin":
		return NpcInteractionQuest
	case "dialog", "dialogue", "talk":
		return NpcInteractionDialogue
	default:
		return NpcInteractionNone
	}
}
