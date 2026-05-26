package main

import "testing"

func TestRefreshCharacterVisibilityPairsSameChannelNearbyPlayers(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	a := testVisibilityClient(1, 101, 0, 0, 10, 10)
	b := testVisibilityClient(2, 102, 0, 0, 20, 20)

	gm.characters[a.ID] = a
	gm.characters[b.ID] = b

	actions := gm.refreshCharacterVisibility(a)
	if len(actions) != 2 {
		t.Fatalf("visibility actions = %d, want 2", len(actions))
	}
	if !gm.characterKnownLocked(a.ID, b.ID) {
		t.Fatal("player A should know player B")
	}
	if !gm.characterKnownLocked(b.ID, a.ID) {
		t.Fatal("player B should know player A")
	}

	recipients := gm.AreaRecipients(a, true)
	if len(recipients) != 2 {
		t.Fatalf("area recipients = %d, want self plus one viewer", len(recipients))
	}
}

func TestRefreshCharacterVisibilitySkipsDifferentChannels(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	a := testVisibilityClient(1, 101, 0, 0, 10, 10)
	b := testVisibilityClient(2, 102, 1, 0, 20, 20)

	gm.characters[a.ID] = a
	gm.characters[b.ID] = b

	actions := gm.refreshCharacterVisibility(a)
	if len(actions) != 0 {
		t.Fatalf("visibility actions = %d, want 0", len(actions))
	}
	if gm.characterKnownLocked(a.ID, b.ID) || gm.characterKnownLocked(b.ID, a.ID) {
		t.Fatal("players on different channels should not know each other")
	}
}

func TestRefreshCharacterVisibilityRemovesOutOfRangePlayers(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	a := testVisibilityClient(1, 101, 0, 0, 10, 10)
	b := testVisibilityClient(2, 102, 0, 0, 20, 20)

	gm.characters[a.ID] = a
	gm.characters[b.ID] = b

	gm.refreshCharacterVisibility(a)
	b.Char.X = 100
	b.Char.Y = 100

	actions := gm.refreshCharacterVisibility(a)
	if len(actions) != 2 {
		t.Fatalf("visibility removal actions = %d, want 2", len(actions))
	}
	for _, action := range actions {
		if !action.remove {
			t.Fatal("expected only remove actions after moving out of range")
		}
	}
	if gm.characterKnownLocked(a.ID, b.ID) || gm.characterKnownLocked(b.ID, a.ID) {
		t.Fatal("out-of-range players should be removed from known character sets")
	}
}

func TestRefreshMonsterVisibilityResendsWhenPlayerReturnsToRange(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	monster := &Monster{
		SessionID: 20101,
		EntryID:   65,
		MapID:     0,
		LocalX:    20,
		LocalY:    20,
		Health:    100,
		State:     MonsterStateOK,
	}

	gm.characters[c.ID] = c
	gm.monsters[monster.SessionID] = monster

	visible := gm.refreshMonsterVisibility(c, false)
	if len(visible) != 1 || visible[0] != monster {
		t.Fatalf("monster visibility = %d, want the nearby monster", len(visible))
	}
	if !gm.monsterKnownLocked(c.ID, monster.SessionID) {
		t.Fatal("player should know nearby monster after first refresh")
	}

	visible = gm.refreshMonsterVisibility(c, false)
	if len(visible) != 0 {
		t.Fatalf("second monster visibility = %d, want no duplicate sends", len(visible))
	}

	visible = gm.refreshMonsterVisibility(c, true)
	if len(visible) != 1 || visible[0] != monster {
		t.Fatalf("forced monster visibility = %d, want resend even when known", len(visible))
	}

	c.Char.X = 200
	c.Char.Y = 200
	visible = gm.refreshMonsterVisibility(c, false)
	if len(visible) != 0 {
		t.Fatalf("out-of-range monster visibility = %d, want no sends", len(visible))
	}
	if gm.monsterKnownLocked(c.ID, monster.SessionID) {
		t.Fatal("out-of-range monster should be removed from known monster set")
	}

	c.Char.X = 10
	c.Char.Y = 10
	visible = gm.refreshMonsterVisibility(c, false)
	if len(visible) != 1 || visible[0] != monster {
		t.Fatalf("returning monster visibility = %d, want resend", len(visible))
	}
}

