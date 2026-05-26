package main

import (
	"encoding/binary"
	"testing"

	"asda2/shared/types"
)

func TestDefaultSkillsAreAvailableForEveryClass(t *testing.T) {
	classes := []byte{
		byte(types.Asda2ClassOHS),
		byte(types.Asda2ClassBow),
		byte(types.Asda2ClassAttackMage),
	}
	for _, classID := range classes {
		chr := &Character{Class: classID}
		for _, skillID := range defaultSkillIDs {
			if !skillAvailableForCharacter(chr, skillTemplates[skillID]) {
				t.Fatalf("default skill %d should be available for class %d", skillID, classID)
			}
		}
	}
}

func TestSkillIDsForCharacterStartWithDefaultSkills(t *testing.T) {
	got := skillIDsForCharacter(&Character{Class: byte(types.Asda2ClassTHS)})
	if len(got) < len(defaultSkillIDs) {
		t.Fatalf("skill ids length = %d, want at least %d", len(got), len(defaultSkillIDs))
	}
	for i, skillID := range defaultSkillIDs {
		if got[i] != skillID {
			t.Fatalf("skill id at %d = %d, want default skill %d", i, got[i], skillID)
		}
	}
}

func TestDefaultSkillsUseReferenceRealIDs(t *testing.T) {
	want := []int16{1071, 1072}
	if len(defaultSkillIDs) != len(want) {
		t.Fatalf("default skill count = %d, want %d", len(defaultSkillIDs), len(want))
	}
	for i, skillID := range want {
		if defaultSkillIDs[i] != skillID {
			t.Fatalf("default skill id at %d = %d, want real id %d", i, defaultSkillIDs[i], skillID)
		}
		if _, ok := skillTemplates[skillID]; !ok {
			t.Fatalf("default skill template %d is missing", skillID)
		}
	}
}

func TestClassSkillIDsCoverOHSFistTree(t *testing.T) {
	got := classSkillIDsForClass(byte(types.Asda2ClassOHS))
	if len(got) != 47 {
		t.Fatalf("OHS skill count = %d, want 47", len(got))
	}
	if got[0] != 501 || got[len(got)-1] != 547 {
		t.Fatalf("OHS skill range = %d..%d, want 501..547", got[0], got[len(got)-1])
	}
	if _, ok := skillTemplates[547]; !ok {
		t.Fatalf("OHS final skill template 547 is missing")
	}
}

func TestProfessionSoulGuardUsesReferenceActiveIDs(t *testing.T) {
	got := professionSkillIDsForClass(byte(types.Asda2ClassOHS), 3)
	want := []int16{2228, 2229, 2230}
	if len(got) != len(want) {
		t.Fatalf("soulguard skills = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("soulguard skill at %d = %d, want %d", i, got[i], want[i])
		}
	}
	if normalizeSoulGuardSkillID(1228) != 2228 {
		t.Fatal("legacy soulguard real id 1228 should normalize to active id 2228")
	}
}

func TestSoulGuardSkillMatchesClassAndTier(t *testing.T) {
	chr := &Character{Class: byte(types.Asda2ClassSupportMage)}
	if !soulGuardSkillMatchesCharacter(chr, 2253, 2) {
		t.Fatal("support mage tier 2 soulguard should match skill 2253")
	}
	if soulGuardSkillMatchesCharacter(chr, 2250, 2) {
		t.Fatal("support mage should not match attack mage soulguard skill")
	}
}

func TestSkillIDsForCharacterIncludeClassTreeWithoutDuplicates(t *testing.T) {
	got := skillIDsForCharacter(&Character{Class: byte(types.Asda2ClassOHS)})
	seen := map[int16]bool{}
	for _, skillID := range got {
		if seen[skillID] {
			t.Fatalf("duplicate skill id %d", skillID)
		}
		seen[skillID] = true
	}
	for _, skillID := range []int16{501, 517, 547} {
		if !seen[skillID] {
			t.Fatalf("OHS skill list missing %d", skillID)
		}
	}
}

func TestTrainerCanTeachOnlyMatchingClassSkills(t *testing.T) {
	trainer := &Npc{
		IsTrainer:       true,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupWarrior,
	}
	if !trainerCanTeachSkill(trainer, skillTemplates[501]) {
		t.Fatal("warrior trainer should teach warrior skills")
	}
	if trainerCanTeachSkill(trainer, skillTemplates[701]) {
		t.Fatal("warrior trainer should not teach archer skills")
	}
}

