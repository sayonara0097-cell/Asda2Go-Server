package main

import (
	"sort"
	"strings"
	"time"

	"asda2/shared/db"
)

var guildFF40 = func() []byte {
	out := make([]byte, db.GuildCrestLength)
	for i := range out {
		out[i] = 0xFF
	}
	return out
}()

var guildCreateStub293 = make([]byte, 293)
var guildStab15 = []byte{190, 11, 0, 0}

func sendGuildCreated(c *Client, status byte, g *guildState) {
	p := NewPacket(GuildCreated)
	p.WriteUint8(status)
	if g == nil {
		p.WriteInt32(0)
		p.WriteInt16(0)
		p.WriteAsdaString("", 17)
		p.WriteInt16(0)
		p.WriteUint8(0)
		p.WriteInt32(0)
		p.WriteInt16(0)
		p.WriteUint8(0)
		p.WriteBytes(guildFF40)
		for i := 0; i < 4; i++ {
			p.WriteInt16(0)
		}
		p.WriteInt32(0)
		p.WriteUint8(10)
		p.WriteAsdaString("", 20)
		p.WriteBytes(guildCreateStub293)
		c.Send(p)
		return
	}
	leader := g.leader()
	p.WriteUint32(g.ID)
	p.WriteInt16(int16(g.ID))
	p.WriteAsdaString(g.Name, 17)
	p.WriteInt16(int16(g.Level))
	p.WriteUint8(g.MaxMembers)
	p.WriteInt32(int32(len(g.Members)))
	p.WriteInt16(0)
	p.WriteUint8(0)
	p.WriteBytes(guildFF40)
	p.WriteInt16(int16(guildRankPrivileges(g, 4)))
	p.WriteInt16(int16(guildRankPrivileges(g, 3)))
	p.WriteInt16(int16(guildRankPrivileges(g, 2)))
	p.WriteInt16(int16(guildRankPrivileges(g, 1)))
	if leader != nil {
		p.WriteUint32(leader.AccountID)
	} else {
		p.WriteInt32(0)
	}
	p.WriteUint8(10)
	if leader != nil {
		p.WriteAsdaString(leader.Name, 20)
	} else {
		p.WriteAsdaString("", 20)
	}
	p.WriteBytes(guildCreateStub293)
	c.Send(p)
}

func sendGuildInfoOnLogin(c *Client, g *guildState) {
	if c == nil || c.Char == nil || g == nil {
		return
	}
	leader := g.leader()
	p := NewPacket(GuildInfoOnLogin)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(int16(g.ID))
	p.WriteAsdaString(g.Name, 17)
	p.WriteInt16(int16(g.Level))
	p.WriteUint8(g.MaxMembers)
	p.WriteUint8(g.memberCount())
	p.WriteInt32(g.Points)
	p.WriteUint8(1)
	p.WriteUint8(guildHasCrest(g))
	p.WriteBytes(guildCrestOrEmpty(g))
	p.WriteInt16(int16(guildRankPrivileges(g, 4)))
	p.WriteInt16(int16(guildRankPrivileges(g, 3)))
	p.WriteInt16(int16(guildRankPrivileges(g, 2)))
	p.WriteInt16(int16(guildRankPrivileges(g, 1)))
	if leader != nil {
		p.WriteUint32(leader.AccountID)
		p.WriteUint8(leader.CharNum)
		p.WriteAsdaString(leader.Name, 20)
	} else {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteAsdaString("", 20)
	}
	p.WriteAsdaString(g.MOTD, 256)
	p.WriteAsdaString(formatGuildNoticeTime(g.NoticeTime), 17)
	p.WriteAsdaString(g.NoticeWriter, 20)
	c.Send(p)
}

func sendGuildMembersInfo(c *Client, g *guildState) {
	if c == nil || g == nil {
		return
	}
	members := g.sortedMembers()
	for len(members) > 0 {
		n := 5
		if len(members) < n {
			n = len(members)
		}
		batch := members[:n]
		members = members[n:]
		p := NewPacket(GuildMembersInfo)
		for i := 0; i < 5; i++ {
			if i < len(batch) {
				writeGuildMemberInfo(p, c, g, batch[i])
			} else {
				writeGuildMemberInfo(p, c, g, nil)
			}
		}
		c.Send(p)
	}
	c.Send(NewPacket(GuildMembersInfoEnded))
}

