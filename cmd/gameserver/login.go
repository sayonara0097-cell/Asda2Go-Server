package main

import (
	"log"
	"math/rand"
	"time"

	"asda2/shared/relay"
)

var stub80 = []byte{
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF,
}

func handlePing(c *Client, p *PacketIn) {
	c.Send(NewPacket(Ping))
}

func sendLongTimeBuffsInfo(c *Client) {
	p := NewPacket(LongTimeBuffsInfo)
	for i := 0; i < 20; i++ {
		p.WriteInt16(-1) // packageId
		p.WriteInt16(-1) // itemId
		p.WriteInt32(-1) // seconds remaining
	}
	c.Send(p)
}

// ===========================================================================
// LocationInit  (opcode 7175)
// Mirrors Asda2LoginHandler.LocationInitRequest
//
// Replies with ClientCanLoginToGS { byte(2) } — signals the client it may
// begin sending game-play packets.
// ===========================================================================

func handleLocationInit(c *Client, p *PacketIn) {
	resp := NewPacket(ClientCanLoginToGS)
	resp.WriteUint8(2)
	c.Send(resp)
}

func handleClientVerificationNoop(c *Client, p *PacketIn) {}

// ===========================================================================
// CharacterInitOnLogin  (opcode 7170)
// Mirrors Asda2LoginHandler.CharacterInitOnLoginRequest
//
// Arrives on the game-server TCP connection after the login-server handshake.
// Validates account, reassociates character, and sends the enter-game sequence.
//
// Packet layout:
//   int32  accountId
//   skip 2
//   int16  charNum  (10–12)
//   skip 4
// ===========================================================================

func handleCharacterInitOnLogin(c *Client, p *PacketIn) {
	if c.ServerKind != ServerKindGame {
		log.Printf("[CharInitOnLogin] ignoring game-server request on %s server", c.ServerKind)
		return
	}
	// C# constructor consumes 24 bytes of our payload before handler runs.
	p.Skip(24)
	accountID := p.ReadInt32()
	p.Skip(2)
	charNum := p.ReadInt16()
	if charNum < 10 || charNum > 12 {
		c.Conn.Close()
		return
	}
	p.Skip(4)

	if pending, ok := bridge.ConsumePendingLoginAny(uint32(accountID), byte(charNum), remoteIP(c.Conn.RemoteAddr())); ok {
		if !relay.ValidGameChannel(pending.Channel) || pending.Channel != gameChannel {
			log.Printf("[Bridge] rejecting account=%d charSlot=%d for channel=%d on gameChannel=%d", accountID, charNum, pending.Channel, gameChannel)
			c.Conn.Close()
			return
		}
		c.Channel = pending.Channel
	} else {
		log.Printf("[Bridge] no pending login found for account=%d charSlot=%d; falling back to DB validation", accountID, charNum)
		c.Channel = gameChannel
	}

	row, err := GetCharacterByAccountAndSlot(int(accountID), charNum)
	if err != nil || row == nil {
		legacyEntityLowID := int64(accountID) + int64(charNum)*1_000_000
		row, err = GetCharacterByID(legacyEntityLowID)
		if err != nil || row == nil {
			log.Printf("[CharInitOnLogin] character account=%d slot=%d legacyId=%d not found: %v", accountID, charNum, legacyEntityLowID, err)
			c.Conn.Close()
			return
		}
	}

	// Rebuild account shell if this is a fresh game-server connection
	if c.Account == nil {
		accRow, err := GetAccountByID(int(accountID))
		if err != nil || accRow == nil {
			log.Printf("[CharInitOnLogin] account %d not found: %v", accountID, err)
			c.Conn.Close()
			return
		}
		c.Account = &Account{
			ID:   uint32(accRow.AccountID),
			Name: accRow.Name,
		}
	}
	if !claimGameAccountSession(c, uint32(accountID), c.Channel, byte(charNum), remoteIP(c.Conn.RemoteAddr())) {
		c.Conn.Close()
		return
	}

	chr := CharacterFromRow(row, uint32(accountID))
	if changed, err := ApplyBaseStatsToCharacter(chr, false); err != nil {
		log.Printf("[BaseStats] failed for %q: %v", chr.Name, err)
	} else if changed {
		log.Printf("[BaseStats] applied class=%d level=%d to %q", chr.Class, chr.Level, chr.Name)
	}
	c.Char = chr
	if _, _, err := guildRuntime.attachClient(c); err != nil {
		log.Printf("[Guild] attach on login failed char=%q: %v", chr.Name, err)
	}
	ensureDefaultSkills(c)
	reconcileProfessionSkills(c)
	log.Printf("[CharInitOnLogin] account=%d char=%q slot=%d", accountID, chr.Name, charNum)

	// IsLoginServerStep=false path (mirrors Character.OnLogin game-server sequence).
	sendGameServerLoginSequence(c)
}

