package main

import (
	"errors"
	"log"
	"sync"
	"time"

	"asda2/shared/db"
)

const (
	guildCreateNeedLevel10 byte = 0
	guildCreateOK          byte = 1
	guildCreateNoMoney     byte = 4
	guildCreateInGuild     byte = 5
	guildCreateBadFaction  byte = 7
	guildCreateFailed      byte = 8
	guildCreateNameExists  byte = 9
	guildCreateBadName     byte = 10
)

const (
	guildPrivilegeSetMember byte = 1 << iota
	guildPrivilegeApplicants
	guildPrivilegeUsePoints
	guildPrivilegeEditAnnounce
	guildPrivilegeEditRankSettings
	guildPrivilegeEditCrest
	guildPrivilegeInviteMembers
)

const guildPrivilegeAll uint16 = 127

const (
	guildInfoSilent              byte = 0
	guildInfoAnnouncement        byte = 1
	guildInfoCrestChanged        byte = 2
	guildInfoPrivilegesChanged   byte = 3
	guildInfoGuildLevelChanged   byte = 5
	guildInfoGuildNameChanged    byte = 6
	guildNotificationDonated     byte = 0
	guildNotificationJoined      byte = 1
	guildNotificationLeft        byte = 2
	guildNotificationKicked      byte = 3
	guildNotificationLoggedIn    byte = 4
	guildNotificationLoggedOut   byte = 5
	guildNotificationSilence     byte = 6
	guildNotificationRankChanged byte = 7
	guildNotificationNoteEdited  byte = 8
	guildNotificationNewLeader   byte = 9
)

const (
	guildHistoryJoined byte = 1 + iota
	guildHistoryLeft
	guildHistoryKicked
	guildHistoryAppointedLeader
	guildHistoryAppointedLeaderVote
	guildHistoryDonatedPoints
	guildHistoryUsedPoints
	guildHistoryLevelNowIs
)

const (
	guildSkillLearnFailed byte = iota
	guildSkillLearnOK
	guildSkillLearnProfileProblem
	guildSkillLearnNotInGuild
	guildSkillLearnGuildProblem
	guildSkillLearnNoPermission
	guildSkillLearnSkillProblem
	guildSkillLearnMaxLevel
	guildSkillLearnGuildLevelLow
	guildSkillLearnNoPoints
	guildSkillLearnActivated
)

const (
	guildSkillActivatedFail         byte = 0
	guildSkillActivatedOK           byte = 1
	guildSkillActivatedNoPermission byte = 5
	guildSkillActivatedNoPoints     byte = 7
)

const (
	guildSkillStatusLearned    byte = 1
	guildSkillStatusActivation byte = 2
)

const (
	guildPrivilegeChangeFail         byte = 0
	guildPrivilegeChangeOK           byte = 1
	guildPrivilegeChangeNoPermission byte = 4
)

const (
	guildRankChangeOK           byte = 1
	guildRankChangeNoPermission byte = 6
	guildRankChangeBadRank      byte = 7
	guildRankChangeHigherRank   byte = 9
)

const (
	guildImpeachmentFailed            byte = 0
	guildImpeachmentSuccess           byte = 1
	guildImpeachmentAlreadyInProgress byte = 6
)

const (
	guildInviteTimeout    = 60 * time.Second
	guildImpeachmentDelay = 3 * time.Minute
)

var guildLevelUpCosts = []int32{0, 30000, 70000, 120000, 350000, 500000, 800000, 1200000, 1750000, 2500000}

type guildSkillTemplate struct {
	LearnCosts      []int32
	ActivationCosts []int32
	MaxLevel        byte
}

var guildSkillTemplates = map[int16]guildSkillTemplate{
	0: {MaxLevel: 7, LearnCosts: []int32{0, 60000, 100000, 400000, 700000, 1000000, 2000000, 3000000}, ActivationCosts: []int32{0, 6000, 10000, 40000, 70000, 100000, 200000, 300000}},
	1: {MaxLevel: 7, LearnCosts: []int32{0, 25000, 60000, 100000, 300000, 450000, 1200000, 2000000}, ActivationCosts: []int32{0, 2500, 6000, 10000, 30000, 45000, 120000, 200000}},
	2: {MaxLevel: 7, LearnCosts: []int32{0, 100000, 300000, 400000, 700000, 1000000, 1500000, 2500000}, ActivationCosts: []int32{0, 10000, 30000, 40000, 70000, 100000, 150000, 250000}},
	3: {MaxLevel: 7, LearnCosts: []int32{0, 550000, 1000000, 1350000, 1800000, 2400000, 3000000, 5000000}, ActivationCosts: []int32{0, 55000, 100000, 135000, 180000, 240000, 300000, 500000}},
	4: {MaxLevel: 4, LearnCosts: []int32{0, 500000, 1000000, 2000000, 3000000}, ActivationCosts: []int32{0, 50000, 100000, 200000, 300000}},
}