func sendGuildSkillsInfo(c *Client, g *guildState) {
	if c == nil || c.Char == nil || g == nil {
		return
	}
	p := NewPacket(GuildSkillsInfo)
	p.WriteUint32(c.Char.AccID)
	writeGuildSkills(p, g)
	c.Send(p)
}

func sendUpdateGuildInfo(g *guildState, mode byte, receiver *Client) {
	if g == nil {
		return
	}
	build := func() *PacketOut {
		leader := g.leader()
		p := NewPacket(UpdateGuildInfo)
		p.WriteUint8(2)
		p.WriteUint8(mode)
		p.WriteInt16(int16(g.ID))
		p.WriteAsdaString(g.Name, 17)
		p.WriteInt16(int16(g.Level))
		p.WriteUint8(g.MaxMembers)
		p.WriteUint8(g.memberCount())
		p.WriteInt32(g.Points)
		p.WriteUint8(g.WaveLimit)
		p.WriteUint8(0)
		writeGuildSkills(p, g)
		p.WriteInt16(int16(guildRankPrivileges(g, 4)))
		p.WriteInt16(int16(guildRankPrivileges(g, 3)))
		p.WriteInt16(int16(guildRankPrivileges(g, 2)))
		p.WriteInt16(int16(guildRankPrivileges(g, 1)))
		if leader != nil {
			p.WriteUint32(leader.AccountID)
			p.WriteUint8(leader.CharNum)
			p.WriteAsdaString(leader.Name, 20)
		} else {
			p.WriteInt32(0)
			p.WriteUint8(0)
			p.WriteAsdaString("", 20)
		}
		p.WriteAsdaString(g.MOTD, 293)
		return p
	}
	if receiver != nil {
		receiver.Send(build())
		return
	}
	sendToGuild(g, nil, func(target *Client) *PacketOut { return build() })
}

func sendClanFlagAndNameSelf(c *Client, g *guildState, member *db.GuildMemberData) {
	if c == nil || c.Char == nil || g == nil || member == nil {
		return
	}
	p := NewPacket(ClanFlagAndClanNameInfoSelf)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt32(c.Char.GuildPoints)
	p.WriteBytes(guildStab15)
	p.WriteInt16(int16(g.ID))
	p.WriteAsdaString(g.Name, 17)
	p.WriteInt32(int32(asdaGuildRank(member.RankIndex)))
	p.WriteUint8(3)
	p.WriteUint8(guildHasCrest(g))
	p.WriteBytes(guildCrestOrEmpty(g))
	p.WriteAsdaString(member.PublicNote, 60)
	p.WriteInt32(0)
	c.Send(p)
}

func sendCharacterInfoClanNameToArea(c *Client, g *guildState) {
	if c == nil || c.Char == nil || g == nil {
		return
	}
	for _, target := range World.AreaRecipients(c, true) {
		sendCharacterInfoClanName(target, c.Char, g)
	}
}

func sendCharacterInfoClanName(receiver *Client, chr *Character, g *guildState) {
	if receiver == nil || chr == nil || g == nil {
		return
	}
	p := NewPacket(CharacterInfoClanName)
	p.WriteUint32(chr.AccID)
	p.WriteInt16(int16(g.ID))
	p.WriteInt16(int16(g.Level))
	p.WriteAsdaString(g.Name, 16)
	p.WriteUint8(0)
	p.WriteUint8(3)
	p.WriteUint8(guildHasCrest(g))
	p.WriteBytes(guildCrestOrEmpty(g))
	receiver.Send(p)
}

