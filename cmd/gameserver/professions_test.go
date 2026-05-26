package main

import (
	"testing"

	"asda2/shared/types"
)

func TestApplyCharacterClassEncodesProfessionAndAddsReferenceSkills(t *testing.T) {
	chr := &Character{
		Class:           byte(types.Asda2ClassBow),
		ProfessionLevel: types.EncodedProfessionLevel(byte(types.Asda2ClassBow), 1),
		LearnedSkills: map[int16]byte{
			1240: 1,
			2240: 1,
			701:  1,
		},
	}

	removed, added, ok := applyCharacterClass(chr, 2, byte(types.Asda2ClassAttackMage))
	if !ok {
		t.Fatalf("applyCharacterClass returned false")
	}
	if chr.Class != byte(types.Asda2ClassAttackMage) {
		t.Fatalf("class = %d, want AttackMage", chr.Class)
	}
	if chr.ProfessionLevel != types.EncodedProfessionLevel(byte(types.Asda2ClassAttackMage), 2) {
		t.Fatalf("profession level = %d, want encoded mage level 2", chr.ProfessionLevel)
	}
	if chr.LearnedSkills[1240] != 0 || chr.LearnedSkills[2240] != 0 {
		t.Fatalf("old bow profession skill was not removed")
	}
	if chr.LearnedSkills[1249] != 1 || chr.LearnedSkills[1250] != 1 {
		t.Fatalf("attack mage profession skills were not added: %#v", chr.LearnedSkills)
	}
	if len(removed) < 300 {
		t.Fatalf("removed %d profession skills, want full profession cleanup", len(removed))
	}
	if len(added) != 28 {
		t.Fatalf("added %d skills, want 2 profession skills plus 26 attack mage class skills", len(added))
	}
}

func TestApplyCharacterClassRemovesOtherProfessionSkills(t *testing.T) {
	chr := &Character{
		Class:           byte(types.Asda2ClassOHS),
		ProfessionLevel: types.EncodedProfessionLevel(byte(types.Asda2ClassOHS), 2),
		LearnedSkills: map[int16]byte{
			1228: 1,
			1229: 1,
			1240: 1,
			1252: 1,
			2228: 1,
		},
	}
	_, _, ok := applyCharacterClass(chr, 1, byte(types.Asda2ClassCrossbow))
	if !ok {
		t.Fatalf("applyCharacterClass returned false")
	}
	for _, stale := range []int16{1228, 1229, 1240, 1252, 2228} {
		if chr.LearnedSkills[stale] > 0 {
			t.Fatalf("stale profession skill %d was not removed", stale)
		}
	}
	if chr.LearnedSkills[1237] != 1 {
		t.Fatalf("new crossbow profession skill was not added")
	}
	if chr.LearnedSkills[701] != 1 || chr.LearnedSkills[740] != 1 {
		t.Fatalf("new crossbow class skills were not added")
	}
}

func TestProfessionSkillIDsForClassCapsAtReferenceSkillCount(t *testing.T) {
	got := professionSkillIDsForClass(byte(types.Asda2ClassOHS), 4)
	want := []int16{1228, 1229, 1230}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("skill[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestProfessionSkillsUseLegacyRealIDsForClientSkillList(t *testing.T) {
	tests := map[byte][]int16{
		byte(types.Asda2ClassOHS):         {1228, 1229, 1230},
		byte(types.Asda2ClassSpear):       {1231, 1232, 1233},
		byte(types.Asda2ClassTHS):         {1234, 1235, 1236},
		byte(types.Asda2ClassCrossbow):    {1237, 1238, 1239},
		byte(types.Asda2ClassBow):         {1240, 1241, 1242},
		byte(types.Asda2ClassBalista):     {1243, 1244, 1245},
		byte(types.Asda2ClassHealMage):    {1246, 1247, 1248},
		byte(types.Asda2ClassAttackMage):  {1249, 1250, 1251},
		byte(types.Asda2ClassSupportMage): {1252, 1253, 1254},
	}
	for classID, want := range tests {
		got := professionSkillIDsForClass(classID, 4)
		if len(got) != len(want) {
			t.Fatalf("class %d got %d skills, want %d", classID, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("class %d skill[%d] = %d, want %d", classID, i, got[i], want[i])
			}
		}
	}
}

func TestChangeProfessionReferencePayloadLength(t *testing.T) {
	got := changeProfessionStab6Len +
		4 + // account id
		1 + // constant
		2 + // constant
		2 + // constant
		1 + // constant
		1 + // profession level
		1 + // class
		1 + // skill points
		1 + // constant
		4 + // quest id
		2 + // constant
		4 + // constant
		changeProfessionStab31Len +
		4 + // money
		changeProfessionStab501Len
	if got != 515 {
		t.Fatalf("ChangeProfession payload length = %d, want 515", got)
	}
}
