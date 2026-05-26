package types

import (
	"encoding/binary"
	"math"
	"sync/atomic"
	"time"
)

// Character holds all runtime state for a logged-in player.
// Mirrors WCell.RealmServer/Entities/Character.cs (Asda2 fields).
type Character struct {
	// Identity
	GUID      uint32
	Name      string
	SessionID int16 // unique within the current map session
	AccID     uint32
	MapID     uint16

	// Position (Asda2 uses float32 x/y, no Z)
	X         float32
	Y         float32
	MoveDestX float32
	MoveDestY float32
	RunSpeed  float32
	// Orientation is stored in radians. LastFacing keeps the Asda2 local XY
	// direction so standing characters can face the same way on other clients.
	Orientation float32
	LastFacingX float32
	LastFacingY float32
	// Runtime-only movement clock. Positions are persisted through X/Y on save.
	MoveLastUpdate time.Time

	// Base stats
	Level byte
	HP    int32
	MaxHP int32
	MP    int32
	MaxMP int32
	Exp   int64
	Gold  int64

	// Allocated base stats (STR/AGI/STAM/SPIRIT/INT)
	BaseStrength  int16
	BaseAgility   int16
	BaseStamina   int16
	BaseSpirit    int16
	BaseIntellect int16
	BaseLuck      int16

	// Character appearance
	CharNum   byte // character slot: 10, 11, or 12
	Race      byte
	Class     byte // Asda2 class (warrior, mage, etc.)
	Gender    byte
	Hair      byte
	HairColor byte
	Face      byte
	EyeColor  byte
	Zodiac    byte

	// Profession
	ProfessionLevel byte

	// Asda2 faction / PvP
	FactionID   int16 // -1 = neutral
	FactionRank int16
	HonorPoints int32
	TitlePoints int32
	Rank        int32
	IsInDuel    bool

	// Titles
	DiscoveredTitles [4]uint32 // bitmask, 128 bits
	GetedTitles      [4]uint32
	PreTitleID       int16
	PostTitleID      int16

	// Fishing
	FishingLevel int32

	// Crafting
	CraftingLevel  byte
	CraftingExp    float32
	LearnedRecipes [LearnedRecipeFlagCount]bool

	// Inventory (slot → item GUID; full implementation in items.go)
	RegularInventory   [60]int64
	ShopInventory      [61]int64
	AvatarInventory    [30]int64
	WarehouseInventory [270]int64
	Items              []*ItemRow
	FastSlots          []*FastSlotRow
	TeleportPoints     [10]*TeleportPointRow

	// Warehouse
	WarehousePassword               string
	PremiumWarehouseBagsCount       byte
	PremiumAvatarWarehouseBagsCount byte

	// Combat state
	IsFighting       bool
	IsMoving         bool
	TargetID         int16 // SessionID of current target (-1 = none)
	BonusSkillPoints int
	LearnedSkills    map[int16]byte
	SkillCooldowns   map[int16]time.Time
	SkillBuffExpires map[int16]time.Time

	// Runtime skill modifiers. These mirror small Asda2 aura slices without
	// importing WCell's full aura/stat modifier graph.
	SkillDamageBonusPct            float32
	SkillMagicDamageBonusPct       float32
	SkillDefenseBonusPct           float32
	SkillMagicDefenseBonusPct      float32
	SkillSpeedBonusPct             float32
	SkillHealingDonePct            float32
	GuildSkillDamageBonusPct       float32
	GuildSkillMagicDamageBonusPct  float32
	GuildSkillDefenseBonusPct      float32
	GuildSkillMagicDefenseBonusPct float32
	GuildSkillSpeedBonusPct        float32

	// SoulGuard runtime state. Charge persistence is intentionally runtime-only
	// until the full WCell soul-guard token table is ported.
	GreenCharges       byte
	BlueCharges        byte
	RedCharges         byte
	SoulBuffed1        bool
	SoulBuffed2        bool
	SoulBuffed3        bool
	CurrentSoulGuardID int16

	// Soulmate runtime state used by the focused skill handler.
	SoulmateGUID           uint32
	SoulmateLevel          byte
	CanTeleportToFriend    bool
	TargetSummonMap        uint16
	TargetSummonX          float32
	TargetSummonY          float32
	SoulmateSkillCooldowns map[int16]time.Time

	// Social
	GuildID           uint32
	GuildRank         byte
	GuildPoints       int32
	PartyID           uint32
	GlobalChatColorDB int

	// Buffs (simplified; full aura system in spells.go)
	Buffs [32]int32 // spell IDs of active buffs

	// Settings flags (from CharacterRecord)
	SettingsFlags [16]byte

	// Misc
	RebornCount byte
	WingsItemID int16
	AvatarMask  int32
	ChatBanned  bool
}

type TeleportPointRow struct {
	Guid    int64
	OwnerID uint32
	Slot    byte
	Name    string
	MapID   uint16
	X       int16
	Y       int16
}

