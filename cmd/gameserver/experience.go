package main

import "log"

const maxAsda2Level = 95

var asda2StartXPByLevel = [...]int64{
	0, 40, 150, 401, 865, 1615, 2725, 4270, 6326, 8970,
	12293, 16387, 21349, 27870, 35564, 44546, 54932, 66842, 80398, 95726,
	113034, 132539, 154464, 179039, 206503, 237101, 271086, 308718, 350264, 400572,
	461713, 534854, 620995, 724136, 847277, 995418, 1173559, 1386700, 1639841, 1952982,
	2336123, 2799264, 3352405, 4005546, 4808687, 5811828, 7064969, 8618110, 10494105, 13260405,
	17106653, 21502900, 26449148, 31945395, 37991643, 44196133, 51637984, 59611296, 68135476, 77217132,
	86864452, 97085680, 107889116, 119283112, 131276078, 143876478, 157092830, 170933707, 185407740, 200523610,
	217151067, 235441269, 255560491, 277691636, 302035895, 328814580, 358271134, 390673343, 426315773, 465522446,
	508649787, 556089862, 608273944, 665676435, 728819175, 798276189, 874678904, 958721891, 1051169177, 1152861191,
	1269807007, 1404294696, 1558955538, 1736815507, 1941361570, 2000000000,
}

var asda2XPToNextLevel = [...]int64{
	0, 40, 110, 251, 464, 750, 1110, 1545, 2056, 2644,
	3323, 4094, 4962, 6521, 7694, 8982, 10386, 11910, 13556, 15328,
	17308, 19505, 21925, 24575, 27464, 30598, 33985, 37632, 41546, 50308,
	61141, 73141, 86141, 103141, 123141, 148141, 178141, 213141, 253141, 313141,
	383141, 463141, 553141, 653141, 803141, 1003141, 1253141, 1553141, 1875995, 2766300,
	3846248, 4396247, 4946248, 5496247, 6046248, 6204490, 7441851, 7973312, 8524180, 9081656,
	9647320, 10221228, 10803436, 11393996, 11992966, 12600400, 13216352, 13840877, 14474033, 15115870,
	16627457, 18290202, 20119222, 22131145, 24344259, 26778685, 29456554, 32402209, 35642430, 39206673,
	43127341, 47440075, 52184082, 57402491, 63142740, 69457014, 76402715, 84042987, 92447286, 101692014,
	116945816, 134487689, 154660842, 177859969, 204538964, 2147483647,
}

var asda2BaseMonsterXPByLevel = [...]int64{
	0, 2, 5, 9, 15, 19, 23, 26, 30, 34,
	37, 41, 46, 55, 60, 65, 70, 75, 80, 86,
	92, 98, 105, 112, 120, 128, 136, 145, 154, 168,
	186, 204, 221, 246, 274, 309, 350, 395, 445, 522,
	609, 702, 802, 908, 1071, 1287, 1548, 1849, 2157, 3007,
	3966, 4311, 4623, 4908, 5168, 5086, 5860, 6041, 6223, 6396,
	6563, 6725, 6882, 7034, 7182, 7326, 7467, 7605, 7741, 7558,
	7807, 8094, 8419, 8783, 9187, 9633, 10123, 10659, 11244, 11881,
	12574, 13326, 14143, 15027, 15986, 17024, 18148, 19365, 20682, 21637,
	23722, 26064, 28695, 31648, 34964,
}

// ---- Experience ----

func grantMonsterKillExp(c *Client, monster *Monster) {
	if c == nil || c.Char == nil || monster == nil {
		return
	}

	xp := monsterKillExp(c.Char, monster)
	if xp <= 0 {
		return
	}

	c.Char.Exp += xp
	sendExpGained(c, monster, xp, true)

	oldLevel := c.Char.Level
	leveled := tryLevelUp(c)
	if leveled {
		autoAdvanceProfessionForLevel(c)
		sendLevelUpResponse(c)
	}

	log.Printf("[EXP] %q gained %d xp from monster session=%d entry=%d level=%d->%d exp=%d",
		c.Char.Name, xp, monster.SessionID, monster.EntryID, oldLevel, c.Char.Level, c.Char.Exp)
}

func grantManualExp(c *Client, xp int64, source string) bool {
	if c == nil || c.Char == nil || xp <= 0 {
		return false
	}

	c.Char.Exp += xp
	sendExpGained(c, nil, xp, false)

	oldLevel := c.Char.Level
	leveled := tryLevelUp(c)
	if leveled {
		autoAdvanceProfessionForLevel(c)
		sendLevelUpResponse(c)
		sendUpdateStats(c)
		sendUpdateStatsOne(c)
		sendCharacterHealthUpdate(c)
	}

	log.Printf("[EXP] %q gained %d xp from %s level=%d->%d exp=%d",
		c.Char.Name, xp, source, oldLevel, c.Char.Level, c.Char.Exp)
	return true
}