// ===========================================================================
// SelectCharacter  (opcode 6538)
// Mirrors Asda2CharacterHandler.SelectCharacterRequest
//
// Packet layout:
//   int32   (ignored)
//   uint16  sessionId — session ID of target character to inspect
//
// Returns stat info for the target (used for player-targeting UI).
// ===========================================================================

func handleSelectCharacter(c *Client, p *PacketIn) {
	// C# constructor consumes 24 bytes of our payload before handler runs.
	p.Skip(24)
	_ = p.ReadInt32()
	sessID := p.ReadInt16()

	target := getClientBySessionID(sessID)
	if target == nil || target.Char == nil {
		return
	}
	if c.Char != nil {
		c.Char.TargetID = sessID
	}
	sendSelectCharacterResponse(c, target.Char)
}

// sendSelectCharacterResponse mirrors Asda2CharacterHandler.SelectCharacterInfo
// SelectCharacterRespone { byte(1), int32(accId), int32(maxHp), int32(hp), int16(maxMp), int16(mp), int32(0) }
func sendSelectCharacterResponse(c *Client, chr *Character) {
	p := NewPacket(SelectCharacterRespone)
	p.WriteUint8(1)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt32(chr.MaxHP)
	p.WriteInt32(chr.HP)
	p.WriteInt16(int16(chr.MaxMP))
	p.WriteInt16(int16(chr.MP))
	p.WriteInt32(0)
	c.Send(p)
}

// ===========================================================================
// WhoIsHere  (opcode 7182)
// Mirrors GlobalHandler.WhoIsHereRequest → SendWhoIsHereListResponse
//
// Sends WhoIsHereList packets (8 chars each) for same-map/same-channel ±9-level players,
// then spawns the requesting player's own model, then WhoIsHereListEnded.
// ===========================================================================

func handleWhoIsHere(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}

	World.RefreshCharacterVisibility(c)
	sendWhoIsHereList(c)
	sendCharacterVisibleNow(c, c.Char) // spawn self
	sendWhoIsHereListEnded(c)
}

func sendWhoIsHereList(c *Client) {
	var peers []*Character
	for _, other := range World.CharactersOnMap(c.Char.MapID) {
		if other.ID == c.ID || other.Char == nil {
			continue
		}
		if other.Channel != c.Channel {
			continue
		}
		diff := int(other.Char.Level) - int(c.Char.Level)
		if diff < -9 || diff > 9 {
			continue
		}
		peers = append(peers, other.Char)
	}

	// Batches of 8 (mirrors C# Take(8))
	for len(peers) > 0 {
		n := 8
		if len(peers) < n {
			n = len(peers)
		}
		batch := peers[:n]
		peers = peers[n:]

		pkt := NewPacket(WhoIsHereList)
		for _, chr := range batch {
			pkt.WriteUint8(chr.Level)
			pkt.WriteUint8(chr.ProfessionLevel)
			pkt.WriteUint8(chr.Class)
			pkt.WriteAsdaStringLocale(chr.Name, 20, c.Locale)
			pkt.WriteInt32(-1) // guildId placeholder
			pkt.WriteUint8(0)
			pkt.WriteInt16(chr.SessionID)
			pkt.WriteInt32(int32(chr.AccID))
			pkt.WriteUint8(chr.CharNum)
		}
		c.Send(pkt)
	}
}

func sendWhoIsHereListEnded(c *Client) {
	c.Send(NewPacket(WhoIsHereListEnded))
}

