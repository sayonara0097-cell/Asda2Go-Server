package main

import (
	"sync"
	"time"
)

const (
	maxPartyMembers              = 5
	partyInviteTimeout           = 30 * time.Second
	settingsFlagPartyRequest     = 9
	partyInviteTypeFirstPickup   = 0
	partyInviteTypeItemAutoShare = 1
)

const (
	partyInviteStatusDeclined byte = iota
	partyInviteStatusAccepted
	partyInviteStatusNoTarget
	partyInviteStatusExpired
	partyInviteStatusAlreadyInParty
	partyInviteStatusSent
	partyInviteStatusIncoming
)

type partyInviteFailure byte

const (
	partyInviteOK partyInviteFailure = iota
	partyInviteNoTarget
	partyInviteSelf
	partyInviteFaction
	partyInviteTargetRejects
	partyInviteInviterNotLeader
	partyInviteInviterGroupFull
	partyInviteTargetInGroup
	partyInviteTargetAlreadyInvited
	partyInviteTooManyPending
)

type socialParty struct {
	ID         uint32
	LeaderGUID uint32
	InviteType byte
	Members    map[uint32]*Client
}

type pendingPartyInvite struct {
	InviterGUID uint32
	InviteeGUID uint32
	InviteType  byte
	CreatedAt   time.Time
}

type partyManager struct {
	mu          sync.RWMutex
	nextID      uint32
	parties     map[uint32]*socialParty
	invites     map[uint32]pendingPartyInvite
	charToParty map[uint32]uint32
}

var partyRuntime = &partyManager{
	nextID:      1,
	parties:     make(map[uint32]*socialParty),
	invites:     make(map[uint32]pendingPartyInvite),
	charToParty: make(map[uint32]uint32),
}

func (m *partyManager) addInvite(inviter, invitee *Client, inviteType byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inviteType = normalizePartyInviteType(inviteType)
	m.invites[invitee.Char.GUID] = pendingPartyInvite{
		InviterGUID: inviter.Char.GUID,
		InviteeGUID: invitee.Char.GUID,
		InviteType:  inviteType,
		CreatedAt:   time.Now(),
	}
}

func (m *partyManager) checkInvite(inviter, invitee *Client) partyInviteFailure {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpiredInvitesLocked(time.Now())
	if inviter == nil || inviter.Char == nil || invitee == nil || invitee.Char == nil {
		return partyInviteNoTarget
	}
	if inviter == invitee || inviter.Char.GUID == invitee.Char.GUID {
		return partyInviteSelf
	}
	if !partyRequestsEnabled(invitee.Char) {
		return partyInviteTargetRejects
	}
	if inviter.Char.FactionID != -1 && invitee.Char.FactionID != -1 && inviter.Char.FactionID != invitee.Char.FactionID {
		return partyInviteFaction
	}

	inviterParty := m.partyForLocked(inviter.Char.GUID)
	if inviterParty != nil {
		if inviterParty.LeaderGUID != inviter.Char.GUID {
			return partyInviteInviterNotLeader
		}
		if len(inviterParty.Members) >= maxPartyMembers {
			return partyInviteInviterGroupFull
		}
		if m.pendingInviteCountLocked(inviter.Char.GUID) >= maxPartyMembers-len(inviterParty.Members) {
			return partyInviteTooManyPending
		}
	}
	if m.partyForLocked(invitee.Char.GUID) != nil {
		return partyInviteTargetInGroup
	}
	if _, ok := m.invites[invitee.Char.GUID]; ok {
		return partyInviteTargetAlreadyInvited
	}
	return partyInviteOK
}

func (m *partyManager) inviteTypeFor(inviter *Client, requested byte) byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if inviter == nil || inviter.Char == nil {
		return normalizePartyInviteType(requested)
	}
	if party := m.partyForLocked(inviter.Char.GUID); party != nil {
		return party.InviteType
	}
	return normalizePartyInviteType(requested)
}

