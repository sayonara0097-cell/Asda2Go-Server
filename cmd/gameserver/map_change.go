package main

import (
	"log"
	"sync"
	"time"

	"asda2/shared/relay"
)

const mapChangePendingTTL = 45 * time.Second

type pendingMapChange struct {
	accountID uint32
	charNum   byte
	channel   byte
	expiresAt time.Time
}

var pendingMapChanges = struct {
	sync.Mutex
	byAccount map[uint32]pendingMapChange
}{byAccount: make(map[uint32]pendingMapChange)}

func rememberPendingMapChange(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	pendingMapChanges.Lock()
	pendingMapChanges.byAccount[c.Char.AccID] = pendingMapChange{
		accountID: c.Char.AccID,
		charNum:   c.Char.CharNum,
		channel:   c.Channel,
		expiresAt: time.Now().Add(mapChangePendingTTL),
	}
	pendingMapChanges.Unlock()
}

func consumePendingMapChange(accountID uint32) (pendingMapChange, bool) {
	pendingMapChanges.Lock()
	defer pendingMapChanges.Unlock()

	pending, ok := pendingMapChanges.byAccount[accountID]
	if !ok {
		return pendingMapChange{}, false
	}
	delete(pendingMapChanges.byAccount, accountID)
	if time.Now().After(pending.expiresAt) {
		log.Printf("[MapChange] expired pending reconnect account=%d charSlot=%d", accountID, pending.charNum)
		return pendingMapChange{}, false
	}
	return pending, true
}

func handleCharacterInitOnChanelChange(c *Client, p *PacketIn) {
	if c.ServerKind != ServerKindGame {
		return
	}
	accountID, ok := readMapChangeAccountID(p.Data)
	if !ok {
		log.Printf("[MapChange] reconnect missing account id client=%d payloadLen=%d data=% X", c.ID, len(p.Data), p.Data)
		_ = c.Conn.Close()
		return
	}
	pending, ok := consumePendingMapChange(uint32(accountID))
	if !ok {
		handoff, found, err := relay.ConsumePendingAccountHandoffFromDB(uint32(accountID), remoteIP(c.Conn.RemoteAddr()))
		if err != nil {
			log.Printf("[MapChange] failed to consume channel handoff account=%d client=%d: %v", accountID, c.ID, err)
			_ = c.Conn.Close()
			return
		}
		if !found {
			log.Printf("[MapChange] reconnect without pending teleport/channel handoff account=%d client=%d", accountID, c.ID)
			_ = c.Conn.Close()
			return
		}
		pending = pendingMapChange{
			accountID: handoff.AccountID,
			charNum:   handoff.CharNum,
			channel:   handoff.Channel,
			expiresAt: time.Now().Add(mapChangePendingTTL),
		}
	}
	if !relay.ValidGameChannel(pending.channel) || pending.channel != gameChannel {
		log.Printf("[MapChange] rejecting account=%d charSlot=%d for channel=%d on gameChannel=%d", accountID, pending.charNum, pending.channel, gameChannel)
		_ = c.Conn.Close()
		return
	}

	row, err := GetCharacterByAccountAndSlot(accountID, int16(pending.charNum))
	if err != nil || row == nil {
		log.Printf("[MapChange] character account=%d slot=%d not found: %v", accountID, pending.charNum, err)
		_ = c.Conn.Close()
		return
	}
	accRow, err := GetAccountByID(accountID)
	if err != nil || accRow == nil {
		log.Printf("[MapChange] account=%d not found: %v", accountID, err)
		_ = c.Conn.Close()
		return
	}

	c.Channel = pending.channel
	c.Account = &Account{ID: uint32(accRow.AccountID), Name: accRow.Name}
	if !claimGameAccountSessionForMapChange(c, uint32(accountID), pending.channel, pending.charNum, remoteIP(c.Conn.RemoteAddr())) {
		_ = c.Conn.Close()
		return
	}

	chr := CharacterFromRow(row, uint32(accountID))
	if changed, err := ApplyBaseStatsToCharacter(chr, false); err != nil {
		log.Printf("[BaseStats] failed for %q on map change: %v", chr.Name, err)
	} else if changed {
		log.Printf("[BaseStats] applied class=%d level=%d to %q on map change", chr.Class, chr.Level, chr.Name)
	}
	c.Char = chr
	ensureDefaultSkills(c)
	reconcileProfessionSkills(c)

	log.Printf("[MapChange] reconnect account=%d char=%q slot=%d map=%d", accountID, chr.Name, pending.charNum, chr.MapID)
	sendGameServerLoginSequence(c)
}

func readMapChangeAccountID(data []byte) (int, bool) {
	for _, offset := range []int{24, 0} {
		if len(data) < offset+4 {
			continue
		}
		in := &PacketIn{Data: data[offset:]}
		accountID := int(in.ReadInt32())
		if accountID > 0 {
			return int(accountID), true
		}
	}
	return 0, false
}

func claimGameAccountSessionForMapChange(c *Client, accountID uint32, channel byte, charNum byte, clientIP string) bool {
	disconnectExistingGameAccount(accountID, c.ID)

	token, err := relay.NewAccountSessionToken()
	if err != nil {
		log.Printf("[AccountSession] failed to create map-change token account=%d: %v", accountID, err)
		return false
	}
	if err := relay.ForceClaimAccountSession(accountID, token, relay.AccountSessionGame, channel, charNum, clientIP); err != nil {
		log.Printf("[AccountSession] failed to claim map-change account=%d: %v", accountID, err)
		return false
	}
	c.AccountSessionToken = token
	c.StopAccountSession = relay.StartAccountSessionHeartbeat(accountID, token, func() {
		_ = c.Conn.Close()
	})
	return true
}