// sendCharacterVisibleNow mirrors GlobalHandler.SendCharacterVisibleNowResponse
//
// CharacterVisibleNow — full character spawn packet.
// Field-by-field order taken directly from the C# source.
func sendCharacterVisibleNow(receiver *Client, chr *Character) {
	if receiver == nil || chr == nil {
		return
	}
	p := NewPacket(CharacterVisibleNow)

	p.WriteInt16(chr.SessionID)
	// C#: WriteInt16((short)Asda2X) = WriteInt16(Position.X - Map.Offset) — LOCAL coords
	lx := asda2X(chr.X, chr.MapID)
	ly := asda2Y(chr.Y, chr.MapID)
	p.WriteInt16(int16(lx))
	p.WriteInt16(int16(ly))
	p.WriteUint8(chr.SettingsFlags[15])
	p.WriteUint8(byte(chr.AvatarMask)) // AvatarMask low byte
	p.WriteInt32(int32(rand.Uint32())) // random seed
	p.WriteInt32(int32(chr.AccID))
	p.WriteAsdaStringLocale(chr.Name, 20, receiver.Locale)
	p.WriteUint8(chr.CharNum)
	p.WriteUint8(0)
	p.WriteUint8(chr.Gender)
	p.WriteUint8(chr.ProfessionLevel)
	p.WriteUint8(chr.Class)
	p.WriteUint8(chr.Level)
	p.WriteInt16(0) // isBattlegroundInProgress
	p.WriteInt16(0) // currentBattleGroundId
	// factionId: -1 → send 0
	if chr.FactionID < 0 {
		p.WriteUint8(0)
	} else {
		p.WriteUint8(byte(chr.FactionID))
	}
	p.WriteUint8(0) // isInBattleground
	p.WriteUint8(chr.Zodiac)
	p.WriteUint8(chr.Hair)
	p.WriteUint8(chr.HairColor)
	p.WriteUint8(chr.Face)
	p.WriteUint8(chr.EyeColor)
	p.WriteInt16(chr.SessionID)
	p.WriteInt32(-1)
	// state: 200=dead, 108=sitting, 0=standing
	if chr.HP <= 0 {
		p.WriteInt16(200)
	} else {
		p.WriteInt16(0)
	}
	p.WriteFloat32(lx * 100) // Asda2X — local map coord * 100
	p.WriteFloat32(ly * 100) // Asda2Y — local map coord * 100
	if chr.IsMoving {
		p.WriteFloat32(chr.MoveDestX * 100)
		p.WriteFloat32(chr.MoveDestY * 100)
		p.WriteFloat32(chr.RunSpeed)
	} else {
		p.WriteFloat32(0)
		p.WriteFloat32(0)
		p.WriteFloat32(0)
	}
	p.WriteBytes(stub80) // 16 × 0xFF
	p.WriteInt32(0)

	for slot := int16(0); slot < 20; slot++ {
		item := itemInInventorySlot(chr.Items, 3, slot)
		if item == nil {
			p.WriteInt32(0)
			p.WriteUint8(0)
			continue
		}
		p.WriteInt32(int32(item.ItemID))
		p.WriteUint8(item.Enchant)
	}

	receiver.Send(p)
}

// ===========================================================================
// Helpers
// ===========================================================================

func sendGameServerLoginSequence(c *Client) {
	chr := c.Char

	// Add character to the world map
	World.EnterMap(c)

	// 1. SomeInitGS / SomeInitGSOne — empty signal packets
	c.Send(NewPacket(SomeInitGS))
	c.Send(NewPacket(SomeInitGSOne))

	// 2. CharacterInfoSessIdPosition
	sendCharacterInfoSessIdPosition(c)

	// 3. Inventory info (sends nothing if no items — matches C# foreach on empty list)
	sendInventoryInfo(c)

	// 4. Stats
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
	scheduleDelayedStatsRefresh(c, chr)

	// 5. Fast item slots
	sendAllFastItemSlotsInfo(c)

	// 6. Learned skills (IsFirstGameConnection path)
	sendLearnedSkillsInfo(c)

	// 7. My session ID (uses GetedTitles opcode — C# reuses it intentionally)
	sendMySessionId(c)

	// 8. Pet box size
	sendPetBoxSizeInit(c)

	// 9. Quests list
	sendQuestsList(c)

	// 10. Discovered titles
	sendDiscoveredTitles(c)

	// 11. Geted titles (real titles response, also uses GetedTitles opcode)
	sendGetedTitles(c)

	// 12. Place in title rating
	sendCharacterPlaceInTitleRating(c, chr)

	// 13. Learned recipes
	sendLearnedRecipes(c)

	// 14. Mount box size
	sendMountBoxSizeInit(c)

	// 15. Guild state
	sendGuildLoginInfo(c)

	// 16. Faction & rank
	sendCharacterFactionAndFactionRank(c, chr)

	// 17. Soulmate introduction update (empty for new chars)
	sendSoulmateIntroductionUpdate(c)

	// 18. Long-time buffs
	sendLongTimeBuffsInfo(c)

	// 19. Faction and honor points
	sendFactionAndHonorPointsInit(c)

	// 20. Fishing level
	sendFishingLevel(c)

	// 21. Set client time
	sendSetClientTime(c)

	if hasSavedTeleportPoints(chr) {
		time.AfterFunc(time.Second, func() {
			if c.Char == chr {
				sendSavedLocationsInit(c)
			}
		})
	}

	scheduleInitialEnvironmentVisibility(c)

	log.Printf("[GS] login sequence complete for %q (session=%d)", chr.Name, chr.SessionID)
}

