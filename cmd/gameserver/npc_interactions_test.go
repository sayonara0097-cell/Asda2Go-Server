package main

import (
	"testing"

	"asda2/shared/types"
)

func TestNpcInteractionKindDerivesTrainerFromTemplateName(t *testing.T) {
	npc := &Npc{
		EntryID: 12,
		Name:    "[Warrior Trainer] Gladio",
		Kind:    8,
	}

	if got := npcInteractionKind(npc); got != types.NpcInteractionTrainer {
		t.Fatalf("interaction kind = %d, want trainer", got)
	}
}

func TestClientCanInteractWithNpcUsesReferenceDistance(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	npc := &Npc{SessionID: 1001, EntryID: 12, MapID: 0, LocalX: 20, LocalY: 10, Channel: -1}

	if !clientCanInteractWithNpc(c, npc) {
		t.Fatal("nearby NPC should be interactable")
	}
	npc.LocalX = 26
	if clientCanInteractWithNpc(c, npc) {
		t.Fatal("NPC beyond the 15-unit reference interaction distance should not be interactable")
	}
}