func (m *partyManager) consumeInvite(invitee *Client) (*Client, byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[invitee.Char.GUID]
	if !ok || time.Since(invite.CreatedAt) > partyInviteTimeout {
		delete(m.invites, invitee.Char.GUID)
		return nil, 0
	}
	delete(m.invites, invitee.Char.GUID)
	return getClientByGUID(invite.InviterGUID), invite.InviteType
}

func (m *partyManager) acceptInvite(inviter, invitee *Client, inviteType byte) *socialParty {
	m.mu.Lock()
	defer m.mu.Unlock()
	if inviter.Char == nil || invitee.Char == nil {
		return nil
	}

	party := m.partyForLocked(inviter.Char.GUID)
	if party == nil {
		party = &socialParty{
			ID:         m.nextID,
			LeaderGUID: inviter.Char.GUID,
			InviteType: normalizePartyInviteType(inviteType),
			Members:    make(map[uint32]*Client),
		}
		m.nextID++
		m.parties[party.ID] = party
		m.addMemberLocked(party, inviter)
	}
	if len(party.Members) >= maxPartyMembers || m.partyForLocked(invitee.Char.GUID) != nil {
		return nil
	}
	m.addMemberLocked(party, invitee)
	return party.snapshot()
}

func (m *partyManager) leave(c *Client) ([]*Client, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	party := m.partyForLocked(c.Char.GUID)
	if party == nil {
		return nil, false
	}
	delete(party.Members, c.Char.GUID)
	delete(m.charToParty, c.Char.GUID)
	c.Char.PartyID = 0

	remaining := party.members()
	if len(remaining) <= 1 {
		for _, member := range remaining {
			member.Char.PartyID = 0
			delete(m.charToParty, member.Char.GUID)
		}
		delete(m.parties, party.ID)
		return append(remaining, c), true
	}
	if party.LeaderGUID == c.Char.GUID {
		party.LeaderGUID = remaining[0].Char.GUID
	}
	return remaining, false
}

func (m *partyManager) kick(leader, target *Client) (*socialParty, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if leader == nil || leader.Char == nil || target == nil || target.Char == nil {
		return nil, false
	}
	party := m.partyForLocked(leader.Char.GUID)
	if party == nil || party.LeaderGUID != leader.Char.GUID {
		return nil, false
	}
	if _, ok := party.Members[target.Char.GUID]; !ok || target == leader {
		return nil, false
	}
	delete(party.Members, target.Char.GUID)
	delete(m.charToParty, target.Char.GUID)
	target.Char.PartyID = 0
	if len(party.Members) <= 1 {
		for _, member := range party.Members {
			member.Char.PartyID = 0
			delete(m.charToParty, member.Char.GUID)
		}
		delete(m.parties, party.ID)
	}
	return party.snapshot(), true
}

func (m *partyManager) members(partyID uint32) []*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	party := m.parties[partyID]
	if party == nil {
		return nil
	}
	return party.members()
}

func (m *partyManager) partyFor(guid uint32) *socialParty {
	m.mu.RLock()
	defer m.mu.RUnlock()
	party := m.partyForLocked(guid)
	if party == nil {
		return nil
	}
	return party.snapshot()
}

func (m *partyManager) addMemberLocked(party *socialParty, c *Client) {
	party.Members[c.Char.GUID] = c
	m.charToParty[c.Char.GUID] = party.ID
	c.Char.PartyID = party.ID
}

func (m *partyManager) partyForLocked(guid uint32) *socialParty {
	partyID := m.charToParty[guid]
	if partyID == 0 {
		return nil
	}
	return m.parties[partyID]
}

func (m *partyManager) pendingInviteCountLocked(inviterGUID uint32) int {
	count := 0
	for _, invite := range m.invites {
		if invite.InviterGUID == inviterGUID {
			count++
		}
	}
	return count
}

func (m *partyManager) removeExpiredInvitesLocked(now time.Time) {
	for inviteeGUID, invite := range m.invites {
		if now.Sub(invite.CreatedAt) > partyInviteTimeout {
			delete(m.invites, inviteeGUID)
		}
	}
}