func scheduleDelayedStatsRefresh(c *Client, chr *Character) {
	time.AfterFunc(500*time.Millisecond, func() {
		if c != nil && c.Char == chr {
			sendUpdateStats(c)
			sendUpdateStatsOne(c)
		}
	})
}

func scheduleInitialEnvironmentVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	time.AfterFunc(500*time.Millisecond, func() {
		if c == nil || c.Char == nil {
			return
		}
		gm := World.GetMap(c.Char.MapID)
		if gm == nil {
			return
		}
		var npcs int
		var monsters int
		if npcServerClient != nil {
			npcs, monsters = World.RefreshNpcServerVisibility(c, true)
		} else {
			npcs = gm.sendVisibleNpcsTo(c)
			monsters = gm.sendVisibleMonstersTo(c)
		}
		World.RefreshPortalVisibility(c)
		log.Printf("[GS] initial visibility sent to %q npcs=%d monsters=%d", c.Char.Name, npcs, monsters)
	})
}

// sendCharacterInfoSessIdPosition mirrors Asda2CharacterHandler.SendCharacterInfoSessIdPositionResponse
// CharacterInfoSessIdPosition { int16(sessionId), int16(x), int16(y), int16(-1), byte(settingsFlags[15]), byte(avatarMask) }
func sendCharacterInfoSessIdPosition(c *Client) {
	chr := c.Char
	// C#: WriteInt16(Asda2X) = WriteInt16(Position.X - Map.Offset) — LOCAL map coords
	lx := asda2X(chr.X, chr.MapID)
	ly := asda2Y(chr.Y, chr.MapID)
	p := NewPacket(CharacterInfoSessIdPosition)
	p.WriteInt16(chr.SessionID)
	p.WriteInt16(int16(lx))
	p.WriteInt16(int16(ly))
	p.WriteInt16(-1)
	p.WriteUint8(0) // settingsFlags[15] — zero for fresh character
	p.WriteUint8(byte(chr.AvatarMask))
	c.Send(p)
}

// sendInventoryInfo mirrors Asda2LoginHandler.SendInventoryInfoResponse
// Only sends packets if there are items. Empty inventory = no packets sent.
func sendInventoryInfo(c *Client) {
	if c.Char == nil {
		return
	}
	for _, batch := range chunkItems(itemsInInventory(c.Char.Items, 2), 9) {
		p := NewPacket(RegularInventoryInfo)
		for _, item := range batch {
			writeItemInfoToPacket(p, item, c.Char, false)
		}
		c.Send(p)
	}
	for _, batch := range chunkItems(itemsInInventory(c.Char.Items, 1), 9) {
		p := NewPacket(ShopInventoryInfo)
		for _, item := range batch {
			writeItemInfoToPacket(p, item, c.Char, false)
		}
		c.Send(p)
	}
}

// sendUpdateStats mirrors Asda2CharacterHandler.SendUpdateStatsResponse
// UpdateStats { int32 maxHp, int16 maxMp, int32 hp, int16 mp,
//
//	int16×6 combat stats, int32 blockChance, int32 blockVal,
//	int16(15), int16(7), int16(4), 28B stub40 }
func sendUpdateStats(c *Client) {
	chr := c.Char
	stats := calculateCharacterStats(chr)
	p := NewPacket(UpdateStats)
	p.WriteInt32(stats.MaxHP)
	p.WriteInt16(clampInt16(stats.MaxMP))
	p.WriteInt32(chr.HP)
	p.WriteInt16(int16(chr.MP))
	p.WriteInt16(clampInt16(stats.MinDamage))      // minDamage
	p.WriteInt16(clampInt16(stats.MaxDamage))      // maxDamage
	p.WriteInt16(clampInt16(stats.MinMagicDamage)) // minMagicDamage
	p.WriteInt16(clampInt16(stats.MaxMagicDamage)) // maxMagicDamage
	p.WriteInt16(clampInt16(stats.MagicDefence))   // magicDefence
	p.WriteInt16(clampInt16(stats.DefenceMin))     // defence min
	p.WriteInt16(clampInt16(stats.DefenceMax))     // defence max
	p.WriteInt32(stats.BlockChance)
	p.WriteInt32(stats.BlockValue)
	p.WriteInt16(15)
	p.WriteInt16(7)
	p.WriteInt16(4)
	p.WriteBytes(make([]byte, 28)) // stub40
	c.Send(p)
}

