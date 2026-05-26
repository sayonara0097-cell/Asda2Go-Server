package main

import (
	"log"
	"strings"
	"time"

	"asda2/shared/db"
)

func sendDisolveGuild(c *Client, ok bool) {
	p := NewPacket(GuildDisolved)
	p.WriteUint8(boolByte(ok))
	c.Send(p)
}

func sendGuildSkillsInfoToGuild(g *guildState) {
	sendToGuild(g, nil, func(target *Client) *PacketOut {
		p := NewPacket(GuildSkillsInfo)
		p.WriteUint32(target.Char.AccID)
		writeGuildSkills(p, g)
		return p
	})
}

func sendGuildLoginInfo(c *Client) {
	g, member, ok := requireGuild(c)
	if !ok {
		return
	}
	recalculateGuildSkillBonuses(c.Char, g)
	sendCharacterInfoClanName(c, c.Char, g)
	sendClanFlagAndNameSelf(c, g, member)
	sendGuildInfoOnLogin(c, g)
	sendGuildSkillsInfo(c, g)
	sendGuildNotification(g, guildNotificationLoggedIn, member)
	time.AfterFunc(2*time.Second, func() {
		if c != nil && c.Char != nil && c.Char.GuildID == g.ID {
			sendGuildMembersInfo(c, g)
		}
	})
}

func learnGuildSkill(g *guildState, skillID int16) (*db.GuildSkillData, byte) {
	template, ok := guildSkillTemplates[skillID]
	if !ok {
		return nil, guildSkillLearnSkillProblem
	}
	skill := g.Skills[skillID]
	if skill != nil {
		if skill.Level >= template.MaxLevel {
			return skill, guildSkillLearnMaxLevel
		}
		if skill.IsActivated {
			return skill, guildSkillLearnActivated
		}
		nextLevel := int(skill.Level + 1)
		if nextLevel >= len(template.LearnCosts) || g.Points < template.LearnCosts[nextLevel] {
			return skill, guildSkillLearnNoPoints
		}
		g.Points -= template.LearnCosts[nextLevel]
		skill.Level++
	} else {
		if len(template.LearnCosts) <= 1 || g.Points < template.LearnCosts[1] {
			return nil, guildSkillLearnNoPoints
		}
		g.Points -= template.LearnCosts[1]
		skill = &db.GuildSkillData{GuildID: g.ID, SkillID: skillID, Level: 1, LastMaintenance: time.Now()}
		g.Skills[skillID] = skill
	}
	if err := db.SaveGuildSkill(*skill); err != nil {
		log.Printf("[Guild] save skill failed guild=%d skill=%d: %v", g.ID, skillID, err)
	}
	if err := db.UpdateGuildPoints(g.ID, g.Points); err != nil {
		log.Printf("[Guild] update points after skill failed guild=%d: %v", g.ID, err)
	}
	_ = db.AddGuildHistory(g.ID, guildHistoryUsedPoints, 0, "system", time.Now().Format("15:04:05"))
	return skill, guildSkillLearnOK
}

func toggleGuildSkill(g *guildState, skill *db.GuildSkillData) byte {
	if g == nil || skill == nil {
		return guildSkillActivatedFail
	}
	if skill.IsActivated {
		skill.IsActivated = false
	} else {
		template := guildSkillTemplates[skill.SkillID]
		level := int(skill.Level)
		if level >= len(template.ActivationCosts) || g.Points < template.ActivationCosts[level] {
			return guildSkillActivatedNoPoints
		}
		g.Points -= template.ActivationCosts[level]
		skill.IsActivated = true
		skill.LastMaintenance = time.Now()
		_ = db.UpdateGuildPoints(g.ID, g.Points)
	}
	if err := db.SaveGuildSkill(*skill); err != nil {
		log.Printf("[Guild] save activation failed guild=%d skill=%d: %v", g.ID, skill.SkillID, err)
	}
	return guildSkillActivatedOK
}