func TestRefreshNpcVisibilityMatchesMonsterKnownSetBehavior(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	npc := &Npc{
		SessionID:     1001,
		WorldEntityID: 200001,
		SpawnID:       1,
		EntryID:       3,
		MapID:         0,
		LocalX:        20,
		LocalY:        20,
		Channel:       -1,
	}

	gm.characters[c.ID] = c
	gm.npcs[npc.SessionID] = npc

	visible := gm.refreshNpcVisibility(c, false)
	if len(visible) != 1 || visible[0] != npc {
		t.Fatalf("npc visibility = %d, want nearby npc", len(visible))
	}

	visible = gm.refreshNpcVisibility(c, false)
	if len(visible) != 0 {
		t.Fatalf("second npc visibility = %d, want no duplicate sends", len(visible))
	}

	visible = gm.refreshNpcVisibility(c, true)
	if len(visible) != 1 || visible[0] != npc {
		t.Fatalf("forced npc visibility = %d, want resend even when known", len(visible))
	}

	c.Char.X = 500
	c.Char.Y = 500
	visible = gm.refreshNpcVisibility(c, false)
	if len(visible) != 0 {
		t.Fatalf("out-of-range npc visibility = %d, want no sends", len(visible))
	}
	if _, ok := gm.ensureKnownNpcSetLocked(c.ID)[npc.SessionID]; ok {
		t.Fatal("out-of-range npc should be removed from known npc set")
	}

	c.Char.X = 10
	c.Char.Y = 10
	visible = gm.refreshNpcVisibility(c, false)
	if len(visible) != 1 || visible[0] != npc {
		t.Fatalf("returning npc visibility = %d, want resend", len(visible))
	}
}

func TestMonsterKnownViewersOnlyIncludesKnownAndInRangePlayers(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	known := testVisibilityClient(1, 101, 0, 0, 10, 10)
	unknown := testVisibilityClient(2, 102, 0, 0, 12, 12)
	far := testVisibilityClient(3, 103, 0, 0, 500, 500)
	monster := &Monster{
		SessionID: 20101,
		EntryID:   65,
		MapID:     0,
		LocalX:    20,
		LocalY:    20,
		Health:    100,
		State:     MonsterStateOK,
	}

	gm.characters[known.ID] = known
	gm.characters[unknown.ID] = unknown
	gm.characters[far.ID] = far
	gm.monsters[monster.SessionID] = monster
	gm.ensureKnownMonsterSetLocked(known.ID)[monster.SessionID] = struct{}{}
	gm.ensureKnownMonsterSetLocked(far.ID)[monster.SessionID] = struct{}{}

	viewers := gm.monsterKnownViewers(monster)
	if len(viewers) != 1 || viewers[0] != known {
		t.Fatalf("known viewers = %#v, want only the in-range known player", viewers)
	}
}

func TestRefreshMonsterVisibilityAroundAddsNewViewersAndDropsFarKnown(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	near := testVisibilityClient(1, 101, 0, 0, 10, 10)
	far := testVisibilityClient(2, 102, 0, 0, 500, 500)
	monster := &Monster{
		SessionID: 20101,
		EntryID:   65,
		MapID:     0,
		LocalX:    20,
		LocalY:    20,
		Health:    100,
		State:     MonsterStateOK,
	}

	gm.characters[near.ID] = near
	gm.characters[far.ID] = far
	gm.monsters[monster.SessionID] = monster
	gm.ensureKnownMonsterSetLocked(far.ID)[monster.SessionID] = struct{}{}

	newlyVisible := gm.refreshMonsterVisibilityAround(monster)
	if len(newlyVisible) != 1 || newlyVisible[0] != near {
		t.Fatalf("newly visible = %#v, want only near player", newlyVisible)
	}
	if !gm.monsterKnownLocked(near.ID, monster.SessionID) {
		t.Fatal("near player should become aware of monster")
	}
	if gm.monsterKnownLocked(far.ID, monster.SessionID) {
		t.Fatal("far known player should be dropped from known monster set")
	}
}

func TestMonsterDamageRecipientsIncludeCharacterAndMonsterViewers(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	attacker := testVisibilityClient(1, 101, 0, 0, 100, 100)
	charViewer := testVisibilityClient(2, 102, 0, 0, 102, 100)
	monsterViewer := testVisibilityClient(3, 103, 0, 0, 130, 100)
	monster := &Monster{
		SessionID: 20020,
		EntryID:   589,
		MapID:     0,
		LocalX:    130,
		LocalY:    100,
		Health:    100,
		State:     MonsterStateOK,
	}

	gm.characters[attacker.ID] = attacker
	gm.characters[charViewer.ID] = charViewer
	gm.characters[monsterViewer.ID] = monsterViewer
	gm.monsters[monster.SessionID] = monster
	gm.ensureKnownSetLocked(attacker.ID)[charViewer.ID] = struct{}{}
	gm.ensureKnownMonsterSetLocked(monsterViewer.ID)[monster.SessionID] = struct{}{}

	recipients := gm.monsterDamageRecipients(attacker, monster)
	if len(recipients) != 3 {
		t.Fatalf("recipient count=%d, want 3", len(recipients))
	}
	got := map[uint32]bool{}
	for _, c := range recipients {
		got[c.ID] = true
	}
	for _, c := range []*Client{attacker, charViewer, monsterViewer} {
		if !got[c.ID] {
			t.Fatalf("missing recipient %s", clientDebugLabel(c))
		}
	}
}

func testVisibilityClient(id uint32, sessionID int16, channel byte, mapID uint16, x float32, y float32) *Client {
	return &Client{
		ID:      id,
		Channel: channel,
		Char: &Character{
			SessionID: sessionID,
			AccID:     id + 1000,
			Name:      "test",
			MapID:     mapID,
			X:         x,
			Y:         y,
		},
	}
}