type guildState struct {
	ID                uint32
	Name              string
	Level             byte
	MaxMembers        byte
	Points            int32
	WaveLimit         byte
	Crest             []byte
	MOTD              string
	NoticeWriter      string
	NoticeTime        time.Time
	LeaderCharacterID uint32
	Ranks             map[byte]db.GuildRankData
	Members           map[uint32]*db.GuildMemberData
	Skills            map[int16]*db.GuildSkillData
	History           []db.GuildHistoryData
}

type pendingGuildInvite struct {
	InviterGUID uint32
	InviteeGUID uint32
	GuildID     uint32
	CreatedAt   time.Time
}

type guildImpeachment struct {
	GuildID       uint32
	CandidateGUID uint32
	StartedAt     time.Time
	Votes         map[uint32]struct{}
}

type guildManager struct {
	mu           sync.RWMutex
	guilds       map[uint32]*guildState
	invites      map[uint32]pendingGuildInvite
	impeachments map[uint32]*guildImpeachment
}

var guildRuntime = &guildManager{
	guilds:       make(map[uint32]*guildState),
	invites:      make(map[uint32]pendingGuildInvite),
	impeachments: make(map[uint32]*guildImpeachment),
}

func (m *guildManager) attachClient(c *Client) (*guildState, *db.GuildMemberData, error) {
	if c == nil || c.Char == nil {
		return nil, nil, nil
	}
	data, err := db.LoadGuildForCharacter(c.Char.GUID)
	if err != nil {
		return nil, nil, err
	}
	if data == nil {
		c.Char.GuildID = 0
		c.Char.GuildRank = 0
		return nil, nil, nil
	}
	g := newGuildState(data)
	m.mu.Lock()
	m.guilds[g.ID] = g
	member := g.Members[c.Char.GUID]
	if member != nil {
		updateMemberFromClient(member, c)
	}
	m.mu.Unlock()
	if member == nil {
		c.Char.GuildID = 0
		c.Char.GuildRank = 0
		return nil, nil, nil
	}
	c.Char.GuildID = g.ID
	c.Char.GuildRank = asdaGuildRank(member.RankIndex)
	c.Char.GuildPoints = member.GuildPoints
	if err := db.UpdateGuildMemberSnapshot(c.Char); err != nil {
		log.Printf("[Guild] update member snapshot char=%d: %v", c.Char.GUID, err)
	}
	return g, member, nil
}

func (m *guildManager) detachClient(c *Client) {
	if c == nil || c.Char == nil || c.Char.GuildID == 0 {
		return
	}
	g, member, err := m.guildForCharacter(c.Char)
	if err == nil && g != nil && member != nil {
		sendGuildNotification(g, guildNotificationLoggedOut, member)
	}
	if err := db.UpdateGuildMemberSnapshot(c.Char); err != nil {
		log.Printf("[Guild] update logout snapshot char=%d: %v", c.Char.GUID, err)
	}
}

func (m *guildManager) guildForCharacter(chr *Character) (*guildState, *db.GuildMemberData, error) {
	if chr == nil || chr.GuildID == 0 {
		return nil, nil, nil
	}
	g, err := m.loadGuild(chr.GuildID)
	if err != nil || g == nil {
		return nil, nil, err
	}
	member := g.member(chr.GUID)
	if member == nil {
		return nil, nil, nil
	}
	chr.GuildRank = asdaGuildRank(member.RankIndex)
	return g, member, nil
}

func (m *guildManager) loadGuild(guildID uint32) (*guildState, error) {
	if guildID == 0 {
		return nil, nil
	}
	m.mu.RLock()
	g := m.guilds[guildID]
	m.mu.RUnlock()
	if g != nil {
		return g, nil
	}
	data, err := db.LoadGuildByID(guildID)
	if err != nil || data == nil {
		return nil, err
	}
	g = newGuildState(data)
	m.mu.Lock()
	m.guilds[guildID] = g
	m.mu.Unlock()
	return g, nil
}

func (m *guildManager) setGuild(g *guildState) {
	if g == nil {
		return
	}
	m.mu.Lock()
	m.guilds[g.ID] = g
	m.mu.Unlock()
}

func (m *guildManager) addInvite(inviter *Client, invitee *Client, guildID uint32) {
	if inviter == nil || inviter.Char == nil || invitee == nil || invitee.Char == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpiredInvitesLocked(time.Now())
	m.invites[invitee.Char.GUID] = pendingGuildInvite{
		InviterGUID: inviter.Char.GUID,
		InviteeGUID: invitee.Char.GUID,
		GuildID:     guildID,
		CreatedAt:   time.Now(),
	}
}