type CharacterRow struct {
	// Identity
	EntityLowID int64 // column: EntityLowId  (PK)
	AccountID   int   // column: AccountId
	Name        string
	CharNum     byte // slot 0-2 on the account
	Created     time.Time

	// Appearance
	Race       byte // RaceId
	Class      int  // ClassId (column: ClassId)
	Asda2Class byte
	Gender     byte
	Skin       byte
	Face       byte
	HairStyle  byte
	HairColor  byte
	EyesColor  byte
	AvatarMask int

	// Position / Map
	PositionX   float32
	PositionY   float32
	PositionZ   float32
	Orientation float32
	Map         int // MapId (column: Map)

	// Stats
	Level      int
	Xp         int
	Health     int
	BaseHealth int
	Power      int
	BasePower  int
	Money      int64

	// Base stat allocation
	BaseStrength     int
	BaseStamina      int
	BaseSpirit       int
	BaseIntellect    int
	BaseAgility      int
	BaseLuck         int
	FreeStatPoints   int
	BonusSkillPoints int

	// Asda2 PvP / faction
	Asda2FactionID   int16
	Asda2HonorPoints int
	TitlePoints      int
	Rank             int

	// Titles (stored as 16×uint32 bitmasks, serialised as BLOB in WCell)
	// We load/save them as raw bytes and interpret bit-by-bit.
	DiscoveredTitlesRaw []byte // 64 bytes (16 × uint32)
	GetedTitlesRaw      []byte // 64 bytes
	PreTitleID          int16
	PostTitleID         int16

	// Profession / Crafting / Fishing
	ProfessionLevel   byte
	FishingLevel      int
	CraftingLevel     byte
	CraftingExp       float32
	LearnedRecipesRaw []byte // 72 bytes (18 × uint32)

	// Chat / social
	GlobalChatColorDB int
	ChatBanned        bool
	GuildID           int
	GuildPoints       int

	// Warehouse
	WarehousePassword               string
	PremiumWarehouseBagsCount       byte
	PremiumAvatarWarehouseBagsCount byte

	// Misc
	RebornCount     int
	Zodiac          byte
	SettingsFlags   []byte // 16 bytes
	PetBoxEnchants  byte
	MountBoxExpands byte

	// Items loaded for character-select screen (InventoryType==3 = equipped/avatar)
	// Populated by GetCharactersByAccount, mirrors CharacterRecord.Asda2LoadedItems.
	LoadedItems     []*ItemRow
	LoadedFastSlots []*FastSlotRow
	LoadedTeleports [10]*TeleportPointRow
	LearnedSkills   map[int16]byte
}

var nextSessionID int32

