package main

import "testing"

func TestPartyRuntimeAcceptInviteCreatesParty(t *testing.T) {
	m := &partyManager{
		nextID:      1,
		parties:     make(map[uint32]*socialParty),
		invites:     make(map[uint32]pendingPartyInvite),
		charToParty: make(map[uint32]uint32),
	}
	leader := testSocialClient(1, 1001, "Leader")
	member := testSocialClient(2, 1002, "Member")

	party := m.acceptInvite(leader, member, partyInviteTypeFirstPickup)
	if party == nil {
		t.Fatal("expected party")
	}
	if leader.Char.PartyID == 0 || leader.Char.PartyID != member.Char.PartyID {
		t.Fatalf("party ids leader=%d member=%d", leader.Char.PartyID, member.Char.PartyID)
	}
	if len(party.members()) != 2 {
		t.Fatalf("members=%d", len(party.members()))
	}
}

func TestPartyRuntimeAcceptInviteAddsLaterMembers(t *testing.T) {
	m := newTestPartyManager()
	leader := testSocialClient(1, 1001, "Leader")
	first := testSocialClient(2, 1002, "First")
	second := testSocialClient(3, 1003, "Second")
	enablePartyRequests(leader, first, second)

	party := m.acceptInvite(leader, first, partyInviteTypeFirstPickup)
	if party == nil {
		t.Fatal("expected first party")
	}
	party = m.acceptInvite(leader, second, partyInviteTypeFirstPickup)
	if party == nil {
		t.Fatal("expected second member to join existing party")
	}
	if len(party.members()) != 3 {
		t.Fatalf("members=%d", len(party.members()))
	}
	if second.Char.PartyID == 0 || second.Char.PartyID != leader.Char.PartyID {
		t.Fatalf("party ids leader=%d second=%d", leader.Char.PartyID, second.Char.PartyID)
	}
}

func TestPartyRuntimeLeaveBreaksTwoPersonParty(t *testing.T) {
	m := &partyManager{
		nextID:      1,
		parties:     make(map[uint32]*socialParty),
		invites:     make(map[uint32]pendingPartyInvite),
		charToParty: make(map[uint32]uint32),
	}
	leader := testSocialClient(1, 1001, "Leader")
	member := testSocialClient(2, 1002, "Member")
	m.acceptInvite(leader, member, partyInviteTypeFirstPickup)

	removed, broken := m.leave(member)
	if !broken {
		t.Fatal("expected two-person party to break")
	}
	if len(removed) != 2 {
		t.Fatalf("removed=%d", len(removed))
	}
	if leader.Char.PartyID != 0 || member.Char.PartyID != 0 {
		t.Fatalf("party ids leader=%d member=%d", leader.Char.PartyID, member.Char.PartyID)
	}
}

func TestFriendRuntimeStoresBothSides(t *testing.T) {
	m := &friendManager{
		invites: make(map[uint32]pendingFriendInvite),
		rows:    make(map[uint32]map[uint32]FriendRow),
	}
	a := testSocialClient(1, 1001, "Alpha")
	b := testSocialClient(2, 1002, "Beta")

	m.addFriend(a, b)

	if got := m.friends(a); len(got) != 1 || got[0].Name != "Beta" {
		t.Fatalf("alpha friends=%#v", got)
	}
	if got := m.friends(b); len(got) != 1 || got[0].Name != "Alpha" {
		t.Fatalf("beta friends=%#v", got)
	}
}

func TestPartyInviteConditionsMatchReferenceBasics(t *testing.T) {
	m := newTestPartyManager()
	leader := testSocialClient(1, 1001, "Leader")
	member := testSocialClient(2, 1002, "Member")
	target := testSocialClient(3, 1003, "Target")
	enablePartyRequests(leader, member, target)
	party := m.acceptInvite(leader, member, partyInviteTypeFirstPickup)
	if party == nil {
		t.Fatal("expected party")
	}

	if got := m.checkInvite(member, target); got != partyInviteInviterNotLeader {
		t.Fatalf("non-leader invite=%v", got)
	}
	if got := m.checkInvite(leader, target); got != partyInviteOK {
		t.Fatalf("leader invite=%v", got)
	}
}

func TestPartyInviteRejectsTargetInGroupAndDisabledRequests(t *testing.T) {
	m := newTestPartyManager()
	a := testSocialClient(1, 1001, "A")
	b := testSocialClient(2, 1002, "B")
	c := testSocialClient(3, 1003, "C")
	d := testSocialClient(4, 1004, "D")
	enablePartyRequests(a, b, c)
	party := m.acceptInvite(a, b, partyInviteTypeFirstPickup)
	if party == nil {
		t.Fatal("expected party")
	}
	if got := m.checkInvite(c, b); got != partyInviteTargetInGroup {
		t.Fatalf("target in group=%v", got)
	}
	if got := m.checkInvite(c, d); got != partyInviteTargetRejects {
		t.Fatalf("disabled requests=%v", got)
	}
}

