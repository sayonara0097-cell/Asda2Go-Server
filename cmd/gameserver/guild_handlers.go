package main

import (
	"errors"
	"log"
	"strings"
	"time"

	"asda2/shared/db"
	"asda2/shared/types"
)

func handleCreateGuild(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	name := strings.TrimSpace(readGuildName(c, p, 17))
	if c.Char.FactionID == -1 || c.Char.FactionID == 2 {
		sendGuildCreated(c, guildCreateBadFaction, nil)
		return
	}
	if c.Char.GuildID != 0 {
		sendGuildCreated(c, guildCreateInGuild, nil)
		return
	}
	if !isValidGuildName(name) {
		sendGuildCreated(c, guildCreateBadName, nil)
		return
	}
	exists, err := db.GuildNameExists(name)
	if err != nil {
		log.Printf("[Guild] name check failed: %v", err)
		sendGuildCreated(c, guildCreateFailed, nil)
		return
	}
	if exists {
		sendGuildCreated(c, guildCreateNameExists, nil)
		return
	}
	if c.Char.Level < 10 {
		sendGuildCreated(c, guildCreateNeedLevel10, nil)
		return
	}
	if c.Char.Gold < 10000 {
		sendGuildCreated(c, guildCreateNoMoney, nil)
		return
	}

	c.Char.Gold -= 10000
	g, err := guildRuntime.createGuild(name, c)
	if err != nil {
		c.Char.Gold += 10000
		if errors.Is(err, db.ErrGuildNameExists) {
			sendGuildCreated(c, guildCreateNameExists, nil)
			return
		}
		if errors.Is(err, db.ErrGuildCharacterAlreadyMember) {
			sendGuildCreated(c, guildCreateInGuild, nil)
			return
		}
		log.Printf("[Guild] create failed leader=%s name=%q: %v", c.Char.Name, name, err)
		sendGuildCreated(c, guildCreateFailed, nil)
		return
	}
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Guild] save leader after create failed: %v", err)
	}
	member := g.member(c.Char.GUID)
	sendGuildCreated(c, guildCreateOK, g)
	sendUpdateGuildInfo(g, guildInfoSilent, nil)
	sendClanFlagAndNameSelf(c, g, member)
	sendCharacterInfoClanNameToArea(c, g)
}

func handleDisolveGuild(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok || member.RankIndex != 0 || member.CharacterID != g.LeaderCharacterID {
		sendDisolveGuild(c, false)
		return
	}
	if len(g.Members) > 1 {
		sendDisolveGuild(c, false)
		return
	}
	if err := guildRuntime.deleteGuild(g); err != nil {
		log.Printf("[Guild] disband failed guild=%d: %v", g.ID, err)
		sendDisolveGuild(c, false)
		return
	}
	sendDisolveGuild(c, true)
	sendCharacterInfoClanNameToArea(c, g)
}

func handleSendInviteGuild(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		return
	}
	if !g.hasPrivilege(member, guildPrivilegeInviteMembers) {
		sendSystemGlobalChatResponse(c, announcementSender, "You do not have permission to invite guild members.")
		return
	}
	if len(g.Members) >= int(g.MaxMembers) {
		sendSystemGlobalChatResponse(c, announcementSender, "Your guild roster is full.")
		return
	}
	targetAccID := readGuildInviteTargetAccID(p)
	target := getClientByAccID(targetAccID)
	if target == nil || target.Char == nil || target == c {
		return
	}
	if target.Char.GuildID != 0 {
		sendSystemGlobalChatResponse(c, announcementSender, target.Char.Name+" is already in a guild.")
		return
	}
	if c.Char.FactionID != -1 && target.Char.FactionID != -1 && c.Char.FactionID != target.Char.FactionID {
		sendSystemGlobalChatResponse(c, announcementSender, "You must invite a character from your faction.")
		return
	}
	guildRuntime.addInvite(c, target, g.ID)
	sendJoinMyGuildRequest(target, c, g)
}

