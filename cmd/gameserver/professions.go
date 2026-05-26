package main

import (
	"log"
	"time"

	"asda2/shared/types"
)

func init() {
	for classID, skillIDs := range professionAutoSkillsByClass {
		classGroup := skillClassGroupForClassID(classID)
		for i, skillID := range skillIDs {
			if _, exists := skillTemplates[skillID]; exists {
				continue
			}
			skillTemplates[skillID] = SkillTemplate{
				ID:                      skillID,
				Level:                   1,
				ClassGroup:              classGroup,
				RequiredProfessionLevel: byte(i + 1),
				PowerCost:               5,
				Cooldown:                3 * time.Second,
				Damage:                  20,
				Range:                   professionSkillRange(classGroup),
				EffectID:                defaultSkillEffectID,
				Effect0Misc:             defaultSkillEffect0Misc,
				SoulGuardLevel:          byte(i + 1),
			}
		}
	}
}

func setCharacterClass(c *Client, realProfessionLevel, classID byte) bool {
	if c == nil || c.Char == nil {
		return false
	}
	removed, added, changed := applyCharacterClass(c.Char, realProfessionLevel, classID)
	if !changed {
		return false
	}
	removed = append(removed, allLegacyProfessionSkillIDs()...)
	removed = append(removed, allClassSkillIDs()...)
	if err := DeleteCharacterSkills(c.Char.GUID, removed); err != nil {
		log.Printf("[Profession] failed to remove old profession skills char=%d: %v", c.Char.GUID, err)
		return false
	}
	added = append(added, classSkillIDsForClass(classID)...)
	if !saveProfessionSkills(c.Char, added) {
		return false
	}
	if _, err := ApplyBaseStatsToCharacter(c.Char, true); err != nil {
		log.Printf("[Profession] failed to refresh base stats char=%d class=%d: %v", c.Char.GUID, c.Char.Class, err)
		return false
	}
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Profession] failed to save class char=%d class=%d professionLevel=%d: %v",
			c.Char.GUID, c.Char.Class, c.Char.ProfessionLevel, err)
		return false
	}
	sendChangeProfessionResponse(c)
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
	sendSkillsInfo(c)
	log.Printf("[Profession] %q class=%d realProfessionLevel=%d encodedProfessionLevel=%d",
		c.Char.Name, c.Char.Class, c.Char.RealProfessionLevel(), c.Char.ProfessionLevel)
	return true
}

func applyCharacterClass(chr *Character, realProfessionLevel, classID byte) ([]int16, []int16, bool) {
	if chr == nil || !types.IsAsda2Class(classID) || realProfessionLevel == 0 {
		return nil, nil, false
	}
	if realProfessionLevel > 4 {
		realProfessionLevel = 4
	}

	removed := allProfessionSkillIDs()
	removed = append(removed, allLegacyProfessionSkillIDs()...)
	removed = append(removed, allClassSkillIDs()...)
	if chr.LearnedSkills == nil {
		chr.LearnedSkills = make(map[int16]byte)
	}
	for _, skillID := range removed {
		delete(chr.LearnedSkills, skillID)
	}

	chr.Class = classID
	chr.ProfessionLevel = types.EncodedProfessionLevel(classID, realProfessionLevel)

	added := professionSkillIDsForClass(classID, realProfessionLevel)
	added = append(added, classSkillIDsForClass(classID)...)
	for _, skillID := range added {
		chr.LearnedSkills[skillID] = 1
	}
	return removed, added, true
}

func reconcileProfessionSkills(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	chr := c.Char
	realLevel := chr.RealProfessionLevel()
	if !types.IsAsda2Class(chr.Class) || realLevel == 0 {
		return
	}
	if chr.LearnedSkills == nil {
		chr.LearnedSkills = make(map[int16]byte)
	}
	expected := professionSkillIDsForClass(chr.Class, realLevel)
	expected = append(expected, classSkillIDsForClass(chr.Class)...)
	expectedSet := make(map[int16]struct{}, len(expected))
	for _, skillID := range expected {
		expectedSet[skillID] = struct{}{}
	}

	var remove []int16
	for _, skillID := range allProfessionSkillIDs() {
		if _, keep := expectedSet[skillID]; keep {
			continue
		}
		if chr.LearnedSkills[skillID] > 0 {
			delete(chr.LearnedSkills, skillID)
			remove = append(remove, skillID)
		}
	}
	for _, skillID := range allLegacyProfessionSkillIDs() {
		if chr.LearnedSkills[skillID] > 0 {
			delete(chr.LearnedSkills, skillID)
		}
		remove = append(remove, skillID)
	}
	for _, skillID := range allClassSkillIDs() {
		if _, keep := expectedSet[skillID]; keep {
			continue
		}
		if chr.LearnedSkills[skillID] > 0 {
			delete(chr.LearnedSkills, skillID)
			remove = append(remove, skillID)
		}
	}
	if len(remove) > 0 {
		if err := DeleteCharacterSkills(chr.GUID, remove); err != nil {
			log.Printf("[Profession] failed to reconcile removed skills char=%d: %v", chr.GUID, err)
		}
	}

	var add []int16
	for _, skillID := range expected {
		if chr.LearnedSkills[skillID] > 0 {
			continue
		}
		chr.LearnedSkills[skillID] = 1
		add = append(add, skillID)
	}
	if len(add) > 0 {
		saveProfessionSkills(chr, add)
	}
}

