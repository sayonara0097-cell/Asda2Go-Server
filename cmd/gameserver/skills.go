package main

import (
	"asda2/shared/types"
	"log"
	"sort"
	"time"
)

type skillUseResult byte

const (
	skillUseCannotApply            skillUseResult = 0
	skillUseOK                     skillUseResult = 1
	skillUseLowMP                  skillUseResult = 2
	skillUseWrongJob               skillUseResult = 3
	skillUseCooldown               skillUseResult = 7
	skillUseCannotUseAgainstPlayer skillUseResult = 8
	skillUseDistanceTooFar         skillUseResult = 10
	skillUseInvalidTarget          skillUseResult = 11
	skillUsePlayerIsDead           skillUseResult = 17
	skillUseNoTarget               skillUseResult = 20
	skillUseMoving                 skillUseResult = 24
	skillUseItIsNotAnActiveSkill   skillUseResult = 6
	skillStrikeHitOK                              = 0
	defaultSkillEffectID           int32          = 271
	defaultSkillEffect0Misc        int16          = 0
)

type skillLearnStatus byte

const (
	skillLearnOK              skillLearnStatus = 1
	skillLearnBadSpellLevel   skillLearnStatus = 2
	skillLearnLevelIsMax      skillLearnStatus = 3
	skillLearnBadProfession   skillLearnStatus = 4
	skillLearnNotEnoughMoney  skillLearnStatus = 5
	skillLearnNotEnoughPoints skillLearnStatus = 6
	skillLearnLowLevel        skillLearnStatus = 8
	skillLearnFail            skillLearnStatus = 12
	skillClassAll             byte             = types.NpcClassGroupAll
	skillClassWarrior         byte             = types.NpcClassGroupWarrior
	skillClassArcher          byte             = types.NpcClassGroupArcher
	skillClassMage            byte             = types.NpcClassGroupMage
)

type SkillTemplate struct {
	ID                      int16
	Level                   byte
	ClassGroup              byte
	ClassMask               uint16
	RequiredLevel           byte
	RequiredProfessionLevel byte
	PowerCost               int16
	MoneyCost               int64
	Cooldown                time.Duration
	CastTime                time.Duration
	Duration                time.Duration
	Damage                  int32
	Range                   float64
	EffectID                int32
	Effect0Misc             int16
	Effect0Type             int16
	Effect0BasePoints       int32
	Effect1Type             int16
	Effect1BasePoints       int32
	TargetFlags             uint32
	RequiredTargetType      byte
	SoulGuardLevel          byte
	IsPassive               bool
	Ranks                   map[byte]SkillRankTemplate
}

type SkillRankTemplate struct {
	Level                   byte
	RequiredLevel           byte
	RequiredProfessionLevel byte
	PowerCost               int16
	MoneyCost               int64
	Cooldown                time.Duration
	CastTime                time.Duration
	Duration                time.Duration
	Range                   float64
	Effect0Misc             int16
	Effect0Type             int16
	Effect0BasePoints       int32
	Effect1Type             int16
	Effect1BasePoints       int32
	EffectID                int32
	TargetFlags             uint32
	RequiredTargetType      byte
	SoulGuardLevel          byte
	IsPassive               bool
}