func partyRequestsEnabled(chr *Character) bool {
	if chr == nil {
		return false
	}
	if len(chr.SettingsFlags) <= settingsFlagPartyRequest {
		return false
	}
	return chr.SettingsFlags[settingsFlagPartyRequest] == 1
}

func normalizePartyInviteType(inviteType byte) byte {
	if inviteType == partyInviteTypeItemAutoShare {
		return partyInviteTypeItemAutoShare
	}
	return partyInviteTypeFirstPickup
}

func (p *socialParty) members() []*Client {
	out := make([]*Client, 0, len(p.Members))
	if leader := p.Members[p.LeaderGUID]; leader != nil {
		out = append(out, leader)
	}
	for guid, member := range p.Members {
		if guid != p.LeaderGUID {
			out = append(out, member)
		}
	}
	return out
}

func (p *socialParty) snapshot() *socialParty {
	if p == nil {
		return nil
	}
	out := &socialParty{
		ID:         p.ID,
		LeaderGUID: p.LeaderGUID,
		InviteType: p.InviteType,
		Members:    make(map[uint32]*Client, len(p.Members)),
	}
	for guid, member := range p.Members {
		out.Members[guid] = member
	}
	return out
}

type pendingFriendInvite struct {
	InviterGUID uint32
	InviteeGUID uint32
	CreatedAt   time.Time
}

type friendManager struct {
	mu      sync.RWMutex
	invites map[uint32]pendingFriendInvite
	rows    map[uint32]map[uint32]FriendRow
}

var friendRuntime = &friendManager{
	invites: make(map[uint32]pendingFriendInvite),
	rows:    make(map[uint32]map[uint32]FriendRow),
}

func (m *friendManager) addInvite(inviter, invitee *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invites[invitee.Char.GUID] = pendingFriendInvite{
		InviterGUID: inviter.Char.GUID,
		InviteeGUID: invitee.Char.GUID,
		CreatedAt:   time.Now(),
	}
}

func (m *friendManager) inviterFor(invitee *Client) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	invite, ok := m.invites[invitee.Char.GUID]
	if !ok {
		return nil
	}
	return getClientByGUID(invite.InviterGUID)
}

func (m *friendManager) consumeInvite(inviter, invitee *Client) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	invite, ok := m.invites[invitee.Char.GUID]
	if !ok || invite.InviterGUID != inviter.Char.GUID || time.Since(invite.CreatedAt) > 60*time.Second {
		delete(m.invites, invitee.Char.GUID)
		return false
	}
	delete(m.invites, invitee.Char.GUID)
	return true
}

func (m *friendManager) clearInvite(invitee *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.invites, invitee.Char.GUID)
}

func (m *friendManager) addFriend(a, b *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addFriendLocked(a.Char.GUID, friendRowFromClient(b))
	m.addFriendLocked(b.Char.GUID, friendRowFromClient(a))
}

func (m *friendManager) deleteFriend(c *Client, targetAccID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows[c.Char.GUID], targetAccID)
}

func (m *friendManager) friends(c *Client) []FriendRow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rows := m.rows[c.Char.GUID]
	out := make([]FriendRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

func (m *friendManager) addFriendLocked(ownerGUID uint32, row FriendRow) {
	if m.rows[ownerGUID] == nil {
		m.rows[ownerGUID] = make(map[uint32]FriendRow)
	}
	m.rows[ownerGUID][row.AccountID] = row
}

func friendRowFromClient(c *Client) FriendRow {
	return FriendRow{
		CharacterID:     c.Char.GUID,
		AccountID:       c.Char.AccID,
		CharNum:         c.Char.CharNum,
		Name:            c.Char.Name,
		MapID:           c.Char.MapID,
		Level:           c.Char.Level,
		ProfessionLevel: c.Char.ProfessionLevel,
		Class:           c.Char.Class,
		Online:          true,
	}
}