func (m *guildManager) consumeInvite(invitee *Client) (*Client, *guildState) {
	if invitee == nil || invitee.Char == nil {
		return nil, nil
	}
	m.mu.Lock()
	invite, ok := m.invites[invitee.Char.GUID]
	if !ok || time.Since(invite.CreatedAt) > guildInviteTimeout {
		delete(m.invites, invitee.Char.GUID)
		m.mu.Unlock()
		return nil, nil
	}
	delete(m.invites, invitee.Char.GUID)
	m.mu.Unlock()
	inviter := getClientByGUID(invite.InviterGUID)
	g, _ := m.loadGuild(invite.GuildID)
	return inviter, g
}

func (m *guildManager) removeExpiredInvitesLocked(now time.Time) {
	for inviteeGUID, invite := range m.invites {
		if now.Sub(invite.CreatedAt) > guildInviteTimeout {
			delete(m.invites, inviteeGUID)
		}
	}
}

func (m *guildManager) createGuild(name string, leader *Client) (*guildState, error) {
	if leader == nil || leader.Char == nil {
		return nil, errors.New("leader has no character")
	}
	data, err := db.CreateGuild(name, leader.Char)
	if err != nil {
		return nil, err
	}
	g := newGuildState(data)
	member := g.member(leader.Char.GUID)
	if member != nil {
		leader.Char.GuildID = g.ID
		leader.Char.GuildRank = asdaGuildRank(member.RankIndex)
	}
	m.setGuild(g)
	return g, nil
}

func (m *guildManager) addMember(g *guildState, invitee *Client) (*db.GuildMemberData, error) {
	if g == nil || invitee == nil || invitee.Char == nil {
		return nil, errors.New("missing guild or invitee")
	}
	if err := db.AddGuildMember(g.ID, invitee.Char, 4); err != nil {
		return nil, err
	}
	data, err := db.LoadGuildByID(g.ID)
	if err != nil {
		return nil, err
	}
	refreshed := newGuildState(data)
	m.setGuild(refreshed)
	member := refreshed.member(invitee.Char.GUID)
	if member != nil {
		invitee.Char.GuildID = refreshed.ID
		invitee.Char.GuildRank = asdaGuildRank(member.RankIndex)
	}
	return member, nil
}

func (m *guildManager) removeMember(g *guildState, member *db.GuildMemberData, kicked bool) error {
	if g == nil || member == nil {
		return nil
	}
	historyType := guildHistoryLeft
	if kicked {
		historyType = guildHistoryKicked
	}
	if err := db.RemoveGuildMember(g.ID, member.CharacterID, historyType, member.Name); err != nil {
		return err
	}
	m.mu.Lock()
	delete(g.Members, member.CharacterID)
	if c := getClientByGUID(member.CharacterID); c != nil && c.Char != nil {
		c.Char.GuildID = 0
		c.Char.GuildRank = 0
	}
	m.mu.Unlock()
	return nil
}