func sendGuildNotification(g *guildState, status byte, member *db.GuildMemberData) {
	if g == nil || member == nil {
		return
	}
	var exclude uint32
	if status == guildNotificationJoined || status == guildNotificationLoggedIn {
		exclude = member.CharacterID
	}
	sendToGuild(g, map[uint32]struct{}{exclude: {}}, func(target *Client) *PacketOut {
		online := getClientByGUID(member.CharacterID)
		p := NewPacket(GuildNotification)
		p.WriteUint8(2)
		p.WriteUint8(status)
		p.WriteInt16(int16(g.ID))
		p.WriteUint32(member.AccountID)
		p.WriteUint8(member.CharNum)
		p.WriteAsdaStringLocale(member.Name, 20, target.Locale)
		p.WriteUint8(member.Level)
		prof, class, points, onlineFlag, mapID := guildMemberLiveFields(member, online)
		p.WriteUint8(prof)
		p.WriteUint8(class)
		p.WriteUint8(asdaGuildRank(member.RankIndex))
		p.WriteInt32(points)
		p.WriteUint8(onlineFlag)
		p.WriteAsdaStringLocale(formatGuildLastLogin(member.LastLogin), 17, LocaleTahadi)
		p.WriteUint8(onlineFlag)
		p.WriteUint8(mapID)
		p.WriteAsdaStringLocale(member.PublicNote, 60, target.Locale)
		p.WriteInt32(0)
		return p
	})
}

func sendGuildChat(g *guildState, sender *Client, msg string) {
	if g == nil || sender == nil || sender.Char == nil {
		return
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	sendToGuild(g, nil, func(target *Client) *PacketOut {
		p := NewPacket(GuildChatRes)
		p.WriteInt16(int16(g.ID))
		p.WriteUint32(sender.Char.AccID)
		p.WriteInt16(sender.Char.SessionID)
		p.WriteAsdaStringLocale(sender.Char.Name, 20, target.Locale)
		p.WriteCStringLocale(msg, maxChatMessageLen, target.Locale)
		return p
	})
}

func sendJoinMyGuildRequest(invitee *Client, inviter *Client, g *guildState) {
	if invitee == nil || inviter == nil || invitee.Char == nil || inviter.Char == nil || g == nil {
		return
	}
	p := NewPacket(JoinMyGuildRequest)
	p.WriteUint32(invitee.Char.AccID)
	p.WriteInt16(invitee.Char.SessionID)
	p.WriteUint32(inviter.Char.AccID)
	p.WriteInt16(inviter.Char.SessionID)
	p.WriteInt16(int16(g.ID))
	p.WriteAsdaStringLocale(g.Name, 17, invitee.Locale)
	invitee.Send(p)
}

func sendInviteGuildAccepted(invitee *Client, g *guildState) {
	if invitee == nil || invitee.Char == nil || g == nil {
		return
	}
	leader := g.leader()
	p := NewPacket(SendInviteGuiledResponse)
	p.WriteUint8(1)
	if leader != nil {
		p.WriteUint32(leader.AccountID)
	} else {
		p.WriteInt32(0)
	}
	p.WriteInt16(7)
	p.WriteUint32(invitee.Char.AccID)
	p.WriteInt16(invitee.Char.SessionID)
	p.WriteInt16(int16(g.ID))
	p.WriteAsdaStringLocale(g.Name, 17, invitee.Locale)
	invitee.Send(p)
}

func sendYouHaveLeftGuild(c *Client, ok bool) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(YouHaveLeftGuild)
	p.WriteUint8(boolByte(ok))
	p.WriteUint32(c.Char.AccID)
	c.Send(p)
}

func sendYouHaveBeenKicked(member *Client) {
	if member == nil || member.Char == nil {
		return
	}
	p := NewPacket(YouHaveBeenKickedFromGuild)
	p.WriteUint8(1)
	p.WriteUint32(member.Char.AccID)
	p.WriteUint32(member.Char.AccID)
	p.WriteAsdaStringLocale(member.Char.Name, 20, member.Locale)
	member.Send(p)
}

func sendChangeClanMemberRankResult(c *Client, status byte) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ChangeClanMemberRankAccessResult)
	p.WriteUint8(status)
	p.WriteUint32(c.Char.AccID)
	c.Send(p)
}