func setCharacterLevel(c *Client, level byte, source string) bool {
	if c == nil || c.Char == nil {
		return false
	}
	if level < 1 {
		level = 1
	}
	if level > maxAsda2Level {
		level = maxAsda2Level
	}

	oldLevel := c.Char.Level
	c.Char.Level = level
	c.Char.Exp = 0
	if _, err := ApplyBaseStatsToCharacter(c.Char, true); err != nil {
		log.Printf("[BaseStats] level set stats failed for %q: %v", c.Char.Name, err)
		c.Char.HP = c.Char.MaxHP
		c.Char.MP = c.Char.MaxMP
	}
	c.Char.TargetID = -1
	c.Char.IsFighting = false

	sendLevelUpResponse(c)
	autoAdvanceProfessionForLevel(c)
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
	sendCharacterHealthUpdate(c)

	log.Printf("[EXP] %q level set by %s %d->%d exp=%d",
		c.Char.Name, source, oldLevel, c.Char.Level, c.Char.Exp)
	return oldLevel != c.Char.Level
}

func monsterKillExp(chr *Character, monster *Monster) int64 {
	if chr == nil || monster == nil || chr.Level >= maxAsda2Level {
		return 0
	}

	xp := baseMonsterXP(monster.Level)
	switch int(chr.Level) - int(monster.Level) {
	case 1:
		xp = int64(float64(xp) * 0.99)
	case 2:
		xp = int64(float64(xp) * 0.95)
	case 3:
		xp = int64(float64(xp) * 0.90)
	case 4:
		xp = int64(float64(xp) * 0.85)
	case 5:
		xp = int64(float64(xp) * 0.80)
	case 6:
		xp = int64(float64(xp) * 0.01)
	}

	if xp < 1 {
		xp = 1
	}
	if cap := xpRequiredForNextLevel(chr.Level) / 4; cap > 0 && xp > cap {
		xp = cap
	}
	return xp
}

func tryLevelUp(c *Client) bool {
	if c == nil || c.Char == nil {
		return false
	}
	chr := c.Char
	oldLevel := chr.Level
	if oldLevel >= maxAsda2Level {
		return false
	}

	level := int(oldLevel)
	exp := chr.Exp
	for level < maxAsda2Level {
		required := xpRequiredForNextLevel(byte(level))
		if required <= 0 || exp < required {
			break
		}
		exp -= required
		level++
	}
	if level == int(oldLevel) {
		chr.Exp = exp
		return false
	}

	chr.Level = byte(level)
	chr.Exp = 0 // Mirrors Character.OnLevelChanged: visible current-level XP resets after level-up.
	if _, err := ApplyBaseStatsToCharacter(chr, true); err != nil {
		log.Printf("[BaseStats] level-up stats failed for %q: %v", chr.Name, err)
		chr.HP = chr.MaxHP
		chr.MP = chr.MaxMP
	}
	log.Printf("[EXP] %q leveled up %d -> %d", chr.Name, oldLevel, chr.Level)
	return true
}

func sendExpGained(c *Client, monster *Monster, xp int64, fromKillNPC bool) {
	if c == nil || c.Char == nil || xp == 0 {
		return
	}

	p := NewPacket(ExpGained)
	if fromKillNPC {
		p.WriteUint8(0)
	} else {
		p.WriteUint8(1)
	}
	p.WriteInt64(characterTotalXP(c.Char))
	p.WriteInt64(xp)
	if monster != nil {
		p.WriteUint16(uint16(monster.SessionID))
	} else {
		p.WriteUint16(0)
	}
	c.Send(p)
}

func sendLevelUpResponse(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	chr := c.Char
	stats := calculateCharacterStats(chr)
	p := NewPacket(LvlUp)
	p.WriteInt16(chr.SessionID)
	p.WriteUint8(chr.Level)
	p.WriteInt64(characterTotalXP(chr))
	p.WriteInt16(0)
	writeCharacterAttributes(p, stats.Total)
	writeZeroCharacterAttributes(p)
	writeCharacterAttributes(p, stats.Total)
	p.WriteInt32(stats.MaxHP)
	p.WriteInt16(clampInt16(stats.MaxMP))
	p.WriteInt32(chr.HP)
	p.WriteInt16(int16(chr.MP))
	p.WriteInt16(clampInt16(stats.MinDamage))
	p.WriteInt16(clampInt16(stats.MaxDamage))
	p.WriteInt16(clampInt16(stats.MinMagicDamage))
	p.WriteInt16(clampInt16(stats.MaxMagicDamage))
	p.WriteInt32(stats.MagicDefence)
	p.WriteBytes(make([]byte, 80))
	c.SendToArea(p)
}

func characterTotalXP(chr *Character) int64 {
	if chr == nil {
		return 0
	}
	return chr.Exp + startXPForLevel(chr.Level)
}

func startXPForLevel(level byte) int64 {
	idx := int(level) - 1
	if idx < 0 || idx >= len(asda2StartXPByLevel) {
		return 0
	}
	return asda2StartXPByLevel[idx]
}

func xpRequiredForNextLevel(level byte) int64 {
	idx := int(level)
	if idx <= 0 || idx >= len(asda2XPToNextLevel) {
		return 0
	}
	return asda2XPToNextLevel[idx]
}

func baseMonsterXP(level byte) int64 {
	idx := int(level)
	if idx <= 0 {
		idx = 1
	}
	if idx >= len(asda2BaseMonsterXPByLevel) {
		return 40000
	}
	return asda2BaseMonsterXPByLevel[idx]
}