var (
	// Temporary Asda2 MVP skills. These are small, explicit entries from the
	// Asda2 spell-line IDs, not a port of WCell's full WoW spell system.
	defaultSkillIDs       = []int16{1071, 1072}
	legacyDefaultSkillIDs = []int16{2071, 2072}
	warriorSkillIDs       = []int16{501, 502, 503}
	archerSkillIDs        = []int16{701, 702, 704}
	mageSkillIDs          = []int16{901, 903, 905, 906}
	skillTemplates        = map[int16]SkillTemplate{
		1071: {
			ID:          1071, // Bash
			Level:       1,
			ClassGroup:  skillClassAll,
			PowerCost:   3,
			Cooldown:    2 * time.Second,
			Damage:      16,
			Range:       3.2,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		1072: {
			ID:          1072, // ArrowStrike
			Level:       1,
			ClassGroup:  skillClassAll,
			PowerCost:   3,
			Cooldown:    2 * time.Second,
			Damage:      16,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		501: {
			ID:          501, // FatalBlow
			Level:       1,
			ClassGroup:  skillClassWarrior,
			PowerCost:   3,
			Cooldown:    2 * time.Second,
			Damage:      20,
			Range:       3.2,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		502: {
			ID:          502, // DeepPierce
			Level:       1,
			ClassGroup:  skillClassWarrior,
			PowerCost:   4,
			Cooldown:    2 * time.Second,
			Damage:      22,
			Range:       3.2,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		503: {
			ID:          503, // JumpSlash
			Level:       1,
			ClassGroup:  skillClassWarrior,
			PowerCost:   5,
			Cooldown:    3 * time.Second,
			Damage:      24,
			Range:       3.8,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		701: {
			ID:          701, // Barrage
			Level:       1,
			ClassGroup:  skillClassArcher,
			PowerCost:   4,
			Cooldown:    2 * time.Second,
			Damage:      18,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		702: {
			ID:          702, // BloodyArrow
			Level:       1,
			ClassGroup:  skillClassArcher,
			PowerCost:   5,
			Cooldown:    2 * time.Second,
			Damage:      20,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		704: {
			ID:          704, // AimedShot
			Level:       1,
			ClassGroup:  skillClassArcher,
			PowerCost:   6,
			Cooldown:    3 * time.Second,
			Damage:      24,
			Range:       9.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		901: {
			ID:          901, // BlazingEarth
			Level:       1,
			ClassGroup:  skillClassMage,
			PowerCost:   7,
			Cooldown:    2 * time.Second,
			Damage:      24,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		903: {
			ID:          903, // WindBlade
			Level:       1,
			ClassGroup:  skillClassMage,
			PowerCost:   6,
			Cooldown:    2 * time.Second,
			Damage:      22,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		905: {
			ID:          905, // Fireball
			Level:       1,
			ClassGroup:  skillClassMage,
			PowerCost:   8,
			Cooldown:    3 * time.Second,
			Damage:      25,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
		906: {
			ID:          906, // Lightning
			Level:       1,
			ClassGroup:  skillClassMage,
			PowerCost:   8,
			Cooldown:    3 * time.Second,
			Damage:      26,
			Range:       8.0,
			EffectID:    defaultSkillEffectID,
			Effect0Misc: defaultSkillEffect0Misc,
		},
	}
	skillStrikeStab15  = []byte{0, 0, 0, 0, 0xFF, 0xFF, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	skillLearnStab24   = []byte{8, 0, 224, 147, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	skillRuntimeSource = "static-fallback"
)

var classSkillIDsByClass = map[byte][]int16{
	byte(types.Asda2ClassOHS):         skillRange(501, 547),
	byte(types.Asda2ClassSpear):       skillRange(548, 568),
	byte(types.Asda2ClassTHS):         append(skillRange(569, 616), skillRange(632, 639)...),
	byte(types.Asda2ClassCrossbow):    skillRange(701, 740),
	byte(types.Asda2ClassBow):         skillRange(741, 766),
	byte(types.Asda2ClassBalista):     append(skillRange(767, 825), skillRange(841, 848)...),
	byte(types.Asda2ClassAttackMage):  skillRange(901, 926),
	byte(types.Asda2ClassHealMage):    skillRange(927, 943),
	byte(types.Asda2ClassSupportMage): append(skillRange(944, 1046), skillRange(1062, 1077)...),
}

var professionAutoSkillsByClass = map[byte][]int16{
	byte(types.Asda2ClassOHS):         {2228, 2229, 2230},
	byte(types.Asda2ClassSpear):       {2231, 2232, 2233},
	byte(types.Asda2ClassTHS):         {2234, 2235, 2236},
	byte(types.Asda2ClassCrossbow):    {2237, 2238, 2239},
	byte(types.Asda2ClassBow):         {2240, 2241, 2242},
	byte(types.Asda2ClassBalista):     {2243, 2244, 2245},
	byte(types.Asda2ClassAttackMage):  {2249, 2250, 2251},
	byte(types.Asda2ClassSupportMage): {2252, 2253, 2254},
	byte(types.Asda2ClassHealMage):    {2246, 2247, 2248},
}

var legacyProfessionAutoSkillsByClass = map[byte][]int16{
	byte(types.Asda2ClassOHS):         {1228, 1229, 1230},
	byte(types.Asda2ClassSpear):       {1231, 1232, 1233},
	byte(types.Asda2ClassTHS):         {1234, 1235, 1236},
	byte(types.Asda2ClassCrossbow):    {1237, 1238, 1239},
	byte(types.Asda2ClassBow):         {1240, 1241, 1242},
	byte(types.Asda2ClassBalista):     {1243, 1244, 1245},
	byte(types.Asda2ClassAttackMage):  {1249, 1250, 1251},
	byte(types.Asda2ClassSupportMage): {1252, 1253, 1254},
	byte(types.Asda2ClassHealMage):    {1246, 1247, 1248},
}

func init() {
	for classID, skillIDs := range classSkillIDsByClass {
		classGroup := skillClassGroupForClassID(classID)
		for _, skillID := range skillIDs {
			if _, exists := skillTemplates[skillID]; exists {
				continue
			}
			skillTemplates[skillID] = SkillTemplate{
				ID:          skillID,
				Level:       1,
				ClassGroup:  classGroup,
				PowerCost:   5,
				Cooldown:    3 * time.Second,
				Damage:      20,
				Range:       professionSkillRange(classGroup),
				EffectID:    defaultSkillEffectID,
				Effect0Misc: defaultSkillEffect0Misc,
			}
		}
	}
}

func initSkillRuntime() error {
	rows, err := LoadAsda2SkillTemplates()
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		log.Printf("[Skill] %d templates loaded source=static-fallback", len(skillTemplates))
		return nil
	}
	setSkillTemplatesFromDBRows(rows)
	skillRuntimeSource = "SkillTemplate"
	log.Printf("[Skill] %d templates loaded source=SkillTemplate ranks=%d", len(skillTemplates), len(rows))
	return nil
}

func setSkillTemplatesFromDBRows(rows []Asda2SkillTemplateRow) {
	next := make(map[int16]SkillTemplate)
	for _, row := range rows {
		if row.RealID <= 0 || row.Level == 0 {
			continue
		}
		classGroup := skillClassGroupForClassMask(row.ClassMask)
		if row.ClassGroup != skillClassAll {
			classGroup = row.ClassGroup
		}
		cooldown := time.Duration(row.CooldownMillis) * time.Millisecond
		castTime := time.Duration(row.CastTimeMillis) * time.Millisecond
		duration := time.Duration(row.DurationMillis) * time.Millisecond
		rank := SkillRankTemplate{
			Level:                   row.Level,
			RequiredLevel:           row.LearnLevel,
			RequiredProfessionLevel: row.RequiredProfessionLevel,
			PowerCost:               row.PowerCost,
			MoneyCost:               row.Cost,
			Cooldown:                cooldown,
			CastTime:                castTime,
			Duration:                duration,
			Range:                   float64(row.MaxRange),
			Effect0Misc:             row.Effect0Misc,
			Effect0Type:             row.Effect0Type,
			Effect0BasePoints:       row.Effect0BasePoints,
			Effect1Type:             row.Effect1Type,
			Effect1BasePoints:       row.Effect1BasePoints,
			EffectID:                row.EffectID,
			TargetFlags:             row.TargetFlags,
			RequiredTargetType:      row.RequiredTargetType,
			SoulGuardLevel:          row.SoulGuardLevel,
			IsPassive:               row.IsPassive,
		}
		skill := next[row.RealID]
		if skill.ID == 0 {
			damage := row.Damage
			if damage <= 0 {
				damage = 20
			}
			effectID := row.EffectID
			if effectID == 0 {
				effectID = defaultSkillEffectID
			}
			skill = SkillTemplate{
				ID:                 row.RealID,
				ClassGroup:         classGroup,
				ClassMask:          row.ClassMask,
				Damage:             damage,
				Range:              float64(row.MaxRange),
				EffectID:           effectID,
				Effect0Misc:        row.Effect0Misc,
				Effect0Type:        row.Effect0Type,
				Effect0BasePoints:  row.Effect0BasePoints,
				Effect1Type:        row.Effect1Type,
				Effect1BasePoints:  row.Effect1BasePoints,
				TargetFlags:        row.TargetFlags,
				RequiredTargetType: row.RequiredTargetType,
				SoulGuardLevel:     row.SoulGuardLevel,
				IsPassive:          row.IsPassive,
				Ranks:              make(map[byte]SkillRankTemplate),
			}
		}
		if skill.ClassGroup == skillClassAll || classGroup != skillClassAll {
			skill.ClassGroup = classGroup
		}
		skill.ClassMask |= row.ClassMask
		skill.Ranks[row.Level] = rank
		if row.Level > skill.Level {
			skill.Level = row.Level
		}
		if row.Level == 1 || skill.RequiredLevel == 0 {
			skill.RequiredLevel = row.LearnLevel
			skill.RequiredProfessionLevel = row.RequiredProfessionLevel
			skill.PowerCost = row.PowerCost
			skill.MoneyCost = row.Cost
			skill.Cooldown = cooldown
			skill.CastTime = castTime
			skill.Duration = duration
			skill.Range = float64(row.MaxRange)
			if row.Damage > 0 {
				skill.Damage = row.Damage
			}
			if row.EffectID > 0 {
				skill.EffectID = row.EffectID
			}
			skill.Effect0Misc = row.Effect0Misc
			skill.Effect0Type = row.Effect0Type
			skill.Effect0BasePoints = row.Effect0BasePoints
			skill.Effect1Type = row.Effect1Type
			skill.Effect1BasePoints = row.Effect1BasePoints
			skill.TargetFlags = row.TargetFlags
			skill.RequiredTargetType = row.RequiredTargetType
			skill.SoulGuardLevel = row.SoulGuardLevel
			skill.IsPassive = row.IsPassive
		}
		next[row.RealID] = skill
	}
	if len(next) > 0 {
		skillTemplates = next
	}
}

func skillRange(first, last int16) []int16 {
	if last < first {
		return nil
	}
	out := make([]int16, 0, int(last-first)+1)
	for id := first; id <= last; id++ {
		out = append(out, id)
	}
	return out
}

func classSkillIDsForClass(classID byte) []int16 {
	skillIDs := classSkillIDsByClass[classID]
	out := make([]int16, len(skillIDs))
	copy(out, skillIDs)
	return out
}

func allClassSkillIDs() []int16 {
	var out []int16
	for classID := byte(types.Asda2ClassOHS); classID <= byte(types.Asda2ClassHealMage); classID++ {
		out = append(out, classSkillIDsByClass[classID]...)
	}
	return out
}

func useSkillOnMonster(c *Client, skillID int16, targetType byte, targetID uint16) skillUseResult {
	return useRuntimeSkill(c, skillID, targetType, targetID, runtimeSkillOptions{})
}

func learnRuntimeSkill(c *Client, skillID int16, level byte) (SkillTemplate, skillLearnStatus) {
	if c == nil || c.Char == nil {
		return SkillTemplate{}, skillLearnFail
	}
	skill, ok := skillTemplates[skillID]
	if !ok {
		return SkillTemplate{}, skillLearnFail
	}
	if !skillAvailableForCharacter(c.Char, skill) {
		return SkillTemplate{}, skillLearnBadProfession
	}
	if level == 0 {
		level = 1
	}
	rankSkill, ok := skillTemplateAtLevel(skill, level)
	if !ok {
		return SkillTemplate{}, skillLearnFail
	}
	if rankSkill.RequiredLevel > 0 && c.Char.Level < rankSkill.RequiredLevel {
		return SkillTemplate{}, skillLearnLowLevel
	}
	if rankSkill.RequiredProfessionLevel > 0 && c.Char.RealProfessionLevel() < rankSkill.RequiredProfessionLevel {
		return SkillTemplate{}, skillLearnBadProfession
	}
	if c.Char.LearnedSkills == nil {
		c.Char.LearnedSkills = make(map[int16]byte)
	}
	currentLevel := c.Char.LearnedSkills[skill.ID]
	if currentLevel >= level {
		return SkillTemplate{}, skillLearnLevelIsMax
	}
	if level > 1 && currentLevel != level-1 {
		return SkillTemplate{}, skillLearnBadSpellLevel
	}
	if availableSkillPoints(c.Char) <= 0 {
		return SkillTemplate{}, skillLearnNotEnoughPoints
	}
	if rankSkill.MoneyCost > 0 && c.Char.Gold < rankSkill.MoneyCost {
		return SkillTemplate{}, skillLearnNotEnoughMoney
	}
	if err := SaveCharacterSkill(c.Char.GUID, rankSkill.ID, level); err != nil {
		log.Printf("[Skill] failed to save learned skill char=%d skill=%d level=%d: %v", c.Char.GUID, skill.ID, level, err)
		return SkillTemplate{}, skillLearnFail
	}
	if rankSkill.MoneyCost > 0 {
		c.Char.Gold -= rankSkill.MoneyCost
		if err := SaveCharacter(c.Char); err != nil {
			log.Printf("[Skill] failed to save skill cost char=%d skill=%d level=%d: %v", c.Char.GUID, skill.ID, level, err)
			return SkillTemplate{}, skillLearnFail
		}
	}
	c.Char.LearnedSkills[rankSkill.ID] = level
	log.Printf("[Skill] %q learned skill=%d level=%d class=%d", c.Char.Name, rankSkill.ID, level, c.Char.Class)
	return rankSkill, skillLearnOK
}

func skillLearned(c *Client, skillID int16) bool {
	return c != nil && c.Char != nil && c.Char.LearnedSkills != nil && c.Char.LearnedSkills[skillID] > 0
}

func skillAvailableForCharacter(chr *Character, skill SkillTemplate) bool {
	if chr == nil {
		return false
	}
	if skill.ClassMask != 0 {
		return skill.ClassMask&asda2ClassMaskForCharacter(chr) != 0
	}
	return skill.ClassGroup == skillClassAll || skill.ClassGroup == skillClassGroupForCharacter(chr)
}

func skillTemplateAtLevel(skill SkillTemplate, level byte) (SkillTemplate, bool) {
	if level == 0 {
		level = 1
	}
	if len(skill.Ranks) == 0 {
		if level > skill.Level {
			return SkillTemplate{}, false
		}
		skill.Level = level
		return skill, true
	}
	rank, ok := skill.Ranks[level]
	if !ok {
		return SkillTemplate{}, false
	}
	skill.Level = rank.Level
	skill.RequiredLevel = rank.RequiredLevel
	skill.RequiredProfessionLevel = rank.RequiredProfessionLevel
	skill.PowerCost = rank.PowerCost
	skill.MoneyCost = rank.MoneyCost
	skill.Cooldown = rank.Cooldown
	skill.CastTime = rank.CastTime
	skill.Duration = rank.Duration
	skill.Range = rank.Range
	skill.Effect0Misc = rank.Effect0Misc
	skill.Effect0Type = rank.Effect0Type
	skill.Effect0BasePoints = rank.Effect0BasePoints
	skill.Effect1Type = rank.Effect1Type
	skill.Effect1BasePoints = rank.Effect1BasePoints
	if rank.EffectID > 0 {
		skill.EffectID = rank.EffectID
	}
	skill.TargetFlags = rank.TargetFlags
	skill.RequiredTargetType = rank.RequiredTargetType
	skill.SoulGuardLevel = rank.SoulGuardLevel
	skill.IsPassive = rank.IsPassive
	return skill, true
}

func skillIDsForCharacter(chr *Character) []int16 {
	if skillRuntimeSource == "SkillTemplate" {
		out := make([]int16, 0, len(skillTemplates))
		for skillID, skill := range skillTemplates {
			if skillAvailableForCharacter(chr, skill) {
				out = append(out, skillID)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
		return out
	}

	family := skillProfessionFamilyForCharacter(chr)
	var base []int16
	switch family {
	case types.Asda2ProfessionWarrior:
		base = warriorSkillIDs
	case types.Asda2ProfessionArcher:
		base = archerSkillIDs
	default:
		base = mageSkillIDs
	}
	professionSkills := allProfessionSkillIDsForFamily(family)
	classSkills := classSkillIDsForClass(chr.Class)
	out := make([]int16, 0, len(defaultSkillIDs)+len(professionSkills)+len(classSkills)+len(base))
	out = append(out, defaultSkillIDs...)
	out = append(out, professionSkills...)
	out = append(out, classSkills...)
	out = append(out, base...)
	return uniqueSkillIDs(out)
}

func uniqueSkillIDs(skillIDs []int16) []int16 {
	seen := make(map[int16]struct{}, len(skillIDs))
	out := make([]int16, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		if _, exists := seen[skillID]; exists {
			continue
		}
		seen[skillID] = struct{}{}
		out = append(out, skillID)
	}
	return out
}

func skillProfessionFamilyForCharacter(chr *Character) types.Asda2ProfessionFamily {
	if chr == nil {
		return types.Asda2ProfessionMage
	}
	family := types.Asda2ClassFamily(chr.Class)
	if family != types.Asda2ProfessionNone {
		return family
	}
	return types.Asda2ProfessionMage
}

func skillClassGroupForCharacter(chr *Character) byte {
	switch skillProfessionFamilyForCharacter(chr) {
	case types.Asda2ProfessionWarrior:
		return skillClassWarrior
	case types.Asda2ProfessionArcher:
		return skillClassArcher
	default:
		return skillClassMage
	}
}

func asda2ClassMaskForCharacter(chr *Character) uint16 {
	if chr == nil {
		return 0
	}
	switch types.Asda2Class(chr.Class) {
	case types.Asda2ClassOHS:
		return 2
	case types.Asda2ClassSpear:
		return 4
	case types.Asda2ClassTHS:
		return 8
	case types.Asda2ClassCrossbow:
		return 512
	case types.Asda2ClassBow:
		return 1024
	case types.Asda2ClassBalista:
		return 2048
	case types.Asda2ClassAttackMage, types.Asda2ClassSupportMage, types.Asda2ClassHealMage:
		return 32
	default:
		return 0
	}
}

func skillClassGroupForClassMask(mask uint16) byte {
	if mask == 0 {
		return skillClassAll
	}
	hasWarrior := mask&(2|4|8) != 0
	hasArcher := mask&(512|1024|2048) != 0
	hasMage := mask&32 != 0
	switch {
	case hasWarrior && !hasArcher && !hasMage:
		return skillClassWarrior
	case hasArcher && !hasWarrior && !hasMage:
		return skillClassArcher
	case hasMage && !hasWarrior && !hasArcher:
		return skillClassMage
	default:
		return skillClassAll
	}
}

func availableSkillPoints(chr *Character) int {
	return chr.AvailableSkillPoints()
}

func ensureDefaultSkills(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	if c.Char.LearnedSkills == nil {
		c.Char.LearnedSkills = make(map[int16]byte)
	}
	removedLegacy := false
	for _, skillID := range legacyDefaultSkillIDs {
		if c.Char.LearnedSkills[skillID] > 0 {
			delete(c.Char.LearnedSkills, skillID)
			removedLegacy = true
		}
	}
	if removedLegacy {
		if err := DeleteCharacterSkills(c.Char.GUID, legacyDefaultSkillIDs); err != nil {
			log.Printf("[Skill] failed to remove legacy default skills char=%d: %v", c.Char.GUID, err)
		}
	}
	for _, skillID := range defaultSkillIDs {
		if c.Char.LearnedSkills[skillID] > 0 {
			continue
		}
		c.Char.LearnedSkills[skillID] = 1
		if err := SaveCharacterSkill(c.Char.GUID, skillID, 1); err != nil {
			log.Printf("[Skill] failed to save default skill char=%d skill=%d: %v", c.Char.GUID, skillID, err)
		}
	}
}

func skillCooldownRemaining(c *Client, skillID int16) time.Duration {
	if c == nil || c.Char == nil || c.Char.SkillCooldowns == nil {
		return 0
	}
	expiresAt := c.Char.SkillCooldowns[skillID]
	if expiresAt.IsZero() {
		return 0
	}
	return time.Until(expiresAt)
}

func setSkillCooldown(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	if c.Char.SkillCooldowns == nil {
		c.Char.SkillCooldowns = make(map[int16]time.Time)
	}
	c.Char.SkillCooldowns[skill.ID] = time.Now().Add(skill.Cooldown)
}

func scheduleSkillReady(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil || skill.Cooldown <= 0 {
		return
	}
	sessionID := c.Char.SessionID
	time.AfterFunc(skill.Cooldown, func() {
		current := getClientBySessionID(sessionID)
		if current == c && current.Char != nil {
			sendSkillReady(current, skill.ID)
		}
	})
}

func sendSkillsInfo(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	learned := learnedSkillIDsForCharacter(c)
	for start := 0; start < len(learned); start += 18 {
		end := start + 18
		if end > len(learned) {
			end = len(learned)
		}
		p := NewPacket(SkillsInfo)
		for _, skillID := range learned[start:end] {
			skill := skillTemplates[skillID]
			skill.Level = c.Char.LearnedSkills[skillID]
			writeSkillInfo(p, skill)
		}
		c.Send(p)
	}
}

func learnedSkillIDsForCharacter(c *Client) []int16 {
	if c == nil || c.Char == nil || len(c.Char.LearnedSkills) == 0 {
		return nil
	}
	available := skillIDsForCharacter(c.Char)
	learned := make([]int16, 0, len(available))
	for _, skillID := range available {
		if c.Char.LearnedSkills[skillID] > 0 {
			learned = append(learned, skillID)
		}
	}
	return learned
}

func writeSkillInfo(p *PacketOut, skill SkillTemplate) {
	p.WriteUint16(uint16(skill.ID))
	p.WriteUint8(skill.Level)
	p.WriteUint8(1)
	p.WriteInt32(int32(skill.Cooldown / time.Millisecond))
	p.WriteInt16(256)
	p.WriteInt16(skill.PowerCost)
	p.WriteInt16(skill.Effect0Misc)
	p.WriteUint8(100)
	p.WriteUint8(100)
	p.WriteInt16(4)
	p.WriteInt32(150000)
	p.WriteInt16(0)
	p.WriteInt64(0)
	p.WriteInt16(0)
}

func sendUseSkillResult(c *Client, skillID int16, result skillUseResult) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UseSkillResult)
	p.WriteUint8(byte(result))
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skillID)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	c.Send(p)
}

func sendSkillLearnedResponse(c *Client, status skillLearnStatus, skillID int16, level byte) {
	if c == nil || c.Char == nil {
		return
	}
	chr := c.Char
	stats := calculateCharacterStats(chr)
	p := NewPacket(SkillLearned)
	p.WriteUint8(byte(status))
	p.WriteInt16(int16(availableSkillPoints(chr)))
	p.WriteInt32(clampInt32(chr.Gold))
	p.WriteInt16(skillID)
	p.WriteUint8(level)
	p.WriteUint8(1)
	writeCharacterAttributes(p, stats.Total)
	writeZeroCharacterAttributes(p)
	writeCharacterAttributes(p, stats.Total)
	p.WriteInt32(stats.MaxHP)
	p.WriteInt16(clampInt16(stats.MaxMP))
	p.WriteInt32(chr.HP)
	p.WriteInt16(clampInt16(chr.MP))
	p.WriteInt16(clampInt16(stats.MinDamage))
	p.WriteInt16(clampInt16(stats.MaxDamage))
	p.WriteInt16(clampInt16(stats.MinMagicDamage))
	p.WriteInt16(clampInt16(stats.MaxMagicDamage))
	p.WriteInt16(clampInt16(stats.MagicDefence))
	p.WriteInt16(clampInt16(stats.DefenceMin))
	p.WriteInt16(clampInt16(stats.DefenceMax))
	p.WriteFloat32(float32(stats.BlockChance))
	p.WriteFloat32(float32(stats.BlockValue))
	p.WriteInt16(15)
	p.WriteInt16(7)
	p.WriteInt16(4)
	p.WriteBytes(make([]byte, 28))
	c.Send(p)
}

func sendSkillLearnedFirstTime(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	seconds := int16((skill.Cooldown + time.Second - time.Nanosecond) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	p := NewPacket(SkillLearnedFirstTime)
	p.WriteInt16(skill.ID)
	p.WriteUint8(1)
	p.WriteUint8(1)
	p.WriteInt16(seconds)
	p.WriteBytes(make([]byte, 2))
	p.WriteInt16(int16(defaultSkillEffectID))
	p.WriteInt32(28)
	p.WriteUint8(100)
	p.WriteUint8(100)
	p.WriteInt16(8)
	p.WriteBytes(skillLearnStab24)
	c.Send(p)
}

func sendSetSkillCooldown(c *Client, skill SkillTemplate) {
	if c == nil || c.Char == nil {
		return
	}
	seconds := int16((skill.Cooldown + time.Second - time.Nanosecond) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	p := NewPacket(SetSkillCooldown)
	p.WriteUint8(1)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skill.ID)
	p.WriteInt16(seconds)
	c.Send(p)
}

func sendSkillReady(c *Client, skillID int16) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(SkillReady)
	p.WriteInt16(skillID)
	c.Send(p)
}

func sendAnimateSkillStrike(c *Client, skill SkillTemplate, target *Monster, damage int32) {
	if c == nil || c.Char == nil || target == nil {
		return
	}
	p := NewPacket(AnimateSkillStrike)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(skill.ID)
	p.WriteInt16(6)
	p.WriteUint8(1) // target count
	p.WriteUint8(0) // 0 = NPC target, 1 = player target in the original C# packet
	p.WriteUint16(uint16(target.SessionID))
	p.WriteInt32(damage)
	p.WriteInt32(skillStrikeHitOK)
	p.WriteInt32(skill.EffectID)
	p.WriteBytes(skillStrikeStab15)
	c.SendToArea(p)
}

func sendCharacterMPUpdate(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(CharMpUpdate)
	p.WriteInt16(c.Char.SessionID)
	p.WriteInt16(clampInt16(c.Char.MaxMP))
	p.WriteInt16(clampInt16(c.Char.MP))
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteInt16(0)
	c.SendToArea(p)
}

func clampInt32(value int64) int32 {
	if value > 2147483647 {
		return 2147483647
	}
	if value < -2147483648 {
		return -2147483648
	}
	return int32(value)
}
