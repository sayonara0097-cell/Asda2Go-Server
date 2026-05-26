package main

import "asda2/shared/types"

func applyQuestClassReward(c *Client, questID int) bool {
	if c == nil || c.Char == nil {
		return false
	}
	realLevel, classID, ok := questClassReward(questID, c.Char.Class)
	if !ok {
		return false
	}
	return setCharacterClass(c, realLevel, classID)
}

func questClassReward(questID int, currentClass byte) (byte, byte, bool) {
	switch questID {
	case 2022:
		return 1, byte(types.Asda2ClassOHS), true
	case 2473:
		return 1, byte(types.Asda2ClassSpear), true
	case 2472:
		return 1, byte(types.Asda2ClassTHS), true
	case 2023:
		return 1, byte(types.Asda2ClassCrossbow), true
	case 2474:
		return 1, byte(types.Asda2ClassBow), true
	case 2024:
		return 1, byte(types.Asda2ClassAttackMage), true
	case 2475:
		return 1, byte(types.Asda2ClassSupportMage), true
	case 2476:
		return 1, byte(types.Asda2ClassHealMage), true
	case 2055:
		switch currentClass {
		case byte(types.Asda2ClassOHS):
			return 2, byte(types.Asda2ClassOHS), true
		case byte(types.Asda2ClassTHS):
			return 2, byte(types.Asda2ClassTHS), true
		case byte(types.Asda2ClassSpear):
			return 2, byte(types.Asda2ClassSpear), true
		}
	case 2057:
		switch currentClass {
		case byte(types.Asda2ClassBow):
			return 2, byte(types.Asda2ClassBow), true
		case byte(types.Asda2ClassCrossbow):
			return 2, byte(types.Asda2ClassCrossbow), true
		}
	case 2058:
		switch currentClass {
		case byte(types.Asda2ClassAttackMage):
			return 2, byte(types.Asda2ClassAttackMage), true
		case byte(types.Asda2ClassSupportMage):
			return 2, byte(types.Asda2ClassSupportMage), true
		case byte(types.Asda2ClassHealMage):
			return 2, byte(types.Asda2ClassHealMage), true
		}
	}
	return 0, 0, false
}