func TestTrainerLearnStatusInfersNearbyMatchingTrainer(t *testing.T) {
	c := testVisibilityClient(90, 190, 0, 0, 10, 10)
	c.Char.Class = byte(types.Asda2ClassOHS)
	trainer := &Npc{
		SessionID:       1001,
		EntryID:         11,
		MapID:           0,
		LocalX:          10,
		LocalY:          10,
		Channel:         -1,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupWarrior,
	}
	withTestNpcWorld(t, trainer)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := trainerLearnStatus(c, 501); got != skillLearnOK {
		t.Fatalf("trainer learn status = %d, want ok", got)
	}
	state, ok := currentNpcInteraction(c)
	if !ok || state.EntryID != trainer.EntryID || state.Kind != types.NpcInteractionTrainer {
		t.Fatalf("inferred trainer state = %#v, ok=%t", state, ok)
	}
}

func TestTrainerLearnStatusPrefersMatchingTrainerOverCloserWrongTrainer(t *testing.T) {
	c := testVisibilityClient(91, 191, 0, 0, 10, 10)
	c.Char.Class = byte(types.Asda2ClassOHS)
	wrongTrainer := &Npc{
		SessionID:       1001,
		EntryID:         12,
		MapID:           0,
		LocalX:          10,
		LocalY:          10,
		Channel:         -1,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupArcher,
	}
	matchingTrainer := &Npc{
		SessionID:       1002,
		EntryID:         13,
		MapID:           0,
		LocalX:          12,
		LocalY:          10,
		Channel:         -1,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupWarrior,
	}
	withTestNpcWorld(t, wrongTrainer, matchingTrainer)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := trainerLearnStatus(c, 501); got != skillLearnOK {
		t.Fatalf("trainer learn status = %d, want ok", got)
	}
	state, ok := currentNpcInteraction(c)
	if !ok || state.EntryID != matchingTrainer.EntryID {
		t.Fatalf("inferred trainer entry = %d, ok=%t, want %d", state.EntryID, ok, matchingTrainer.EntryID)
	}
}

func TestTrainerLearnStatusRejectsWhenNoTrainerIsNearby(t *testing.T) {
	c := testVisibilityClient(92, 192, 0, 0, 10, 10)
	c.Char.Class = byte(types.Asda2ClassOHS)
	trainer := &Npc{
		SessionID:       1001,
		EntryID:         11,
		MapID:           0,
		LocalX:          100,
		LocalY:          100,
		Channel:         -1,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupWarrior,
	}
	withTestNpcWorld(t, trainer)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := trainerLearnStatus(c, 501); got != skillLearnFail {
		t.Fatalf("trainer learn status = %d, want fail", got)
	}
}

func TestTrainerLearnStatusRejectsExplicitWrongClassTrainer(t *testing.T) {
	c := testVisibilityClient(93, 193, 0, 0, 10, 10)
	c.Char.Class = byte(types.Asda2ClassOHS)
	trainer := &Npc{
		SessionID:       1001,
		EntryID:         12,
		MapID:           0,
		LocalX:          10,
		LocalY:          10,
		Channel:         -1,
		InteractionKind: types.NpcInteractionTrainer,
		ClassGroup:      types.NpcClassGroupArcher,
	}
	withTestNpcWorld(t, trainer)
	rememberNpcInteraction(c, trainer, uint16(trainer.SessionID), types.NpcInteractionTrainer)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := trainerLearnStatus(c, 501); got != skillLearnBadProfession {
		t.Fatalf("trainer learn status = %d, want bad profession", got)
	}
}

func TestLearnRuntimeSkillReturnsReferenceMoneyStatus(t *testing.T) {
	old := skillTemplates[501]
	skill := old
	skill.MoneyCost = 100
	skillTemplates[501] = skill
	t.Cleanup(func() { skillTemplates[501] = old })

	c := &Client{Char: &Character{
		GUID:          100,
		Name:          "learner",
		Class:         byte(types.Asda2ClassOHS),
		Level:         50,
		Gold:          99,
		LearnedSkills: map[int16]byte{},
	}}

	if _, got := learnRuntimeSkill(c, 501, 1); got != skillLearnNotEnoughMoney {
		t.Fatalf("learn status = %d, want not enough money", got)
	}
}