// sendUpdateStatsOne mirrors Asda2CharacterHandler.SendUpdateStatsOneResponse.
// The packet writes base stats, equipment/modifier deltas, then base stats again.
func sendUpdateStatsOne(c *Client) {
	chr := c.Char
	stats := calculateCharacterStats(chr)
	p := NewPacket(UpdateStatsOne)
	// Raw base stats
	writeCharacterAttributes(p, stats.Base)
	// Equipment/modifier stat deltas.
	writeCharacterAttributes(p, stats.Bonus)
	// Reference client expects the base stat group again at the tail.
	writeCharacterAttributes(p, stats.Base)
	c.Send(p)
}

// sendLearnedSkillsInfo mirrors Asda2CharacterHandler.SendLearnedSkillsInfo
// Sends the current Asda2 MVP skill list until DB-backed learned skills are added.
func sendLearnedSkillsInfo(c *Client) {
	sendSkillsInfo(c)
}

// sendMySessionId mirrors Asda2CharacterHandler.SendMySessionIdResponse
// Uses GetedTitles opcode (intentional C# reuse).
// stab14 = 3 zero bytes, stub13 = 61 zero bytes.
func sendMySessionId(c *Client) {
	chr := c.Char
	p := NewPacket(GetedTitles)
	p.WriteInt16(chr.SessionID)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt16(0)
	p.WriteBytes(make([]byte, 3)) // stab14
	p.WriteUint8(chr.Class)
	p.WriteBytes(make([]byte, 61)) // stub13
	c.Send(p)
}

// sendPetBoxSizeInit mirrors Asda2CharacterHandler.SendPetBoxSizeInitResponse
// PetBoxSizeInit { int32(accId), byte(petBoxSize) }
// petBoxSize = (PetBoxEnchants + 1) * 6, default = 6 for new char (0+1)*6
func sendPetBoxSizeInit(c *Client) {
	chr := c.Char
	p := NewPacket(PetBoxSizeInit)
	p.WriteInt32(int32(chr.AccID))
	p.WriteUint8(6) // (0+1)*6 = 6 default slots
	c.Send(p)
}

// sendQuestsList mirrors Asda2QuestHandler.SendQuestsListResponse
// QuestsList — 12 quest slots (all with defaults matching C# source), then 1 byte(254) + 149 bytes.
func sendQuestsList(c *Client) {
	p := NewPacket(QuestsList)
	for i := 0; i < 12; i++ {
		p.WriteInt32(-1) // questId
		p.WriteUint8(0)  // unk1
		p.WriteInt16(-1) // questSlot
		p.WriteUint8(0)  // questStage
		p.WriteInt16(-1) // oneMoreQuestId
		p.WriteInt16(2)  // IsCompleted
		p.WriteInt16(-1) // unk2
		p.WriteInt32(-1) // questItemId × 5
		p.WriteInt16(0)
		p.WriteInt32(-1)
		p.WriteInt16(0)
		p.WriteInt32(-1)
		p.WriteInt16(0)
		p.WriteInt32(-1)
		p.WriteInt16(0)
		p.WriteInt32(-1)
		p.WriteInt16(0)
	}
	p.WriteUint8(254)
	p.WriteBytes(make([]byte, 149))
	c.Send(p)
}

