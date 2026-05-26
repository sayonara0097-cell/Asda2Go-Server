package types

// Asda2MapInfo is the Asda2-only map registry. IDs mirror MapId.cs, while
// ZoneID mirrors ZoneId.cs "*Main" entries for DBs that accidentally store
// zone ids in character/map records.
type Asda2MapInfo struct {
	ID       uint16
	ZoneID   uint16
	Name     string
	Fighting bool
}

const (
	MapIDSilaris         uint16 = 0
	MapIDRainRiver       uint16 = 1
	MapIDConquestLand    uint16 = 2
	MapIDAlpia           uint16 = 3
	MapIDNightValey      uint16 = 4
	MapIDAquaton         uint16 = 5
	MapIDSunnyCoast      uint16 = 6
	MapIDFlamio          uint16 = 7
	MapIDQueenPalace     uint16 = 8
	MapIDCastleOfChess   uint16 = 9
	MapIDFlamioPlains    uint16 = 10
	MapIDNeptillanNode   uint16 = 11
	MapIDFlamionMoutain  uint16 = 12
	MapIDFlabis          uint16 = 13
	MapIDStagnantDesert  uint16 = 14
	MapIDDragonLair      uint16 = 15
	MapIDFirewayForest   uint16 = 16
	MapIDCastleOfChaos   uint16 = 17
	MapIDInferion        uint16 = 18
	MapIDBatleField      uint16 = 19
	MapIDDecaronLab      uint16 = 20
	MapIDFieldOfHonnor   uint16 = 21
	MapIDOX              uint16 = 22
	MapIDIceQuarry       uint16 = 23
	MapIDBurnedoutForest uint16 = 24
	MapIDDesolatedMarsh  uint16 = 25
	MapIDFrigidWastes    uint16 = 26
	MapIDNeverFall       uint16 = 27
	MapIDGuildwave       uint16 = 28
	MapIDFantagle        uint16 = 29
	MapIDWindCanyon      uint16 = 30
	MapIDAstrica         uint16 = 31
	MapIDElysion         uint16 = 32
	MapIDAcanpolis       uint16 = 33
	MapIDLabyrinthos     uint16 = 34
)

var Asda2MapInfos = []Asda2MapInfo{
	{ID: MapIDSilaris, ZoneID: 4988, Name: "Silaris"},
	{ID: MapIDRainRiver, ZoneID: 4989, Name: "RainRiver"},
	{ID: MapIDConquestLand, ZoneID: 4990, Name: "ConquestLand"},
	{ID: MapIDAlpia, ZoneID: 4991, Name: "Alpia"},
	{ID: MapIDNightValey, ZoneID: 4992, Name: "NightValey"},
	{ID: MapIDAquaton, ZoneID: 4993, Name: "Aquaton"},
	{ID: MapIDSunnyCoast, ZoneID: 4994, Name: "SunnyCoast"},
	{ID: MapIDFlamio, ZoneID: 4995, Name: "Flamio"},
	{ID: MapIDQueenPalace, ZoneID: 4996, Name: "QueenPalace"},
	{ID: MapIDCastleOfChess, ZoneID: 4997, Name: "CastleOfChess"},
	{ID: MapIDFlamioPlains, ZoneID: 4998, Name: "FlamioPlains"},
	{ID: MapIDNeptillanNode, ZoneID: 4999, Name: "NeptillanNode"},
	{ID: MapIDFlamionMoutain, ZoneID: 5000, Name: "FlamionMoutain"},
	{ID: MapIDFlabis, ZoneID: 5001, Name: "Flabis"},
	{ID: MapIDStagnantDesert, ZoneID: 5002, Name: "StagnantDesert"},
	{ID: MapIDDragonLair, ZoneID: 5003, Name: "DragonLair"},
	{ID: MapIDFirewayForest, ZoneID: 5004, Name: "FirewayForest"},
	{ID: MapIDCastleOfChaos, ZoneID: 5005, Name: "CastleOfChaos"},
	{ID: MapIDInferion, ZoneID: 5006, Name: "Inferion"},
	{ID: MapIDBatleField, ZoneID: 5007, Name: "BatleField", Fighting: true},
	{ID: MapIDDecaronLab, ZoneID: 5008, Name: "DecaronLab"},
	{ID: MapIDFieldOfHonnor, ZoneID: 5009, Name: "FieldOfHonnor"},
	{ID: MapIDOX, ZoneID: 5010, Name: "OX"},
	{ID: MapIDIceQuarry, ZoneID: 5011, Name: "IceQuarry"},
	{ID: MapIDBurnedoutForest, ZoneID: 5013, Name: "BurnedoutForest"},
	{ID: MapIDDesolatedMarsh, ZoneID: 5015, Name: "DesolatedMarsh"},
	{ID: MapIDFrigidWastes, ZoneID: 5014, Name: "FrigidWastes"},
	{ID: MapIDNeverFall, ZoneID: 5012, Name: "NeverFall"},
	{ID: MapIDGuildwave, ZoneID: 5016, Name: "Guildwave"},
	{ID: MapIDFantagle, ZoneID: 5017, Name: "Fantagle"},
	{ID: MapIDWindCanyon, ZoneID: 5018, Name: "WindCanyon"},
	{ID: MapIDAstrica, ZoneID: 5019, Name: "Astrica"},
	{ID: MapIDElysion, ZoneID: 5020, Name: "Elysion"},
	{ID: MapIDAcanpolis, ZoneID: 5021, Name: "Acanpolis"},
	{ID: MapIDLabyrinthos, ZoneID: 5022, Name: "Labyrinthos"},
}

var asda2MapByID = buildAsda2MapByID()
var asda2MapIDByZoneID = buildAsda2MapIDByZoneID()

func buildAsda2MapByID() map[uint16]Asda2MapInfo {
	out := make(map[uint16]Asda2MapInfo, len(Asda2MapInfos))
	for _, info := range Asda2MapInfos {
		out[info.ID] = info
	}
	return out
}

func buildAsda2MapIDByZoneID() map[uint16]uint16 {
	out := make(map[uint16]uint16, len(Asda2MapInfos))
	for _, info := range Asda2MapInfos {
		out[info.ZoneID] = info.ID
	}
	return out
}

func NormalizeAsda2MapID(mapID uint16) uint16 {
	if _, ok := asda2MapByID[mapID]; ok {
		return mapID
	}
	if normalized, ok := asda2MapIDByZoneID[mapID]; ok {
		return normalized
	}
	return mapID
}

func IsAsda2MapID(mapID uint16) bool {
	_, ok := asda2MapByID[NormalizeAsda2MapID(mapID)]
	return ok
}
