package types

import "testing"

func TestNormalizeNpcTemplateDerivesTrainerMetadata(t *testing.T) {
	row := NormalizeNpcTemplate(NpcTemplateRow{
		EntryID: 12,
		Name:    "[Warrior Trainer] Gladio",
		Kind:    8,
	})

	if !row.IsTrainer {
		t.Fatal("trainer name should mark template as trainer")
	}
	if row.InteractionKind != NpcInteractionTrainer {
		t.Fatalf("interaction kind = %d, want trainer", row.InteractionKind)
	}
	if row.ClassGroup != NpcClassGroupWarrior {
		t.Fatalf("class group = %d, want warrior", row.ClassGroup)
	}
}

func TestNormalizeNpcTemplateDerivesCommonNpcInteractions(t *testing.T) {
	cases := []struct {
		name string
		want NpcInteractionKind
	}{
		{name: "[General Shop] Ian", want: NpcInteractionVendor},
		{name: "Bulletin Board", want: NpcInteractionQuest},
		{name: "Village Resident", want: NpcInteractionDialogue},
	}

	for _, tc := range cases {
		got := NormalizeNpcTemplate(NpcTemplateRow{Name: tc.name}).InteractionKind
		if got != tc.want {
			t.Fatalf("%q interaction = %d, want %d", tc.name, got, tc.want)
		}
	}
}