func TestReadLearnSkillRequestSupportsLegacyPayload(t *testing.T) {
	raw := []byte{0xF5, 0x01, 0x01}

	req, ok := readLearnSkillRequest(&Character{Class: byte(types.Asda2ClassOHS)}, raw)
	if !ok {
		t.Fatal("learn skill request was not parsed")
	}
	if req.skillID != 501 || req.level != 1 || req.offset != 0 {
		t.Fatalf("request = %#v, want skill 501 level 1 offset 0", req)
	}
}

func TestReadLearnSkillRequestDetectsClientHeaderBeforeSkill(t *testing.T) {
	raw := make([]byte, 40)
	binary.LittleEndian.PutUint16(raw[0:], 27362)
	binary.LittleEndian.PutUint16(raw[28:], 501)
	raw[30] = 1

	req, ok := readLearnSkillRequest(&Character{Class: byte(types.Asda2ClassOHS)}, raw)
	if !ok {
		t.Fatal("learn skill request was not parsed")
	}
	if req.skillID != 501 || req.level != 1 || req.offset != 28 {
		t.Fatalf("request = %#v, want skill 501 level 1 offset 28", req)
	}
}

func TestReadLearnSkillRequestPrefersCharacterSkillOverWrongClassNoise(t *testing.T) {
	raw := make([]byte, 40)
	binary.LittleEndian.PutUint16(raw[20:], 701)
	raw[22] = 1
	binary.LittleEndian.PutUint16(raw[28:], 501)
	raw[30] = 1

	req, ok := readLearnSkillRequest(&Character{Class: byte(types.Asda2ClassOHS)}, raw)
	if !ok {
		t.Fatal("learn skill request was not parsed")
	}
	if req.skillID != 501 || req.offset != 28 {
		t.Fatalf("request = %#v, want warrior skill at offset 28", req)
	}
}

func TestReadLearnSkillRequestRejectsUnknownPayload(t *testing.T) {
	raw := []byte{0xE2, 0x6A, 0x01, 0x00}

	if req, ok := readLearnSkillRequest(&Character{Class: byte(types.Asda2ClassOHS)}, raw); ok {
		t.Fatalf("request = %#v, want parse failure", req)
	}
}

func TestSkillTemplatesFromDBRowsUseReferenceClassMaskAndRanks(t *testing.T) {
	withDBSkillTemplates(t, []Asda2SkillTemplateRow{
		{ID: 1507, RealID: 507, Level: 1, LearnLevel: 10, RequiredProfessionLevel: 1, ClassMask: 8, PowerCost: 12},
		{ID: 2507, RealID: 507, Level: 2, LearnLevel: 12, RequiredProfessionLevel: 1, ClassMask: 8, PowerCost: 14},
	})

	skill := skillTemplates[507]
	if skill.Level != 2 {
		t.Fatalf("skill max level = %d, want 2", skill.Level)
	}
	if !skillAvailableForCharacter(&Character{Class: byte(types.Asda2ClassTHS)}, skill) {
		t.Fatal("THS character should be allowed by ClassMask=8")
	}
	if skillAvailableForCharacter(&Character{Class: byte(types.Asda2ClassOHS)}, skill) {
		t.Fatal("OHS character should not be allowed by ClassMask=8")
	}
	if rank, ok := skillTemplateAtLevel(skill, 2); !ok || rank.RequiredLevel != 12 || rank.PowerCost != 14 {
		t.Fatalf("rank 2 = %#v, ok=%t, want reference rank data", rank, ok)
	}
}

func TestSkillTemplatesFromDBRowsPreserveEffectMetadata(t *testing.T) {
	withDBSkillTemplates(t, []Asda2SkillTemplateRow{
		{
			ID: 1507, RealID: 507, Level: 1, LearnLevel: 10, RequiredProfessionLevel: 1,
			ClassMask: 8, PowerCost: 12, Effect0Type: spellEffectApplyAura,
			Effect0BasePoints: 15, EffectID: 777, DurationMillis: 30000,
		},
	})

	skill := skillTemplates[507]
	if skill.PrimaryEffectKind() != skillEffectBuff {
		t.Fatalf("effect kind = %d, want buff", skill.PrimaryEffectKind())
	}
	if skill.Effect0BasePoints != 15 || skill.EffectID != 777 || skill.Duration != 30_000_000_000 {
		t.Fatalf("skill effect metadata = %#v, want DB values", skill)
	}
}

