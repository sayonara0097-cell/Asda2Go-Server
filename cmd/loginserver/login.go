package main

import (
	"asda2/shared/relay"
	charstats "asda2/shared/stats"
	"asda2/shared/types"
	"log"
	"net"
	"time"
)

var unk51 = make([]byte, 28)

const (
	CreateCharResultOk           byte = 0
	CreateCharResultBadName      byte = 2
	CreateCharResultAlreadyInUse byte = 3
)

func handlePing(c *Client, p *PacketIn) {
	c.Send(NewPacket(Ping))
}

// ===========================================================================
// AuthorizeRequest  (opcode 1003)
// Mirrors AuthenticationHandler.AuthChallengeRequest
//
// Packet layout (payload after opcode, relative to our p.Data[0]):
//
// C# dispatches: new RealmPacketIn(segment, offset=7, ...) which does
//   ++pos (1) + ReadUInt16 opcode (2) + pos+=24 → handler entry at raw[34].
// AuthChallengeRequest then does packet.Position += 20 → raw[54].
// Our p.Data[0] = raw[10], so we must skip 54-10 = 44 bytes before the name.
//
//   +44 skip          header consumed by C# constructor + handler preamble
//   32B  name         ReadAsdaString(32) — null-terminated, field-aligned
//   [+19 if Locale.Ru]
//   32B  password     ReadAsdaString(32)
// ===========================================================================

func handleAuthorizeRequest(c *Client, p *PacketIn) {
	if c.ServerKind != ServerKindLogin {
		log.Printf("[Auth] ignoring authorize on %s server", c.ServerKind)
		return
	}
	if c.Account != nil {
		return // already authenticated
	}

	p.Skip(44)
	name := p.ReadAsdaStringLocale(32, c.Locale)
	if c.Locale == LocaleRu {
		p.Skip(19)
	}
	password := p.ReadAsdaStringLocale(32, c.Locale)

	log.Printf("[Auth] login attempt: %q", name)

	row, err := GetAccountByName(name)
	if err != nil {
		log.Printf("[Auth] DB error for %q: %v", name, err)
		sendAuthFail(c, 5)
		return
	}
	if row == nil || row.Password != password {
		sendAuthFail(c, 5)
		return
	}
	if !row.IsActive {
		log.Printf("[Auth] account %q is inactive/banned", name)
		sendAuthFail(c, 5)
		return
	}

	remoteIP := c.Conn.RemoteAddr().(*net.TCPAddr).IP
	if !claimLoginAccountSession(c, uint32(row.AccountID), remoteIP.String()) {
		sendAuthFail(c, 5)
		return
	}
	_ = UpdateAccountLogin(row.AccountID, remoteIP)

	c.Account = &Account{
		ID:       uint32(row.AccountID),
		Name:     row.Name,
		Password: row.Password,
		IsOnline: true,
		LastIP:   remoteIP,
	}

	log.Printf("[Auth] %q authenticated (id=%d)", name, row.AccountID)
	sendAuthSuccess(c)
}

// sendAuthSuccess mirrors AuthenticationHandler.SendAuthChallengeSuccessReply
//
// 1. AuthorizeResponse  { int16(1), int32(accountId), byte(1) }
// 2. ChanelInfoResponse (sent twice, identical payload)
func sendAuthSuccess(c *Client) {
	resp := NewPacket(AuthorizeResponse)
	resp.WriteInt16(1)
	resp.WriteInt32(int32(c.Account.ID))
	resp.WriteUint8(1)
	c.Send(resp)

	for i := 0; i < 2; i++ {
		sendChanelInfoResponse(c, nil)
	}
}

// sendAuthFail mirrors AuthenticationHandler.SendAuthChallengeFailReply (non-ban path)
// AuthorizeResponse { int32(errorCode), int16(5), byte(0) }
func sendAuthFail(c *Client, errorCode int32) {
	resp := NewPacket(AuthorizeResponse)
	resp.WriteInt32(errorCode)
	resp.WriteInt16(5)
	resp.WriteUint8(0)
	c.Send(resp)
}

// ===========================================================================
// SelectChanelRequest  (opcode 1006)
// Mirrors AuthenticationHandler.SelectChanelRequest
//
// Loads all character rows for the account and sends:
//  1. CharacterInfos   — one packet per character (avatar equipment preview)
//  2. CharacterNames   — single packet, all 3 slots
//  3. ShowCharactersView { byte(1) }
// ===========================================================================