func (m *guildManager) deleteGuild(g *guildState) error {
	if g == nil {
		return nil
	}
	if err := db.DeleteGuild(g.ID); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.guilds, g.ID)
	for _, member := range g.Members {
		if c := getClientByGUID(member.CharacterID); c != nil && c.Char != nil {
			c.Char.GuildID = 0
			c.Char.GuildRank = 0
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *guildManager) startImpeachment(g *guildState, candidate *db.GuildMemberData) byte {
	if g == nil || candidate == nil {
		return guildImpeachmentFailed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.impeachments[g.ID]; current != nil && time.Since(current.StartedAt) < 4*time.Minute {
		return guildImpeachmentAlreadyInProgress
	}
	m.impeachments[g.ID] = &guildImpeachment{
		GuildID:       g.ID,
		CandidateGUID: candidate.CharacterID,
		StartedAt:     time.Now(),
		Votes:         make(map[uint32]struct{}),
	}
	time.AfterFunc(guildImpeachmentDelay, func() {
		guildRuntime.finishImpeachment(g.ID)
	})
	return guildImpeachmentSuccess
}

func (m *guildManager) addImpeachmentVote(guildID uint32, member *db.GuildMemberData) {
	if guildID == 0 || member == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.impeachments[guildID]
	if current == nil || current.CandidateGUID == member.CharacterID || member.RankIndex == 0 {
		return
	}
	current.Votes[member.CharacterID] = struct{}{}
}

func (m *guildManager) finishImpeachment(guildID uint32) {
	m.mu.Lock()
	current := m.impeachments[guildID]
	delete(m.impeachments, guildID)
	m.mu.Unlock()
	if current == nil {
		return
	}
	g, err := m.loadGuild(guildID)
	if err != nil || g == nil {
		return
	}
	candidate := g.member(current.CandidateGUID)
	if candidate == nil {
		sendImpeachmentResult(g, false)
		return
	}
	accepted := float64(len(current.Votes)) / float64(max(1, len(g.Members))) * 100
	if accepted <= 50 {
		sendImpeachmentResult(g, false)
		return
	}
	leader := g.leader()
	if leader != nil {
		leader.RankIndex = 1
		_ = db.UpdateGuildMemberRank(g.ID, leader.CharacterID, leader.RankIndex)
		sendGuildNotification(g, guildNotificationRankChanged, leader)
	}
	candidate.RankIndex = 0
	g.LeaderCharacterID = candidate.CharacterID
	_ = db.UpdateGuildMemberRank(g.ID, candidate.CharacterID, candidate.RankIndex)
	_ = db.AddGuildHistory(g.ID, guildHistoryAppointedLeaderVote, 0, candidate.Name, time.Now().Format("15:04:05"))
	sendGuildNotification(g, guildNotificationNewLeader, candidate)
	sendImpeachmentResult(g, true)
	sendUpdateGuildInfo(g, guildInfoSilent, nil)
}

func newGuildState(data *db.GuildData) *guildState {
	if data == nil {
		return nil
	}
	g := &guildState{
		ID:                data.ID,
		Name:              data.Name,
		Level:             data.Level,
		MaxMembers:        data.MaxMembers,
		Points:            data.Points,
		WaveLimit:         data.WaveLimit,
		Crest:             normalizeCrest(data.Crest),
		MOTD:              data.MOTD,
		NoticeWriter:      data.NoticeWriter,
		NoticeTime:        data.NoticeTime,
		LeaderCharacterID: data.LeaderCharacterID,
		Ranks:             make(map[byte]db.GuildRankData),
		Members:           make(map[uint32]*db.GuildMemberData),
		Skills:            make(map[int16]*db.GuildSkillData),
		History:           append([]db.GuildHistoryData(nil), data.History...),
	}
	for _, rank := range data.Ranks {
		g.Ranks[rank.RankIndex] = rank
	}
	for _, member := range data.Members {
		m := member
		g.Members[m.CharacterID] = &m
	}
	for _, skill := range data.Skills {
		s := skill
		g.Skills[s.SkillID] = &s
	}
	return g
}

func (g *guildState) member(characterID uint32) *db.GuildMemberData {
	if g == nil {
		return nil
	}
	return g.Members[characterID]
}

func (g *guildState) memberByAccountChar(accountID uint32, charNum byte) *db.GuildMemberData {
	if g == nil {
		return nil
	}
	for _, member := range g.Members {
		if member.AccountID == accountID && member.CharNum == charNum {
			return member
		}
	}
	return nil
}

func (g *guildState) leader() *db.GuildMemberData {
	if g == nil {
		return nil
	}
	if member := g.Members[g.LeaderCharacterID]; member != nil {
		return member
	}
	for _, member := range g.Members {
		if member.RankIndex == 0 {
			return member
		}
	}
	return nil
}

func (g *guildState) hasPrivilege(member *db.GuildMemberData, privilege byte) bool {
	if g == nil || member == nil {
		return false
	}
	if member.RankIndex == 0 || member.CharacterID == g.LeaderCharacterID {
		return true
	}
	rank := g.Ranks[member.RankIndex]
	return rank.Privileges&uint16(privilege) != 0
}

func (g *guildState) sortedMembers() []*db.GuildMemberData {
	out := make([]*db.GuildMemberData, 0, len(g.Members))
	for _, member := range g.Members {
		out = append(out, member)
	}
	sortGuildMembers(out)
	return out
}

func (g *guildState) memberCount() byte {
	if g == nil {
		return 0
	}
	if len(g.Members) > 255 {
		return 255
	}
	return byte(len(g.Members))
}

func updateMemberFromClient(member *db.GuildMemberData, c *Client) {
	if member == nil || c == nil || c.Char == nil {
		return
	}
	member.Name = c.Char.Name
	member.Level = c.Char.Level
	member.ProfessionLevel = c.Char.ProfessionLevel
	member.Class = c.Char.Class
	member.LastMapID = c.Char.MapID
	member.GuildPoints = c.Char.GuildPoints
}

func asdaGuildRank(rankIndex byte) byte {
	if rankIndex > 4 {
		return 0
	}
	return 4 - rankIndex
}

func rankIndexFromAsda(asdaRank byte) byte {
	if asdaRank > 4 {
		return 4
	}
	return 4 - asdaRank
}

func normalizeCrest(crest []byte) []byte {
	out := make([]byte, db.GuildCrestLength)
	copy(out, crest)
	return out
}