func sendPrivilegesChanged(c *Client, status byte, guildID uint32) {
	p := NewPacket(PrivilagiesChanged)
	p.WriteUint8(status)
	p.WriteInt16(int16(guildID))
	c.Send(p)
}

func sendAnnouncementEdited(c *Client, ok bool) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(AnnouncementEdited)
	p.WriteUint8(boolByte(ok))
	p.WriteUint32(c.Char.AccID)
	c.Send(p)
}

func sendGuildHistory(c *Client, g *guildState) {
	if c == nil || g == nil {
		return
	}
	p := NewPacket(GuildHistory)
	for i := 0; i < 12 && i < len(g.History); i++ {
		h := g.History[i]
		p.WriteInt16(int16(g.ID))
		p.WriteUint8(h.Type)
		p.WriteInt32(h.Value)
		p.WriteAsdaStringLocale(h.TriggerName, 20, c.Locale)
		p.WriteAsdaString(h.EventTime, 17)
	}
	c.Send(p)
	c.Send(NewPacket(GuildHistoryEnded))
}

func sendUpdateGuildPoints(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(UpdateGuildPoints)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt32(c.Char.GuildPoints)
	c.Send(p)
}

func sendGuildPointsDonated(c *Client, ok bool) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(GuildPointsDonated)
	p.WriteUint8(boolByte(ok))
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(int16(c.Char.GuildID))
	p.WriteInt32(c.Char.GuildPoints)
	c.Send(p)
}

func sendGuildLevelUp(c *Client, ok bool) {
	p := NewPacket(GuildLeveluped)
	p.WriteUint8(boolByte(ok))
	c.Send(p)
}

func sendClanSkillLearned(c *Client, g *guildState, skill *db.GuildSkillData, status byte, skillID int16) {
	if c == nil || c.Char == nil || g == nil {
		return
	}
	level := byte(1)
	if skill != nil {
		skillID = skill.SkillID
		level = skill.Level
	}
	p := NewPacket(ClanSkillLearned)
	p.WriteUint8(status)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(int16(g.ID))
	p.WriteInt16(skillID)
	p.WriteUint8(level)
	c.Send(p)
}

func sendGuildSkillStatusChanged(g *guildState, skill *db.GuildSkillData, status byte) {
	if g == nil || skill == nil {
		return
	}
	sendToGuild(g, nil, func(target *Client) *PacketOut {
		p := NewPacket(GuildSkillStatusChanged)
		p.WriteUint8(status)
		p.WriteInt16(int16(g.ID))
		p.WriteInt16(skill.SkillID)
		p.WriteUint8(skill.Level)
		p.WriteUint8(boolByte(skill.IsActivated))
		return p
	})
}

func sendGuildSkillActivated(c *Client, g *guildState, skill *db.GuildSkillData, status byte) {
	if c == nil || c.Char == nil || g == nil || skill == nil {
		return
	}
	p := NewPacket(GuildSkillActivated)
	p.WriteUint8(status)
	p.WriteUint32(c.Char.AccID)
	p.WriteInt16(int16(g.ID))
	p.WriteInt16(skill.SkillID)
	p.WriteUint8(skill.Level)
	c.Send(p)
}

func sendImpeachmentStatus(c *Client, status byte) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ImpeachmentStatus)
	p.WriteUint8(status)
	p.WriteInt16(int16(c.Char.GuildID))
	p.WriteUint32(c.Char.AccID)
	c.Send(p)
}

func sendImpeachmentAnswer(g *guildState, newLeaderName string) {
	if g == nil {
		return
	}
	leader := g.leader()
	sendToGuild(g, nil, func(target *Client) *PacketOut {
		p := NewPacket(ImpeachmentAnswer)
		p.WriteInt16(int16(g.ID))
		if leader != nil {
			p.WriteAsdaStringLocale(leader.Name, 20, target.Locale)
		} else {
			p.WriteAsdaString("", 20)
		}
		p.WriteAsdaStringLocale(newLeaderName, 20, target.Locale)
		return p
	})
}