func TestPartyInviteRejectsFullPartyAndPendingInvites(t *testing.T) {
	m := newTestPartyManager()
	leader := testSocialClient(1, 1001, "Leader")
	enablePartyRequests(leader)
	for i := uint32(2); i <= maxPartyMembers; i++ {
		member := testSocialClient(i, 1000+i, "Member")
		enablePartyRequests(member)
		if party := m.acceptInvite(leader, member, partyInviteTypeFirstPickup); party == nil {
			t.Fatalf("member %d was not added", i)
		}
	}
	target := testSocialClient(9, 1009, "Target")
	enablePartyRequests(target)
	if got := m.checkInvite(leader, target); got != partyInviteInviterGroupFull {
		t.Fatalf("full party=%v", got)
	}

	m = newTestPartyManager()
	leader = testSocialClient(10, 1010, "Leader")
	first := testSocialClient(11, 1011, "First")
	second := testSocialClient(12, 1012, "Second")
	enablePartyRequests(leader, first, second)
	m.addInvite(leader, first, partyInviteTypeFirstPickup)
	if got := m.checkInvite(leader, first); got != partyInviteTargetAlreadyInvited {
		t.Fatalf("pending invite=%v", got)
	}
	if got := m.checkInvite(leader, second); got != partyInviteOK {
		t.Fatalf("second invite=%v", got)
	}
}

func TestReadPartyInviteRequestFindsEmbeddedSession(t *testing.T) {
	sender := testSocialClient(1, 1001, "Sender")
	target := testSocialClient(2, 1002, "Target")
	sender.Channel = 0
	target.Channel = 0
	sender.Char.MapID = 0
	target.Char.MapID = 0
	target.Char.SessionID = 2
	withGameClients(t, sender, target)

	p := &PacketIn{Data: []byte{
		0xC9, 0x0E, 0x5B, 0x7B, 0xDD, 0xCC, 0xBB, 0xAA,
		0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22,
		0x11, 0x10, 0x0F, 0x0E, 0x02, 0x00, 0x01,
	}}
	sessionID, inviteType := readPartyInviteRequest(sender, p)
	if sessionID != 2 {
		t.Fatalf("sessionID=%d", sessionID)
	}
	if inviteType != 1 {
		t.Fatalf("inviteType=%d", inviteType)
	}
}

func TestReadPartyInviteAnswerUsesWCellOffset(t *testing.T) {
	accepted := &PacketIn{Data: []byte{
		0xB8, 0xA0, 0x60, 0x04, 0xE0, 0xA5, 0xB6, 0xF3,
		0x16, 0xF0, 0x40, 0xCC, 0xAA, 0xBB, 0xCC, 0xDD,
		0x01, 0x00,
	}}
	if !readPartyInviteAnswer(accepted) {
		t.Fatal("expected accepted")
	}

	declined := &PacketIn{Data: []byte{
		0xB8, 0xA0, 0x60, 0x04, 0xE0, 0xA5, 0xB6, 0xF3,
		0x16, 0xF0, 0x40, 0xCC, 0xAA, 0xBB, 0xCC, 0xDD,
		0x00, 0x01,
	}}
	if readPartyInviteAnswer(declined) {
		t.Fatal("expected declined")
	}
}

func testSocialClient(guid uint32, accID uint32, name string) *Client {
	return &Client{
		Char: &Character{
			GUID:            guid,
			AccID:           accID,
			Name:            name,
			SessionID:       int16(guid),
			CharNum:         byte(guid),
			Level:           1,
			ProfessionLevel: 1,
			Class:           1,
			MapID:           0,
			MaxHP:           100,
			HP:              100,
			MaxMP:           50,
			MP:              50,
		},
	}
}

func newTestPartyManager() *partyManager {
	return &partyManager{
		nextID:      1,
		parties:     make(map[uint32]*socialParty),
		invites:     make(map[uint32]pendingPartyInvite),
		charToParty: make(map[uint32]uint32),
	}
}

func enablePartyRequests(clients ...*Client) {
	for _, c := range clients {
		c.Char.SettingsFlags[settingsFlagPartyRequest] = 1
	}
}

func withGameClients(t *testing.T, list ...*Client) {
	t.Helper()
	clientsMu.Lock()
	oldClients := clients
	oldNextID := nextID
	clients = make(map[uint32]*Client)
	nextID = 1
	for _, c := range list {
		c.ID = nextID
		clients[c.ID] = c
		nextID++
	}
	clientsMu.Unlock()

	t.Cleanup(func() {
		clientsMu.Lock()
		clients = oldClients
		nextID = oldNextID
		clientsMu.Unlock()
	})
}