func handleJoinGuildRequest(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	accepted := readGuildInviteAnswer(p)
	inviter, g := guildRuntime.consumeInvite(c)
	if inviter == nil || inviter.Char == nil || g == nil {
		return
	}
	if !accepted {
		sendSystemGlobalChatResponse(inviter, announcementSender, c.Char.Name+" declined your guild invitation.")
		return
	}
	if c.Char.GuildID != 0 || len(g.Members) >= int(g.MaxMembers) {
		return
	}
	member, err := guildRuntime.addMember(g, c)
	if err != nil {
		log.Printf("[Guild] add member failed guild=%d char=%d: %v", g.ID, c.Char.GUID, err)
		return
	}
	refreshed, _, _ := guildRuntime.guildForCharacter(c.Char)
	if refreshed != nil {
		g = refreshed
	}
	sendSystemGlobalChatResponse(inviter, announcementSender, c.Char.Name+" accepted your guild invitation.")
	sendInviteGuildAccepted(c, g)
	sendClanFlagAndNameSelf(c, g, member)
	sendGuildInfoOnLogin(c, g)
	sendGuildSkillsInfo(c, g)
	sendGuildMembersInfo(c, g)
	sendGuildNotification(g, guildNotificationJoined, member)
	sendCharacterInfoClanNameToArea(c, g)
}

func handleLeaveGuild(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		sendYouHaveLeftGuild(c, false)
		return
	}
	sendYouHaveLeftGuild(c, true)
	if member.RankIndex == 0 || member.CharacterID == g.LeaderCharacterID {
		if err := guildRuntime.deleteGuild(g); err != nil {
			log.Printf("[Guild] leader leave disband failed guild=%d: %v", g.ID, err)
		}
		sendCharacterInfoClanNameToArea(c, g)
		return
	}
	sendGuildNotification(g, guildNotificationLeft, member)
	if err := guildRuntime.removeMember(g, member, false); err != nil {
		log.Printf("[Guild] leave failed guild=%d char=%d: %v", g.ID, member.CharacterID, err)
	}
	sendCharacterInfoClanNameToArea(c, g)
}

func handleKickFromGuild(c *Client, p *PacketIn) {
	g, actor, ok := requireGuild(c)
	if !ok {
		return
	}
	if !g.hasPrivilege(actor, guildPrivilegeEditRankSettings) {
		sendSystemGlobalChatResponse(c, announcementSender, "You do not have permission to kick guild members.")
		return
	}
	targetAccID, targetCharNum := readGuildMemberTarget(p)
	target := g.memberByAccountChar(targetAccID, targetCharNum)
	if target == nil || target.RankIndex == 0 {
		return
	}
	if asdaGuildRank(target.RankIndex) > asdaGuildRank(actor.RankIndex) {
		sendSystemGlobalChatResponse(c, announcementSender, "You cannot kick a higher-rank guild member.")
		return
	}
	if online := getClientByGUID(target.CharacterID); online != nil {
		sendYouHaveBeenKicked(online)
	}
	sendGuildNotification(g, guildNotificationKicked, target)
	if err := guildRuntime.removeMember(g, target, true); err != nil {
		log.Printf("[Guild] kick failed guild=%d target=%d: %v", g.ID, target.CharacterID, err)
	}
}

func handleSetPrivilegies(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		sendPrivilegesChanged(c, guildPrivilegeChangeFail, 0)
		return
	}
	if !g.hasPrivilege(member, guildPrivilegeEditRankSettings) {
		sendPrivilegesChanged(c, guildPrivilegeChangeNoPermission, g.ID)
		return
	}
	privileges, ok := readGuildPrivileges(p)
	if !ok {
		sendPrivilegesChanged(c, guildPrivilegeChangeFail, g.ID)
		return
	}
	for rankIndex, privilege := range privileges {
		rank := g.Ranks[rankIndex]
		rank.Privileges = privilege
		g.Ranks[rankIndex] = rank
	}
	if err := db.UpdateGuildRanks(g.ID, privileges); err != nil {
		log.Printf("[Guild] update privileges failed guild=%d: %v", g.ID, err)
		sendPrivilegesChanged(c, guildPrivilegeChangeFail, g.ID)
		return
	}
	sendUpdateGuildInfo(g, guildInfoPrivilegesChanged, nil)
	sendPrivilegesChanged(c, guildPrivilegeChangeOK, g.ID)
}