func applyGuildSkillToOnlineMembers(g *guildState, skill *db.GuildSkillData) {
	if g == nil || skill == nil {
		return
	}
	for _, member := range g.Members {
		online := getClientByGUID(member.CharacterID)
		if online == nil || online.Char == nil {
			continue
		}
		recalculateGuildSkillBonuses(online.Char, g)
		sendUpdateStats(online)
		sendUpdateStatsOne(online)
	}
}

func recalculateGuildSkillBonuses(chr *Character, g *guildState) {
	if chr == nil {
		return
	}
	removeGuildSkillBonuses(chr)
	if g == nil {
		return
	}
	for _, skill := range g.Skills {
		if skill == nil || !skill.IsActivated {
			continue
		}
		value := guildSkillBonusValue(g, skill)
		switch skill.SkillID {
		case 0:
			chr.GuildSkillDamageBonusPct += value
			chr.GuildSkillMagicDamageBonusPct += value
		case 1:
			chr.GuildSkillDefenseBonusPct += value
			chr.GuildSkillMagicDefenseBonusPct += value
		case 2:
			chr.GuildSkillSpeedBonusPct += value
		}
	}
	applyGuildSkillBonuses(chr)
}

func removeGuildSkillBonuses(chr *Character) {
	chr.SkillDamageBonusPct -= chr.GuildSkillDamageBonusPct
	chr.SkillMagicDamageBonusPct -= chr.GuildSkillMagicDamageBonusPct
	chr.SkillDefenseBonusPct -= chr.GuildSkillDefenseBonusPct
	chr.SkillMagicDefenseBonusPct -= chr.GuildSkillMagicDefenseBonusPct
	chr.SkillSpeedBonusPct -= chr.GuildSkillSpeedBonusPct
	chr.GuildSkillDamageBonusPct = 0
	chr.GuildSkillMagicDamageBonusPct = 0
	chr.GuildSkillDefenseBonusPct = 0
	chr.GuildSkillMagicDefenseBonusPct = 0
	chr.GuildSkillSpeedBonusPct = 0
}

func applyGuildSkillBonuses(chr *Character) {
	chr.SkillDamageBonusPct += chr.GuildSkillDamageBonusPct
	chr.SkillMagicDamageBonusPct += chr.GuildSkillMagicDamageBonusPct
	chr.SkillDefenseBonusPct += chr.GuildSkillDefenseBonusPct
	chr.SkillMagicDefenseBonusPct += chr.GuildSkillMagicDefenseBonusPct
	chr.SkillSpeedBonusPct += chr.GuildSkillSpeedBonusPct
}

func guildSkillBonusValue(g *guildState, skill *db.GuildSkillData) float32 {
	if g == nil || skill == nil {
		return 0
	}
	bonuses := map[int16][]float32{
		0: {0, 0.03, 0.04, 0.05, 0.06, 0.07, 0.08, 0.10},
		1: {0, 0.03, 0.04, 0.05, 0.06, 0.07, 0.08, 0.10},
		2: {0, 0.03, 0.04, 0.05, 0.06, 0.07, 0.08, 0.10},
		3: {0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 0.07},
		4: {0, 0.05, 0.06, 0.07, 0.10},
	}
	values := bonuses[skill.SkillID]
	level := int(skill.Level)
	if level >= len(values) {
		level = len(values) - 1
	}
	if level < 0 || len(values) == 0 {
		return 0
	}
	return values[level]
}

func requireGuild(c *Client) (*guildState, *db.GuildMemberData, bool) {
	if c == nil || c.Char == nil {
		return nil, nil, false
	}
	g, member, err := guildRuntime.guildForCharacter(c.Char)
	if err != nil {
		log.Printf("[Guild] load failed char=%d: %v", c.Char.GUID, err)
		return nil, nil, false
	}
	return g, member, g != nil && member != nil
}

