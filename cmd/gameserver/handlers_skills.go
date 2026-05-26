package main

// ---- Skills ----

func handleUseSkill(c *Client, p *PacketIn) {
	// Mirrors Asda2SpellHandler.UseSkillRequest
	if c.Char == nil {
		return
	}
	if p.Remaining() < 10 {
		return
	}
	skillID := p.ReadInt16()
	p.Skip(1)
	_ = p.ReadInt16() // unk1
	_ = p.ReadInt16() // unk2
	targetType := p.ReadUint8()
	targetID := p.ReadUint16()

	result := useSkillOnMonster(c, skillID, targetType, targetID)
	if result != skillUseOK {
		sendUseSkillResult(c, skillID, result)
	}
}

func handleLearnSkill(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 2 {
		return
	}
	req, ok := readLearnSkillRequest(c.Char, p.Data)
	if !ok {
		debugNpcInteractionf("rejected trainer learn char=%q reason=bad-payload payload=% X",
			c.Char.Name, limitedPacketBytes(p.Data))
		sendSkillLearnedResponse(c, skillLearnFail, 0, 0)
		return
	}
	skillID := req.skillID
	level := req.level
	if req.offset > 0 {
		debugNpcInteractionf("parsed trainer learn char=%q skill=%d level=%d offset=%d",
			c.Char.Name, skillID, level, req.offset)
	}

	if status := trainerLearnStatus(c, skillID); status != skillLearnOK {
		sendSkillLearnedResponse(c, status, 0, 0)
		return
	}

	skill, status := learnRuntimeSkill(c, skillID, level)
	if status != skillLearnOK {
		sendSkillLearnedResponse(c, status, 0, 0)
		return
	}

	sendSkillLearnedFirstTime(c, skill)
	sendSkillLearnedResponse(c, skillLearnOK, skill.ID, level)
	sendSkillsInfo(c)
}

func handleCancelSkill(c *Client, p *PacketIn) {
	if p.Remaining() < 3 {
		return
	}
	p.Skip(1)
	skillID := p.ReadInt16()
	if !removeSkillBuff(c, skillID, true) {
		sendBuffEndedForCharacter(c, skillID)
	}
}

func handleUseSoulGuardSkill(c *Client, p *PacketIn) {
	if c.Char == nil || p.Remaining() < 10 {
		return
	}
	skillID := p.ReadInt16()
	p.Skip(1)
	_ = p.ReadInt16()
	_ = p.ReadInt16()
	targetType := p.ReadUint8()
	targetID := p.ReadUint16()

	result := useRuntimeSkill(c, skillID, targetType, targetID, runtimeSkillOptions{soulGuard: true})
	if result != skillUseOK {
		sendUseSkillResult(c, skillID, result)
		refreshSoulGuard(c, false, 0)
	}
}

func handleUseSoulmateSkill(c *Client, p *PacketIn) {
	useSoulmateSkill(c, p)
}
