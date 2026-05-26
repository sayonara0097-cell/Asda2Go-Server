package main

import (
	"testing"

	"asda2/shared/types"
)

func TestQuestClassRewardFirstProfessionQuests(t *testing.T) {
	tests := map[int]byte{
		2022: byte(types.Asda2ClassOHS),
		2473: byte(types.Asda2ClassSpear),
		2472: byte(types.Asda2ClassTHS),
		2023: byte(types.Asda2ClassCrossbow),
		2474: byte(types.Asda2ClassBow),
		2024: byte(types.Asda2ClassAttackMage),
		2475: byte(types.Asda2ClassSupportMage),
		2476: byte(types.Asda2ClassHealMage),
	}
	for questID, wantClass := range tests {
		realLevel, classID, ok := questClassReward(questID, 0)
		if !ok {
			t.Fatalf("quest %d did not return a class reward", questID)
		}
		if realLevel != 1 || classID != wantClass {
			t.Fatalf("quest %d reward = level %d class %d, want level 1 class %d", questID, realLevel, classID, wantClass)
		}
	}
}

func TestQuestClassRewardSecondProfessionKeepsCurrentBranch(t *testing.T) {
	realLevel, classID, ok := questClassReward(2058, byte(types.Asda2ClassHealMage))
	if !ok {
		t.Fatalf("quest 2058 did not return a reward for HealMage")
	}
	if realLevel != 2 || classID != byte(types.Asda2ClassHealMage) {
		t.Fatalf("quest 2058 reward = level %d class %d, want level 2 HealMage", realLevel, classID)
	}
	if _, _, ok := questClassReward(2057, byte(types.Asda2ClassBalista)); ok {
		t.Fatalf("quest 2057 should not reward Balista in the reference mapping")
	}
}
