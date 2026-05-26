package types

// BaseStatsRow is the Asda2-only stat table used by login, character select,
// and game runtime. Attr1..Attr6 intentionally preserve the source column names
// until the exact client stat-display mapping is fully verified.
type BaseStatsRow struct {
	ClassID    byte
	Level      byte
	BaseHealth int
	BasePower  int
	Attr1      int
	Attr2      int
	Attr3      int
	Attr4      int
	Attr5      int
	Attr6      int
}

// BaseStatsClassID maps the current Asda2 class ids into the four class stat
// tables from the official config dump.
func BaseStatsClassID(asda2Class byte) byte {
	switch {
	case asda2Class >= 1 && asda2Class <= 3:
		return 1 // warrior line
	case asda2Class >= 4 && asda2Class <= 6:
		return 2 // archer line
	case asda2Class >= 7 && asda2Class <= 9:
		return 3 // mage line
	default:
		return 0 // starter/unknown line
	}
}
