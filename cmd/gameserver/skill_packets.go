package main

import "encoding/binary"

type learnSkillRequest struct {
	skillID int16
	level   byte
	offset  int
}

func readLearnSkillRequest(chr *Character, data []byte) (learnSkillRequest, bool) {
	best := learnSkillRequest{}
	bestScore := -1 << 30
	for offset := 0; offset+2 <= len(data); offset++ {
		skillID := int16(binary.LittleEndian.Uint16(data[offset:]))
		skill, ok := skillTemplates[skillID]
		if !ok {
			continue
		}

		level, score := readLearnSkillLevel(data, offset, skill)
		score += learnSkillOffsetScore(offset)
		if skillAvailableForCharacter(chr, skill) {
			score += 20
		} else {
			score -= 10
		}

		if score > bestScore {
			best = learnSkillRequest{skillID: skillID, level: level, offset: offset}
			bestScore = score
		}
	}
	return best, bestScore > -20
}

func readLearnSkillLevel(data []byte, offset int, skill SkillTemplate) (byte, int) {
	if offset+2 >= len(data) {
		return 1, 2
	}
	rawLevel := data[offset+2]
	switch {
	case rawLevel == 0:
		return 1, -2
	case rawLevel == skill.Level:
		return rawLevel, 10
	case rawLevel <= 10:
		return rawLevel, 4
	default:
		return 1, -6
	}
}

func learnSkillOffsetScore(offset int) int {
	switch offset {
	case 0:
		return 6
	case 28:
		return 12
	case 20, 24, 26, 30, 32, 34, 36:
		return 6
	default:
		if offset >= 20 && offset <= 40 {
			return 3
		}
		return 0
	}
}