func handleSelectChanelRequest(c *Client, p *PacketIn) {
	if c.Account == nil {
		sendAuthFail(c, 5)
		return
	}

	rows, err := GetCharactersByAccount(int(c.Account.ID))
	if err != nil {
		log.Printf("[CharSelect] DB error for account %d: %v", c.Account.ID, err)
		return
	}
	c.Account.Characters = rows

	sendCharacterInfosResponse(c)
	sendCharacterNamesResponse(c)
	sendShowCharactersViewResponse(c)
}

// sendCharacterInfosResponse mirrors AuthenticationHandler.SendCharacterInfoLSResponse
//
// One CharacterInfos packet per character.
// Layout per packet:
//
//	int32(0)
//	byte(charNum)
//	byte(2)
//	20 × equipment slot:
//	  if item found (InventoryType==3, slot==index):
//	    int32(itemId), int16(slot), int32(-1), int32(0)
//	  else:
//	    int32(0), int16(0), int32(-1), int32(0)
func sendCharacterInfosResponse(c *Client) {
	for _, chr := range c.Account.Characters {
		p := NewPacket(CharacterInfos)
		p.WriteInt32(0)
		p.WriteUint8(chr.CharNum)
		p.WriteUint8(2)

		for slot := 0; slot < 20; slot++ {
			found := false
			for _, item := range chr.LoadedItems {
				if item.Slot == int16(slot) && item.InventoryType == 3 {
					p.WriteInt32(int32(item.ItemID))
					p.WriteInt16(item.Slot)
					p.WriteInt32(-1)
					p.WriteInt32(0)
					found = true
					break
				}
			}
			if !found {
				p.WriteInt32(0)
				p.WriteInt16(0)
				p.WriteInt32(-1)
				p.WriteInt32(0)
			}
		}
		c.Send(p)
	}
}