// sendDiscoveredTitles mirrors Asda2TitlesHandler.SendDiscoveredTitlesResponse
// DiscoveredTitles { int32(accId), int16(sessionId), 64B(titleMask=all zero),
//
//	int16(1),int16(10),int16(2),int16(20),int16(3),int16(40) }
func sendDiscoveredTitles(c *Client) {
	chr := c.Char
	p := NewPacket(DiscoveredTitles)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt16(chr.SessionID)
	p.WriteBytes(make([]byte, 64)) // 16 × uint32 zero bitmask (no titles discovered)
	p.WriteInt16(1)
	p.WriteInt16(10)
	p.WriteInt16(2)
	p.WriteInt16(20)
	p.WriteInt16(3)
	p.WriteInt16(40)
	c.Send(p)
}

// sendGetedTitles mirrors Asda2TitlesHandler.SendGetedTitlesResponse
// GetedTitles { int16(sessionId), int32(accId), int32(titlePoints), 64B(titleMask=all zero) }
// NOTE: same opcode as sendMySessionId — client distinguishes by packet order.
func sendGetedTitles(c *Client) {
	chr := c.Char
	p := NewPacket(GetedTitles)
	p.WriteInt16(chr.SessionID)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt32(0)                // Asda2TitlePoints
	p.WriteBytes(make([]byte, 64)) // 16 × uint32 zero bitmask (no titles earned)
	c.Send(p)
}

// sendMountBoxSizeInit mirrors Asda2MountHandler.SendMountBoxSizeInitResponse
// MountBoxSizeInit { int32(accId), byte(mountBoxSize) }
// Default MountBoxSize = 10 (base value in C# entity)
func sendMountBoxSizeInit(c *Client) {
	chr := c.Char
	p := NewPacket(MountBoxSizeInit)
	p.WriteInt32(int32(chr.AccID))
	p.WriteUint8(10) // default mount box size
	c.Send(p)
}

// sendSoulmateIntroductionUpdate mirrors Asda2SoulmateHandler.SendCharacterSoulMateIntrodactionUpdateResponse
//
// C# ALWAYS sends this packet even when the character has no soulmate.
// Null-soulmate layout:
//
//	int32(0)   soulmate accId = 0
//	byte(0)    hasSoulmate = false
//
// Omitting this packet breaks all subsequent packet parsing because the client
// reads the stream sequentially and misaligns on the missing opcode.
func sendSoulmateIntroductionUpdate(c *Client) {
	p := NewPacket(CharacterSoulMateIntrodactionUpdate)
	p.WriteInt32(0) // soulmate accId (0 = none)
	p.WriteUint8(0) // hasSoulmate = false
	c.Send(p)
}

// sendCharacterFactionAndFactionRank mirrors GlobalHandler.CreateCharacterFactionPacket
// CharacterFactionAndFactionRank { int32(accId), int16(sessionId), int16(factionId), byte(factionRank) }
func sendCharacterFactionAndFactionRank(c *Client, chr *Character) {
	p := NewPacket(CharacterFactionAndFactionRank)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt16(chr.SessionID)
	p.WriteInt16(chr.FactionID)
	p.WriteUint8(byte(chr.FactionRank))
	c.Send(p)
}

// sendCharacterPlaceInTitleRating mirrors GlobalHandler.CreateCharacterPlaceInRaitingPacket
// CharacterPlaceInTitleRating { int16(sessionId), int32(accId), int32(rank), int16(preTitleId), int16(postTitleId) }
func sendCharacterPlaceInTitleRating(c *Client, chr *Character) {
	p := NewPacket(CharacterPlaceInTitleRating)
	p.WriteInt16(chr.SessionID)
	p.WriteInt32(int32(chr.AccID))
	p.WriteInt32(chr.Rank)
	p.WriteInt16(chr.PreTitleID)
	p.WriteInt16(chr.PostTitleID)
	c.Send(p)
}

// sendFactionAndHonorPointsInit mirrors Asda2CharacterHandler.SendFactionAndHonorPointsInitResponse
// FactionAndHonorPointsInit { int16(factionId), int32(honorPoints), byte(factionRank) }
func sendFactionAndHonorPointsInit(c *Client) {
	chr := c.Char
	p := NewPacket(FactionAndHonorPointsInit)
	p.WriteInt16(chr.FactionID)
	p.WriteInt32(chr.HonorPoints)
	p.WriteUint8(byte(chr.FactionRank))
	c.Send(p)
}

// sendFishingLevel mirrors Asda2FishingHandler.SendFishingLvlResponse
// UpdateFishingLvl { int32(fishingLevel) }
func sendFishingLevel(c *Client) {
	p := NewPacket(UpdateFishingLvl)
	p.WriteInt32(0)
	c.Send(p)
}