func isValidGuildName(name string) bool {
	runes := []rune(name)
	if len(runes) < 3 || len(runes) > 17 {
		return false
	}
	for _, ch := range runes {
		if ch == ' ' || ch == '.' {
			return false
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		if !isAllowedArabicNameRune(ch) {
			return false
		}
	}
	return true
}

func isAllowedArabicNameRune(ch rune) bool {
	switch ch {
	case 'ض', 'ص', 'ث', 'ق', 'ف', 'غ', 'ع', 'ه', 'خ', 'ح', 'ج', 'د', 'ش', 'س',
		'ي', 'ب', 'ل', 'ا', 'ت', 'ن', 'م', 'ك', 'ط', 'ذ', 'ئ', 'ء', 'ؤ', 'ر', 'ى',
		'ة', 'و', 'ز', 'ظ', 'إ', 'پ', 'چ', 'ژ', 'گ', 'ک':
		return true
	default:
		return false
	}
}

func readGuildName(c *Client, p *PacketIn, maxLen int) string {
	if p == nil {
		return ""
	}
	p.Seek(0)
	return p.ReadAsdaStringLocale(maxLen, c.Locale)
}

func readGuildInviteTargetAccID(p *PacketIn) uint32 {
	if p == nil || len(p.Data) < 4 {
		return 0
	}
	p.Seek(0)
	return p.ReadUint32()
}

func readGuildInviteAnswer(p *PacketIn) bool {
	if p == nil || len(p.Data) == 0 {
		return false
	}
	if len(p.Data) > 16 {
		return p.Data[16] == 1
	}
	return p.Data[0] == 1
}

func readGuildMemberTarget(p *PacketIn) (uint32, byte) {
	if p == nil || len(p.Data) < 7 {
		return 0, 0
	}
	p.Seek(2)
	accID := p.ReadUint32()
	charNum := p.ReadUint8()
	return accID, charNum
}

func readGuildRankChange(p *PacketIn) (byte, uint32, int16, bool) {
	if p == nil || len(p.Data) < 10 {
		return 0, 0, 0, false
	}
	p.Seek(2)
	rank := p.ReadUint8()
	p.Skip(1)
	accID := p.ReadUint32()
	charNum := p.ReadInt16()
	return rank, accID, charNum, true
}

func readGuildPrivileges(p *PacketIn) (map[byte]uint16, bool) {
	if p == nil || len(p.Data) < 10 {
		return nil, false
	}
	p.Seek(2)
	return map[byte]uint16{
		4: p.ReadUint16(),
		3: p.ReadUint16(),
		2: p.ReadUint16(),
		1: p.ReadUint16(),
	}, true
}

func readGuildCrestRequest(p *PacketIn) (int16, []byte, bool) {
	if p == nil || len(p.Data) < 49 {
		return 0, nil, false
	}
	p.Seek(6)
	slot := p.ReadInt16()
	p.Skip(1)
	if p.Remaining() < db.GuildCrestLength {
		return 0, nil, false
	}
	crest := make([]byte, db.GuildCrestLength)
	for i := range crest {
		crest[i] = p.ReadUint8()
	}
	return slot, crest, true
}

func readGuildNotice(c *Client, p *PacketIn) string {
	if p == nil || len(p.Data) <= 2 {
		return ""
	}
	p.Seek(2)
	return strings.TrimSpace(p.ReadAsdaStringLocale(260, c.Locale))
}

func readGuildChat(c *Client, p *PacketIn) string {
	if p == nil {
		return ""
	}
	p.Seek(0)
	return p.ReadCStringLocale(maxChatMessageLen, c.Locale)
}

func readGuildPointDonation(p *PacketIn) int32 {
	if p == nil || len(p.Data) < 6 {
		return -1
	}
	p.Seek(2)
	return p.ReadInt32()
}

func readGuildSkillID(p *PacketIn) int16 {
	if p == nil || len(p.Data) < 4 {
		return -1
	}
	p.Seek(2)
	return p.ReadInt16()
}

func readGuildImpeachmentVote(p *PacketIn) bool {
	if p == nil || len(p.Data) < 3 {
		return false
	}
	p.Seek(2)
	return p.ReadUint8() != 0
}