// DefaultSettingsFlags mirrors CharacterRecord.SetupNewRecord: all request
// toggles enabled unless the client explicitly changes them.
var DefaultSettingsFlags = [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

// AllocSessionID returns a per-map unique session id and wraps at 0x7FFF.
func AllocSessionID() int16 {
	return int16(atomic.AddInt32(&nextSessionID, 1) & 0x7FFF)
}

// MapOffset returns the coordinate offset for a map.
// Mirrors Map.Offset = (float)rgnTemplate.Id * 1000f.
func MapOffset(mapID uint16) float32 {
	mapID = NormalizeAsda2MapID(mapID)
	return float32(mapID) * 1000.0
}

// Asda2X converts a raw world X coordinate to the local map X the client expects.
func Asda2X(worldX float32, mapID uint16) float32 {
	return worldX - MapOffset(mapID)
}

// Asda2Y converts a raw world Y coordinate to the local map Y the client expects.
func Asda2Y(worldY float32, mapID uint16) float32 {
	return worldY - MapOffset(mapID)
}

func CharacterFromRow(r *CharacterRow, accID uint32) *Character {
	maxHP := r.BaseHealth
	if maxHP <= 0 {
		maxHP = r.Health
	}
	hp := r.Health
	if hp <= 0 || hp > maxHP {
		hp = maxHP
	}
	maxMP := r.BasePower
	if maxMP <= 0 {
		maxMP = r.Power
	}
	mp := r.Power
	if mp <= 0 || mp > maxMP {
		mp = maxMP
	}

	chr := &Character{
		GUID:                            uint32(r.EntityLowID),
		AccID:                           accID,
		Name:                            r.Name,
		CharNum:                         r.CharNum,
		SessionID:                       AllocSessionID(),
		MapID:                           NormalizeAsda2MapID(uint16(r.Map)),
		X:                               r.PositionX,
		Y:                               r.PositionY,
		Orientation:                     r.Orientation,
		LastFacingX:                     float32(math.Cos(float64(r.Orientation))),
		LastFacingY:                     float32(math.Sin(float64(r.Orientation))),
		Level:                           byte(r.Level),
		HP:                              int32(hp),
		MaxHP:                           int32(maxHP),
		MP:                              int32(mp),
		MaxMP:                           int32(maxMP),
		Exp:                             int64(r.Xp),
		Gold:                            r.Money,
		BaseStrength:                    int16(r.BaseStrength),
		BaseAgility:                     int16(r.BaseAgility),
		BaseStamina:                     int16(r.BaseStamina),
		BaseSpirit:                      int16(r.BaseSpirit),
		BaseIntellect:                   int16(r.BaseIntellect),
		BaseLuck:                        int16(r.BaseLuck),
		Race:                            r.Race,
		Class:                           r.Asda2Class,
		Gender:                          r.Gender,
		Hair:                            r.HairStyle,
		HairColor:                       r.HairColor,
		Face:                            r.Face,
		EyeColor:                        r.EyesColor,
		Zodiac:                          r.Zodiac,
		ProfessionLevel:                 r.ProfessionLevel,
		FactionID:                       r.Asda2FactionID,
		HonorPoints:                     int32(r.Asda2HonorPoints),
		TitlePoints:                     int32(r.TitlePoints),
		Rank:                            int32(r.Rank),
		PreTitleID:                      r.PreTitleID,
		PostTitleID:                     r.PostTitleID,
		FishingLevel:                    int32(r.FishingLevel),
		CraftingLevel:                   r.CraftingLevel,
		CraftingExp:                     r.CraftingExp,
		LearnedRecipes:                  DecodeLearnedRecipeMask(r.LearnedRecipesRaw),
		Items:                           r.LoadedItems,
		FastSlots:                       r.LoadedFastSlots,
		TeleportPoints:                  r.LoadedTeleports,
		WarehousePassword:               r.WarehousePassword,
		PremiumWarehouseBagsCount:       r.PremiumWarehouseBagsCount,
		PremiumAvatarWarehouseBagsCount: r.PremiumAvatarWarehouseBagsCount,
		TargetID:                        -1,
		BonusSkillPoints:                r.BonusSkillPoints,
		LearnedSkills:                   make(map[int16]byte),
		SkillCooldowns:                  make(map[int16]time.Time),
		SkillBuffExpires:                make(map[int16]time.Time),
		SoulmateSkillCooldowns:          make(map[int16]time.Time),
		CurrentSoulGuardID:              -1,
		GuildID:                         uint32(r.GuildID),
		GuildPoints:                     int32(r.GuildPoints),
		GlobalChatColorDB:               r.GlobalChatColorDB,
		ChatBanned:                      r.ChatBanned,
		RebornCount:                     byte(r.RebornCount),
		AvatarMask:                      int32(r.AvatarMask),
		RunSpeed:                        0.259,
	}
	copy(chr.SettingsFlags[:], DefaultSettingsFlags[:])
	if len(r.SettingsFlags) >= len(chr.SettingsFlags) {
		copy(chr.SettingsFlags[:], r.SettingsFlags[:len(chr.SettingsFlags)])
	}
	for skillID, level := range r.LearnedSkills {
		if level > 0 {
			chr.LearnedSkills[skillID] = level
		}
	}
	return chr
}

const LearnedRecipeFlagCount = 576

func DecodeLearnedRecipeMask(raw []byte) [LearnedRecipeFlagCount]bool {
	var out [LearnedRecipeFlagCount]bool
	if len(raw) >= LearnedRecipeFlagCount {
		for i := range out {
			out[i] = raw[i] != 0
		}
		return out
	}
	for block := 0; block+3 < len(raw); block += 4 {
		bits := binary.LittleEndian.Uint32(raw[block:])
		base := (block / 4) * 32
		for bit := 0; bit < 32 && base+bit < len(out); bit++ {
			out[base+bit] = bits&(1<<uint(bit)) != 0
		}
	}
	return out
}

func EncodeLearnedRecipeMask(flags [LearnedRecipeFlagCount]bool) []byte {
	raw := make([]byte, LearnedRecipeFlagCount/8)
	for i, learned := range flags {
		if !learned {
			continue
		}
		block := i / 32
		bit := uint(i % 32)
		offset := block * 4
		value := binary.LittleEndian.Uint32(raw[offset:])
		value |= 1 << bit
		binary.LittleEndian.PutUint32(raw[offset:], value)
	}
	return raw
}

func LearnedRecipeCount(flags [LearnedRecipeFlagCount]bool) int {
	count := 0
	for _, learned := range flags {
		if learned {
			count++
		}
	}
	return count
}

// AvailableSkillPoints mirrors the Asda2 WCell spell-point budget without
// importing the full WCell spell collection.
func (c *Character) AvailableSkillPoints() int {
	if c == nil {
		return 0
	}
	used := 0
	for _, level := range c.LearnedSkills {
		used += int(level)
	}
	points := int(c.Level) + 13 + c.BonusSkillPoints - used
	if points < 0 {
		return 0
	}
	return points
}

func (c *Character) RealProfessionLevel() byte {
	if c == nil {
		return 0
	}
	return RealProfessionLevel(c.Class, c.ProfessionLevel)
}