// sendCharacterNamesResponse mirrors AuthenticationHandler.SendCharacterNamesResponse
// (CharacterNames packet).
//
// Layout:
//
//	int32(0)
//	3 slots (charNum 10, 11, 12) each:
//	  byte(charNum or 0)
//	  AsdaString(name, 21)
//	  byte(gender), byte(professionLevel), byte(class), byte(level)
//	  int64(1 if real / 0 if empty)
//	  int32(hp), int16(mp), int32(maxHp), int16(maxMp)
//	  int16(str,agi,sta,spi,int)
//	  int16(10), byte(0)
//	Appearance footer (hair,hairColor,face × 3) then zodiac × 3
//	16 × byte(1)
//	int32(63)
func sendCharacterNamesResponse(c *Client) {
	byNum := map[byte]*CharacterRow{}
	for _, chr := range c.Account.Characters {
		byNum[chr.CharNum] = chr
	}

	p := NewPacket(CharacterNames)
	p.WriteInt32(0)

	var hair [3]byte
	var hairCol [3]byte
	var face [3]byte
	var zodiac [3]byte

	for i, num := range []byte{10, 11, 12} {
		chr, ok := byNum[num]
		if ok {
			hair[i] = chr.HairStyle
			hairCol[i] = chr.HairColor
			face[i] = chr.Face
			zodiac[i] = chr.Zodiac

			p.WriteUint8(chr.CharNum)
			p.WriteAsdaStringLocale(chr.Name, 21, c.Locale)
			p.WriteUint8(chr.Gender)
			p.WriteUint8(chr.ProfessionLevel)
			p.WriteUint8(byte(chr.Asda2Class))
			p.WriteUint8(byte(chr.Level))
			p.WriteInt64(1)
			p.WriteInt32(int32(chr.Health))
			p.WriteInt16(int16(chr.Power))
			p.WriteInt32(int32(chr.BaseHealth))
			p.WriteInt16(int16(chr.BasePower))
			p.WriteInt16(int16(chr.BaseStrength))
			p.WriteInt16(int16(chr.BaseAgility))
			p.WriteInt16(int16(chr.BaseStamina))
			p.WriteInt16(int16(chr.BaseSpirit))
			p.WriteInt16(int16(chr.BaseIntellect))
			p.WriteInt16(int16(chr.BaseLuck))
			p.WriteUint8(0)
		} else {
			// Empty slot
			p.WriteUint8(0)
			p.WriteAsdaStringLocale("", 21, c.Locale)
			p.WriteUint8(0)
			p.WriteUint8(0)
			p.WriteUint8(0)
			p.WriteUint8(0)
			p.WriteInt64(0)
			p.WriteInt32(0)
			p.WriteInt16(0)
			p.WriteInt32(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteInt16(0)
			p.WriteUint8(0)
		}
	}

	// Appearance footer: hair/hairColor/face for slots 0,1,2 then zodiac
	for i := 0; i < 3; i++ {
		p.WriteUint8(hair[i])
		p.WriteUint8(hairCol[i])
		p.WriteUint8(face[i])
	}
	for i := 0; i < 3; i++ {
		p.WriteUint8(zodiac[i])
	}

	for i := 0; i < 16; i++ {
		p.WriteUint8(1)
	}
	p.WriteInt32(63)
	c.Send(p)
}

// sendShowCharactersViewResponse mirrors AuthenticationHandler.SendShowCharactersViewResponse
// ShowCharactersView { byte(1) }
func sendShowCharactersViewResponse(c *Client) {
	p := NewPacket(ShowCharactersView)
	p.WriteUint8(1)
	c.Send(p)
}

// ===========================================================================
// CreateCharacterRequest  (opcode 7172)
// Mirrors Asda2CharacterHandler.CreateCharacterRequest
//
// Packet layout:
//   uint32   (accountId echo — ignored)
//   uint16   skip
//   byte     charNum  (10, 11, or 12)
//   20B      name
//   byte     gender
//   byte     hairStyle
//   byte     hairColor
//   byte     face
//   byte     zodiac
// ===========================================================================

func handleCreateCharacterRequest(c *Client, p *PacketIn) {
	if c.Account == nil || c.Char != nil {
		return
	}

	// C# constructor consumes 24 bytes of our payload before handler runs.
	p.Skip(24)
	_ = p.ReadUint32() // accountId echo
	p.Skip(2)
	charNum := p.ReadUint8()
	if charNum < 10 || charNum > 12 {
		sendCreateCharacterResponse(c, CreateCharResultBadName)
		return
	}

	name := p.ReadAsdaStringLocale(20, c.Locale)
	gender := p.ReadUint8()
	hairStyle := p.ReadUint8()
	hairColor := p.ReadUint8()
	face := p.ReadUint8()
	zodiac := p.ReadUint8()

	log.Printf("[CreateChar] account=%d slot=%d name=%q", c.Account.ID, charNum, name)

	if !isValidCharName(name) {
		sendCreateCharacterResponse(c, CreateCharResultBadName)
		return
	}

	// Ensure characters are loaded
	if c.Account.Characters == nil {
		rows, err := GetCharactersByAccount(int(c.Account.ID))
		if err != nil {
			log.Printf("[CreateChar] DB error loading chars: %v", err)
			sendCreateCharacterResponse(c, CreateCharResultBadName)
			return
		}
		c.Account.Characters = rows
	}

	if len(c.Account.Characters) > 2 {
		sendCreateCharacterResponse(c, CreateCharResultBadName)
		return
	}

	exists, err := CharacterNameExists(name)
	if err != nil {
		log.Printf("[CreateChar] DB error checking name: %v", err)
		sendCreateCharacterResponse(c, CreateCharResultBadName)
		return
	}
	if exists {
		sendCreateCharacterResponse(c, CreateCharResultAlreadyInUse)
		return
	}

	// EntityLowId = accountId + charNum * 1_000_000
	// Mirrors: Character.CharacterIdFromAccIdAndCharNum(account.AccountId, charNum)
	entityLowID := int64(c.Account.ID) + int64(charNum)*1_000_000

	// Default spawn: Alpia (MapId=3), PositionX=3066, PositionY=3350
	// Mirrors CharacterRecord.SetupNewRecord: PositionX=3066f, PositionY=3350f, MapId=Alpia(3)
	row := &CharacterRow{
		AccountID:   int(c.Account.ID),
		EntityLowID: entityLowID,
		Name:        name,
		CharNum:     charNum,
		Gender:      gender,
		HairStyle:   hairStyle,
		HairColor:   hairColor,
		Face:        face,
		Zodiac:      zodiac,
		Level:       1,
		Health:      100,
		BaseHealth:  100,
		Power:       50,
		BasePower:   50,
		Map:         3,      // Alpia
		PositionX:   3066.0, // World coordinate (Asda2X = 3066 - 3*1000 = 66)
		PositionY:   3350.0, // World coordinate (Asda2Y = 3350 - 3*1000 = 350)
		Created:     time.Now(),
	}
	if _, err := ApplyBaseStatsToCharacterRow(row, true); err != nil {
		log.Printf("[CreateChar] base stats unavailable for %q: %v", name, err)
	}

	if err := CreateCharacter(row); err != nil {
		log.Printf("[CreateChar] failed to save: %v", err)
		sendCreateCharacterResponse(c, CreateCharResultBadName)
		return
	}

	c.Account.Characters = append(c.Account.Characters, row)
	log.Printf("[CreateChar] created %q (slot %d) for account %d", name, charNum, c.Account.ID)

	sendCreateCharacterResponseOne(c, row)
	sendCreateCharacterResponse(c, CreateCharResultOk)
}

// sendCreateCharacterResponseOne mirrors Asda2CharacterHandler.SendCreateCharacterResponseOneResponse
func sendCreateCharacterResponseOne(c *Client, chr *CharacterRow) {
	p := NewPacket(CreateCharacterResponseOne)
	p.WriteInt32(int32(chr.AccountID))
	p.WriteAsdaStringLocale(chr.Name, 18, c.Locale)
	p.WriteInt16(0)
	p.WriteInt16(int16(chr.CharNum))
	p.WriteUint8(0)
	p.WriteUint8(chr.Gender)
	p.WriteInt16(0)
	p.WriteInt64(1)
	p.WriteInt32(0)
	p.WriteInt16(0)
	p.WriteInt32(131807896) // C# constant
	p.WriteInt64(0)
	p.WriteUint8(0)
	p.WriteInt32(7683) // C# constant
	p.WriteInt16(0)
	p.WriteUint8(0)
	p.WriteInt16(-1)
	p.WriteInt16(0)
	p.WriteUint8(chr.Zodiac)
	p.WriteUint8(chr.HairStyle)
	p.WriteUint8(chr.HairColor)
	p.WriteUint8(chr.Face)
	c.Send(p)
}

// sendCreateCharacterResponse mirrors Asda2CharacterHandler.SendCreateCharacterResponse
// CreateCharacterResponse { byte(result) }
func sendCreateCharacterResponse(c *Client, result byte) {
	p := NewPacket(CreateCharacterResponse)
	p.WriteUint8(result)
	c.Send(p)
}

// ===========================================================================
// EnterGameRequset  (opcode 7180)
// Mirrors Asda2LoginHandler.PlayerLoginRequestLS
//
// Packet layout:
//   5B skip
//   byte charNum  (10, 11, or 12)
//
// Selects the character and sends the full enter-game sequence.
// ===========================================================================

func handleEnterGameRequest(c *Client, p *PacketIn) {
	if c.ServerKind != ServerKindLogin {
		log.Printf("[EnterGame] ignoring login-server request on %s server", c.ServerKind)
		return
	}
	if c.Account == nil {
		c.Conn.Close()
		return
	}
	if c.Char != nil {
		return // already in game
	}

	// C# constructor consumes 24 bytes of our payload before handler runs.
	// PlayerLoginRequestLS then does packet.Position += 5 → total skip 29.
	p.Skip(29)
	charNum := p.ReadUint8()
	channel := byte(0)
	if p.Remaining() >= 3 {
		p.Skip(2)
		channel = p.ReadUint8()
	}
	if !relay.ValidGameChannel(channel) {
		log.Printf("[EnterGame] invalid channel %d requested by account=%d", channel, c.Account.ID)
		c.Conn.Close()
		return
	}
	if charNum < 10 || charNum > 12 {
		c.Conn.Close()
		return
	}
	c.Channel = bridge.SelectOnlineChannel(channel)
	if c.Channel != channel {
		log.Printf("[EnterGame] requested channel %d unavailable; using channel %d", channel, c.Channel)
	}

	// Ensure characters loaded
	if c.Account.Characters == nil {
		rows, err := GetCharactersByAccount(int(c.Account.ID))
		if err != nil {
			log.Printf("[EnterGame] DB error: %v", err)
			c.Conn.Close()
			return
		}
		c.Account.Characters = rows
	}

	var chosenRow *CharacterRow
	for _, row := range c.Account.Characters {
		if row.CharNum == charNum {
			chosenRow = row
			break
		}
	}
	if chosenRow == nil {
		log.Printf("[EnterGame] account %d has no character in slot %d", c.Account.ID, charNum)
		c.Conn.Close()
		return
	}

	if _, err := ApplyBaseStatsToCharacterRow(chosenRow, false); err != nil {
		log.Printf("[BaseStats] failed for %q: %v", chosenRow.Name, err)
	}
	chr := CharacterFromRow(chosenRow, uint32(c.Account.ID))
	if !promoteLoginAccountSessionToHandoff(c, c.Channel, chr.CharNum) {
		c.Conn.Close()
		return
	}
	c.Char = chr
	log.Printf("[EnterGame] login-server step for %q (slot %d channel %d)", chr.Name, chr.CharNum, c.Channel)
	bridge.RegisterPendingLogin(PendingLogin{
		AccountID: c.Account.ID,
		CharNum:   chr.CharNum,
		Channel:   c.Channel,
		ClientIP:  remoteIP(c.Conn.RemoteAddr()),
	})

	// IsLoginServerStep=true path (mirrors Character.OnLogin when IsLoginServerStep):
	// Send 3 packets then the client disconnects and reconnects to the game server.
	sendEnterGameResponse(c)
	sendEnterGameResponseItemsOnCharacter(c)
	sendEnterWorldIPEResponse(c)
	// Do NOT send LongTimeBuffsInfo here — that is game-server only.
	// The client will reconnect via CharacterInitOnLogin on the game-server port.
}

func sendEnterGameResponse(c *Client) {
	chr := c.Char
	acc := c.Account
	if chr == nil || acc == nil {
		return
	}

	p := NewPacket(EnterGameRespose)
	stats := charstats.CalculateCharacterStats(chr)

	p.WriteInt32(int32(acc.ID))
	p.WriteAsdaStringLocale(chr.Name, 20, c.Locale)
	p.WriteInt16(int16(chr.CharNum))
	p.WriteUint8(chr.Zodiac)
	p.WriteUint8(chr.Gender)
	p.WriteUint8(chr.ProfessionLevel)
	p.WriteUint8(chr.Class)
	p.WriteUint8(chr.Level)
	p.WriteUint32(uint32(chr.Exp))
	p.WriteInt32(0)     // unk
	p.WriteInt16(0)     // AvalibleSkillPoints: keep login-step packet layout conservative
	p.WriteInt16(0)     // unk
	p.WriteUint8(0)     // unk
	p.WriteInt16(15000) // constant from C#
	p.WriteInt16(1000)  // constant from C#
	p.WriteInt64(chr.Gold)
	p.WriteUint8(0) // unk
	p.WriteUint8(3) // unk
	p.WriteInt16(loginWarehouseSlots(chr, false))
	p.WriteInt16(loginWarehouseUsed(chr, types.InventoryWarehouse))
	p.WriteInt16(loginWarehouseSlots(chr, true))
	p.WriteInt16(loginWarehouseUsed(chr, types.InventoryAvatarWarehouse))
	p.WriteUint8(0)
	p.WriteUint8(1)
	p.WriteInt16(-1)
	p.WriteInt16(0)
	p.WriteUint8(chr.Zodiac)
	p.WriteUint8(chr.Hair)
	p.WriteUint8(chr.HairColor)
	p.WriteUint8(chr.Face)
	p.WriteInt32(chr.HP)
	p.WriteInt16(int16(chr.MP))
	p.WriteInt32(stats.MaxHP)
	p.WriteInt16(clampInt16(stats.MaxMP))
	p.WriteInt16(clampInt16(stats.MinDamage))
	p.WriteInt16(clampInt16(stats.MaxDamage))
	p.WriteInt16(clampInt16(stats.MinMagicDamage))
	p.WriteInt16(clampInt16(stats.MaxMagicDamage))
	p.WriteInt16(clampInt16(stats.MagicDefence))
	p.WriteInt16(clampInt16(stats.DefenceMin))
	p.WriteInt16(clampInt16(stats.DefenceMax))
	p.WriteInt32(stats.BlockValue)
	p.WriteInt32(stats.BlockValue)
	p.WriteInt16(15)
	p.WriteInt16(7)
	p.WriteInt16(4)
	p.WriteBytes(unk51) // 28 zero bytes
	p.WriteBytes(chr.SettingsFlags[:])
	p.WriteInt32(chr.AvatarMask)

	// 9 avatar equipment slots (indices 11..19)
	for slot := int16(11); slot < 20; slot++ {
		writeItemInfoToPacket(p, itemInInventorySlot(chr.Items, 3, slot), chr, false)
	}

	c.Send(p)
}

func clampInt16(value int32) int16 {
	if value > 32767 {
		return 32767
	}
	if value < -32768 {
		return -32768
	}
	return int16(value)
}

func loginWarehouseSlots(chr *Character, avatar bool) int16 {
	if chr == nil {
		return types.DefaultWarehouseBagSlots
	}
	extraBags := chr.PremiumWarehouseBagsCount
	if avatar {
		extraBags = chr.PremiumAvatarWarehouseBagsCount
	}
	slots := int16(extraBags+1) * types.DefaultWarehouseBagSlots
	if slots > types.WarehouseInventorySlots {
		return types.WarehouseInventorySlots
	}
	return slots
}

func loginWarehouseUsed(chr *Character, inv byte) int16 {
	if chr == nil {
		return 0
	}
	return int16(len(itemsInInventory(chr.Items, inv)))
}

func writeNullItemToPacket(p *PacketOut) {
	writeItemInfoToPacket(p, nil, nil, false)
}

// ===========================================================================
// sendEnterGameResponseItemsOnCharacter
// Mirrors Asda2LoginHandler.SendEnterGameResponseItemsOnCharacterResponse
//
// EnterGameResponseItemsOnCharacter — equipment slots 0..11 (weapon/armor slots).
// Each slot uses the same WriteItemInfoToPacket layout as writeNullItemToPacket.
// ===========================================================================

func sendEnterGameResponseItemsOnCharacter(c *Client) {
	p := NewPacket(EnterGameResponseItemsOnCharacter)
	p.WriteUint8(1)

	if c.Locale == LocaleTahadi {
		p.WriteInt32(0)
	}

	// 12 equipment slots (indices 0..11)
	for slot := int16(0); slot < 12; slot++ {
		writeItemInfoToPacket(p, itemInInventorySlot(c.Char.Items, 3, slot), c.Char, false)
	}

	c.Send(p)
}

// ===========================================================================
// sendEnterWorldIPEResponse
// Mirrors Asda2LoginHandler.SendEnterWorldIpeResponseResponse
//
// EnterWorldIpeResponse — map-server address, position, 28 aura slots,
// 15 premium-buff slots.
// ===========================================================================

func sendEnterWorldIPEResponse(c *Client) {
	chr := c.Char

	log.Printf("[EnterWorld] sending %q to map %d", chr.Name, chr.MapID)

	p := NewPacket(EnterWorldIpeResponse)

	endpoint := bridge.EndpointForChannel(c.Channel)
	p.WriteInt32(-1)
	p.WriteAsdaString(endpoint.IP, 20) // server address (20 bytes)
	p.WriteUint16(endpoint.Port)
	// C#: Position.X (world coord, NOT Asda2X) and mapId
	p.WriteInt16(int16(chr.MapID))
	p.WriteInt16(int16(chr.X)) // world coord — client converts using map offset
	p.WriteInt16(int16(chr.Y))

	// 28 aura slots — all empty
	for i := 0; i < 28; i++ {
		p.WriteInt16(-1) // spellId
		p.WriteInt16(-1)
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteUint8(2) // constant
		p.WriteInt16(0)
		p.WriteUint8(1)
		p.WriteInt16(1)
	}

	// 15 premium-buff slots — all empty
	for i := 0; i < 15; i++ {
		p.WriteInt32(-1)
		p.WriteInt32(-1)
		p.WriteInt16(-1)
		p.WriteInt32(-1)
		p.WriteInt32(0)
		p.WriteInt16(-1)
	}

	c.Send(p)
}

func isValidCharName(name string) bool {
	runes := []rune(name)
	if len(runes) < 2 || len(runes) > 20 {
		return false
	}
	for _, ch := range name {
		if !isAllowedCharNameRune(ch) {
			return false
		}
	}
	return true
}

func isAllowedCharNameRune(ch rune) bool {
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
		return true
	}
	switch ch {
	case ' ', '.', 'ض', 'ص', 'ث', 'ق', 'ف', 'غ', 'ع', 'ه', 'خ', 'ح', 'ج', 'د', 'ش', 'س',
		'ي', 'ب', 'ل', 'ا', 'ت', 'ن', 'م', 'ك', 'ط', 'ذ', 'ئ', 'ء', 'ؤ', 'ر', 'ى',
		'ة', 'و', 'ز', 'ظ', 'إ', 'پ', 'چ', 'ژ', 'گ', 'ک':
		return true
	default:
		return false
	}
}