func handleChangeClanMemberRank(c *Client, p *PacketIn) {
	g, actor, ok := requireGuild(c)
	if !ok {
		sendChangeClanMemberRankResult(c, guildRankChangeNoPermission)
		return
	}
	asdaRank, targetAccID, targetCharNum, ok := readGuildRankChange(p)
	if !ok || asdaRank > 3 {
		sendChangeClanMemberRankResult(c, guildRankChangeBadRank)
		return
	}
	target := g.memberByAccountChar(targetAccID, byte(targetCharNum))
	if target == nil {
		sendChangeClanMemberRankResult(c, guildRankChangeOK)
		return
	}
	if asdaGuildRank(target.RankIndex) > asdaGuildRank(actor.RankIndex) || asdaRank > asdaGuildRank(actor.RankIndex) {
		sendChangeClanMemberRankResult(c, guildRankChangeHigherRank)
		return
	}
	if !g.hasPrivilege(actor, guildPrivilegeSetMember) {
		sendChangeClanMemberRankResult(c, guildRankChangeNoPermission)
		return
	}
	target.RankIndex = rankIndexFromAsda(asdaRank)
	if err := db.UpdateGuildMemberRank(g.ID, target.CharacterID, target.RankIndex); err != nil {
		log.Printf("[Guild] rank change failed guild=%d target=%d: %v", g.ID, target.CharacterID, err)
		sendChangeClanMemberRankResult(c, guildRankChangeBadRank)
		return
	}
	if online := getClientByGUID(target.CharacterID); online != nil && online.Char != nil {
		online.Char.GuildRank = asdaGuildRank(target.RankIndex)
	}
	sendChangeClanMemberRankResult(c, guildRankChangeOK)
	sendGuildNotification(g, guildNotificationRankChanged, target)
	sendGuildMembersInfo(c, g)
}

func handleRegisterGuildCrest(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok || !g.hasPrivilege(member, guildPrivilegeEditCrest) {
		return
	}
	slot, crest, ok := readGuildCrestRequest(p)
	if !ok {
		return
	}
	item := findItem(c.Char, types.InventoryShop, slot)
	if item == nil || itemTemplateByID(item.ItemID).Category != types.ItemCategoryGuildCrest {
		return
	}
	if err := removeCharacterItem(c.Char, item, 1); err != nil {
		log.Printf("[Guild] consume crest item failed: %v", err)
		return
	}
	g.Crest = normalizeCrest(crest)
	if err := db.UpdateGuildCrest(g.ID, g.Crest); err != nil {
		log.Printf("[Guild] crest update failed guild=%d: %v", g.ID, err)
		return
	}
	sendUpdateGuildInfo(g, guildInfoCrestChanged, nil)
	for _, guildMember := range g.Members {
		if online := getClientByGUID(guildMember.CharacterID); online != nil && online.Char != nil {
			sendCharacterInfoClanNameToArea(online, g)
		}
	}
}

func handleEditClanNotice(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		return
	}
	if !g.hasPrivilege(member, guildPrivilegeEditAnnounce) {
		sendAnnouncementEdited(c, false)
		return
	}
	notice := readGuildNotice(c, p)
	g.MOTD = notice
	g.NoticeWriter = c.Char.Name
	g.NoticeTime = time.Now()
	if err := db.UpdateGuildMOTD(g.ID, notice, c.Char.Name); err != nil {
		log.Printf("[Guild] notice update failed guild=%d: %v", g.ID, err)
		sendAnnouncementEdited(c, false)
		return
	}
	sendUpdateGuildInfo(g, guildInfoAnnouncement, nil)
	sendAnnouncementEdited(c, true)
}

func handleGuildChatReq(c *Client, p *PacketIn) {
	g, _, ok := requireGuild(c)
	if !ok || c.Char.ChatBanned {
		return
	}
	msg := strings.TrimSpace(readGuildChat(c, p))
	if msg == "" || len(msg) > maxChatMessageLen {
		return
	}
	sendGuildChat(g, c, msg)
}

func handleAskForGuildHistory(c *Client, p *PacketIn) {
	g, _, ok := requireGuild(c)
	if !ok {
		return
	}
	history, err := db.LoadGuildHistory(g.ID, 12)
	if err == nil {
		g.History = history
	}
	sendGuildHistory(c, g)
}

func handleReqUpdateGuildPoints(c *Client, p *PacketIn) {
	sendUpdateGuildPoints(c)
}