func saveProfessionSkills(chr *Character, skillIDs []int16) bool {
	for _, skillID := range skillIDs {
		if err := SaveCharacterSkill(chr.GUID, skillID, 1); err != nil {
			log.Printf("[Profession] failed to save profession skill char=%d skill=%d: %v", chr.GUID, skillID, err)
			return false
		}
	}
	return true
}

func allLegacyProfessionSkillIDs() []int16 {
	out := make([]int16, 0, 27)
	for classID := byte(types.Asda2ClassOHS); classID <= byte(types.Asda2ClassHealMage); classID++ {
		out = append(out, legacyProfessionAutoSkillsByClass[classID]...)
	}
	return out
}

func autoAdvanceProfessionForLevel(c *Client) {
	if c == nil || c.Char == nil || c.Char.Class == byte(types.Asda2ClassNone) {
		return
	}
	realLevel := c.Char.RealProfessionLevel()
	switch {
	case c.Char.Level >= 70 && realLevel == 3:
		setCharacterClass(c, 4, c.Char.Class)
	case c.Char.Level >= 50 && realLevel == 2:
		setCharacterClass(c, 3, c.Char.Class)
	case c.Char.Level >= 30 && realLevel == 1:
		setCharacterClass(c, 2, c.Char.Class)
	}
}

func professionSkillIDsForClass(classID, realProfessionLevel byte) []int16 {
	skillIDs := legacyProfessionAutoSkillsByClass[classID]
	if realProfessionLevel == 3 {
		skillIDs = professionAutoSkillsByClass[classID]
	}
	if len(skillIDs) == 0 || realProfessionLevel == 0 {
		return nil
	}
	count := int(realProfessionLevel)
	if count > len(skillIDs) {
		count = len(skillIDs)
	}
	out := make([]int16, count)
	copy(out, skillIDs[:count])
	return out
}

func allProfessionSkillIDs() []int16 {
	out := make([]int16, 0, 27)
	for classID := byte(types.Asda2ClassOHS); classID <= byte(types.Asda2ClassHealMage); classID++ {
		out = append(out, professionAutoSkillsByClass[classID]...)
	}
	return out
}

func allProfessionSkillIDsForFamily(family types.Asda2ProfessionFamily) []int16 {
	var out []int16
	for classID := byte(types.Asda2ClassOHS); classID <= byte(types.Asda2ClassHealMage); classID++ {
		skillIDs := professionAutoSkillsByClass[classID]
		if types.Asda2ClassFamily(classID) == family {
			out = append(out, skillIDs...)
		}
	}
	return out
}

func skillClassGroupForClassID(classID byte) byte {
	switch types.Asda2ClassFamily(classID) {
	case types.Asda2ProfessionWarrior:
		return skillClassWarrior
	case types.Asda2ProfessionArcher:
		return skillClassArcher
	default:
		return skillClassMage
	}
}

func professionSkillRange(classGroup byte) float64 {
	if classGroup == skillClassWarrior {
		return 3.2
	}
	return 8.0
}

func sendChangeProfessionResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	chr := c.Char
	p := NewPacket(ChangeProfession)
	p.WriteBytes(make([]byte, changeProfessionStab6Len))
	p.WriteInt32(int32(chr.AccID))
	p.WriteUint8(2)
	p.WriteInt16(32)
	p.WriteInt16(0)
	p.WriteUint8(3)
	p.WriteUint8(chr.ProfessionLevel)
	p.WriteUint8(chr.Class)
	p.WriteUint8(byte(chr.AvailableSkillPoints()))
	p.WriteUint8(0)
	p.WriteInt32(2475)
	p.WriteInt16(260)
	p.WriteInt32(0)
	p.WriteBytes(make([]byte, changeProfessionStab31Len))
	p.WriteInt32(clampInt32(chr.Gold))
	p.WriteBytes(make([]byte, changeProfessionStab501Len))
	c.Send(p)
}

const (
	// Names mirror WCell.RealmServer/Handlers/Asda2CharacterHandler.cs.
	// The numeric suffixes are legacy names, not lengths.
	changeProfessionStab6Len   = 1
	changeProfessionStab31Len  = 466
	changeProfessionStab501Len = 20
)