func TestLearnRuntimeSkillUpgradesSequentialReferenceRanks(t *testing.T) {
	withDBSkillTemplates(t, []Asda2SkillTemplateRow{
		{ID: 1507, RealID: 507, Level: 1, LearnLevel: 10, RequiredProfessionLevel: 1, ClassMask: 8},
		{ID: 2507, RealID: 507, Level: 2, LearnLevel: 12, RequiredProfessionLevel: 1, ClassMask: 8},
	})
	withSkillPersistenceStubs(t)
	c := &Client{Char: &Character{
		GUID:            100,
		Name:            "learner",
		Class:           byte(types.Asda2ClassTHS),
		Level:           20,
		ProfessionLevel: 1,
		LearnedSkills:   map[int16]byte{507: 1},
	}}

	skill, status := learnRuntimeSkill(c, 507, 2)
	if status != skillLearnOK {
		t.Fatalf("learn status = %d, want ok", status)
	}
	if skill.ID != 507 || skill.Level != 2 {
		t.Fatalf("learned skill = %#v, want skill 507 level 2", skill)
	}
	if got := c.Char.LearnedSkills[507]; got != 2 {
		t.Fatalf("stored skill level = %d, want 2", got)
	}
}

func TestLearnRuntimeSkillRejectsSkippedReferenceRank(t *testing.T) {
	withDBSkillTemplates(t, []Asda2SkillTemplateRow{
		{ID: 1507, RealID: 507, Level: 1, LearnLevel: 10, RequiredProfessionLevel: 1, ClassMask: 8},
		{ID: 2507, RealID: 507, Level: 2, LearnLevel: 12, RequiredProfessionLevel: 1, ClassMask: 8},
	})
	withSkillPersistenceStubs(t)
	c := &Client{Char: &Character{
		GUID:            100,
		Name:            "learner",
		Class:           byte(types.Asda2ClassTHS),
		Level:           20,
		ProfessionLevel: 1,
		LearnedSkills:   map[int16]byte{},
	}}

	if _, status := learnRuntimeSkill(c, 507, 2); status != skillLearnBadSpellLevel {
		t.Fatalf("learn status = %d, want bad spell level", status)
	}
}

func TestLearnRuntimeSkillUsesReferenceRankRequirements(t *testing.T) {
	withDBSkillTemplates(t, []Asda2SkillTemplateRow{
		{ID: 1507, RealID: 507, Level: 1, LearnLevel: 10, RequiredProfessionLevel: 1, ClassMask: 8},
		{ID: 2507, RealID: 507, Level: 2, LearnLevel: 12, RequiredProfessionLevel: 1, ClassMask: 8},
	})
	withSkillPersistenceStubs(t)
	c := &Client{Char: &Character{
		GUID:            100,
		Name:            "learner",
		Class:           byte(types.Asda2ClassTHS),
		Level:           11,
		ProfessionLevel: 1,
		LearnedSkills:   map[int16]byte{507: 1},
	}}

	if _, status := learnRuntimeSkill(c, 507, 2); status != skillLearnLowLevel {
		t.Fatalf("learn status = %d, want low level", status)
	}
}

func withDBSkillTemplates(t *testing.T, rows []Asda2SkillTemplateRow) {
	t.Helper()
	oldTemplates := skillTemplates
	oldSource := skillRuntimeSource
	setSkillTemplatesFromDBRows(rows)
	skillRuntimeSource = "SkillTemplate"
	t.Cleanup(func() {
		skillTemplates = oldTemplates
		skillRuntimeSource = oldSource
	})
}

func withSkillPersistenceStubs(t *testing.T) {
	t.Helper()
	oldSaveCharacterSkill := SaveCharacterSkill
	oldSaveCharacter := SaveCharacter
	SaveCharacterSkill = func(uint32, int16, byte) error { return nil }
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() {
		SaveCharacterSkill = oldSaveCharacterSkill
		SaveCharacter = oldSaveCharacter
	})
}