func sendImpeachmentResult(g *guildState, ok bool) {
	if g == nil {
		return
	}
	leader := g.leader()
	sendToGuild(g, nil, func(target *Client) *PacketOut {
		p := NewPacket(ImpeachmentResult)
		p.WriteUint8(boolByte(ok))
		p.WriteInt16(int16(g.ID))
		if leader != nil {
			p.WriteAsdaStringLocale(leader.Name, 20, target.Locale)
		} else {
			p.WriteAsdaString("", 20)
		}
		return p
	})
}

func writeGuildMemberInfo(p *PacketOut, receiver *Client, g *guildState, member *db.GuildMemberData) {
	if member == nil {
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteAsdaString("", 20)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteAsdaString("", 17)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteAsdaString("", 60)
		p.WriteInt32(0)
		return
	}
	online := getClientByGUID(member.CharacterID)
	prof, class, points, onlineFlag, mapID := guildMemberLiveFields(member, online)
	p.WriteUint32(member.AccountID)
	p.WriteUint8(member.CharNum)
	p.WriteAsdaStringLocale(member.Name, 20, receiver.Locale)
	p.WriteUint8(member.Level)
	p.WriteUint8(prof)
	p.WriteUint8(class)
	p.WriteUint8(asdaGuildRank(member.RankIndex))
	p.WriteInt32(points)
	p.WriteUint8(onlineFlag)
	p.WriteAsdaStringLocale(formatGuildLastLogin(member.LastLogin), 17, LocaleTahadi)
	p.WriteUint8(onlineFlag)
	p.WriteUint8(mapID)
	p.WriteAsdaStringLocale(member.PublicNote, 60, receiver.Locale)
	p.WriteInt32(0)
}

func writeGuildSkills(p *PacketOut, g *guildState) {
	for i := int16(0); i < 10; i++ {
		skill := g.Skills[i]
		if skill == nil {
			p.WriteInt16(-1)
			p.WriteUint8(0)
			p.WriteUint8(0)
			continue
		}
		p.WriteInt16(skill.SkillID)
		p.WriteUint8(skill.Level)
		p.WriteUint8(boolByte(skill.IsActivated))
	}
}

func sendToGuild(g *guildState, exclude map[uint32]struct{}, build func(*Client) *PacketOut) {
	if g == nil || build == nil {
		return
	}
	for _, member := range g.Members {
		if member == nil {
			continue
		}
		if _, skip := exclude[member.CharacterID]; skip && member.CharacterID != 0 {
			continue
		}
		target := getClientByGUID(member.CharacterID)
		if target == nil || target.Char == nil {
			continue
		}
		target.Send(build(target))
	}
}

func guildMemberLiveFields(member *db.GuildMemberData, online *Client) (byte, byte, int32, byte, byte) {
	if online != nil && online.Char != nil {
		return online.Char.ProfessionLevel, online.Char.Class, online.Char.GuildPoints, 1, byte(online.Char.MapID)
	}
	return member.ProfessionLevel, member.Class, 0, 0, 0
}

func guildRankPrivileges(g *guildState, rankIndex byte) uint16 {
	if g == nil {
		return 0
	}
	if rankIndex == 0 {
		return guildPrivilegeAll
	}
	return g.Ranks[rankIndex].Privileges
}

func guildHasCrest(g *guildState) byte {
	if g != nil && len(g.Crest) > 0 && g.Crest[0] != 0 {
		return 1
	}
	return 0
}

func guildCrestOrEmpty(g *guildState) []byte {
	if guildHasCrest(g) == 1 {
		return normalizeCrest(g.Crest)
	}
	return guildFF40
}

func boolByte(ok bool) byte {
	if ok {
		return 1
	}
	return 0
}

func formatGuildLastLogin(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func formatGuildNoticeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func sortGuildMembers(members []*db.GuildMemberData) {
	sort.Slice(members, func(i, j int) bool {
		if members[i].RankIndex != members[j].RankIndex {
			return members[i].RankIndex < members[j].RankIndex
		}
		return strings.ToLower(members[i].Name) < strings.ToLower(members[j].Name)
	})
}