func handleDonateGuildPoints(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		sendGuildPointsDonated(c, false)
		return
	}
	points := readGuildPointDonation(p)
	if points < 0 || c.Char.GuildPoints < points {
		sendGuildPointsDonated(c, false)
		return
	}
	c.Char.GuildPoints -= points
	member.GuildPoints = c.Char.GuildPoints
	g.Points += points
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Guild] save donor points failed: %v", err)
	}
	if err := db.UpdateGuildPoints(g.ID, g.Points); err != nil {
		log.Printf("[Guild] update guild points failed guild=%d: %v", g.ID, err)
	}
	_ = db.AddGuildHistory(g.ID, guildHistoryDonatedPoints, points, c.Char.Name, time.Now().Format("15:04:05"))
	sendUpdateGuildInfo(g, guildInfoSilent, nil)
	sendGuildPointsDonated(c, true)
	sendGuildNotification(g, guildNotificationDonated, member)
}

func handleLevelUpGuild(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok || !g.hasPrivilege(member, guildPrivilegeUsePoints) {
		sendGuildLevelUp(c, false)
		return
	}
	level := int(g.Level)
	if level <= 0 || level >= len(guildLevelUpCosts) {
		sendGuildLevelUp(c, false)
		return
	}
	cost := guildLevelUpCosts[level]
	if g.Points < cost {
		sendGuildLevelUp(c, false)
		return
	}
	g.Points -= cost
	g.Level++
	if err := db.UpdateGuildLevelAndPoints(g.ID, g.Level, g.Points); err != nil {
		log.Printf("[Guild] level update failed guild=%d: %v", g.ID, err)
		sendGuildLevelUp(c, false)
		return
	}
	_ = db.AddGuildHistory(g.ID, guildHistoryLevelNowIs, int32(g.Level), "system", time.Now().Format("15:04:05"))
	sendUpdateGuildInfo(g, guildInfoGuildLevelChanged, nil)
	sendGuildLevelUp(c, true)
	sendClanFlagAndNameSelf(c, g, member)
	for _, guildMember := range g.Members {
		if online := getClientByGUID(guildMember.CharacterID); online != nil && online.Char != nil {
			sendCharacterInfoClanNameToArea(online, g)
		}
	}
}

func handleLearnClanSkill(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	skillID := readGuildSkillID(p)
	if !ok {
		return
	}
	if !g.hasPrivilege(member, guildPrivilegeUsePoints) {
		sendClanSkillLearned(c, g, nil, guildSkillLearnNoPermission, skillID)
		return
	}
	if skillID < 0 || skillID > 4 {
		sendClanSkillLearned(c, g, nil, guildSkillLearnSkillProblem, skillID)
		return
	}
	skill, status := learnGuildSkill(g, skillID)
	sendClanSkillLearned(c, g, skill, status, skillID)
	if status == guildSkillLearnOK {
		sendGuildSkillStatusChanged(g, skill, guildSkillStatusLearned)
		sendGuildSkillsInfoToGuild(g)
		sendUpdateGuildInfo(g, guildInfoSilent, nil)
	}
}

func handleActivateGuildSkill(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		return
	}
	if !g.hasPrivilege(member, guildPrivilegeUsePoints) {
		sendPrivilegesChanged(c, guildPrivilegeChangeNoPermission, g.ID)
		return
	}
	skillID := readGuildSkillID(p)
	skill := g.Skills[skillID]
	if skillID < 0 || skillID > 4 || skill == nil {
		return
	}
	status := toggleGuildSkill(g, skill)
	sendGuildSkillActivated(c, g, skill, status)
	if status == guildSkillActivatedOK {
		applyGuildSkillToOnlineMembers(g, skill)
		sendGuildSkillStatusChanged(g, skill, guildSkillStatusActivation)
		sendUpdateGuildInfo(g, guildInfoSilent, nil)
		sendGuildSkillsInfoToGuild(g)
	}
}

func handleImpeachmentRequest(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok || asdaGuildRank(member.RankIndex) != 3 {
		sendImpeachmentStatus(c, guildImpeachmentFailed)
		return
	}
	status := guildRuntime.startImpeachment(g, member)
	sendImpeachmentStatus(c, status)
	if status == guildImpeachmentSuccess {
		sendImpeachmentAnswer(g, member.Name)
	}
}

func handleImpeachmentAnswer(c *Client, p *PacketIn) {
	// The reference sends ImpeachmentAnswer from server to guild members.
}

func handleImpeachmentVote(c *Client, p *PacketIn) {
	g, member, ok := requireGuild(c)
	if !ok {
		return
	}
	if readGuildImpeachmentVote(p) {
		guildRuntime.addImpeachmentVote(g.ID, member)
	}
}
