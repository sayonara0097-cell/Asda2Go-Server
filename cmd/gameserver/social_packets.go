package main

import (
	"encoding/binary"
	"strings"
)

const (
	partyInviteRequestInvite               byte = 0
	partyInviteRequestAlreadyLogout        byte = 3
	partyInviteRequestTooManyPending       byte = 4
	partyInviteRequestTargetAlreadyInGroup byte = 10
	partyInviteRequestOtherFaction         byte = 11
	partyMemberBuffSlots                        = 28
)

func readWhisperRequest(c *Client, p *PacketIn) (byte, string, string) {
	if len(p.Data) >= 22 {
		p.Seek(0)
		soulmate := p.ReadUint8()
		name := strings.TrimSpace(p.ReadAsdaStringLocale(20, c.Locale))
		msg := strings.TrimSpace(p.ReadCStringLocale(maxChatMessageLen, c.Locale))
		return soulmate, name, msg
	}
	return 0, "", ""
}

func sendWhisperChatResponse(c *Client, soulmate byte, status byte, senderAccID uint32, receiverSession int16, sender string, msg string) {
	p := NewPacket(WishperChat)
	p.WriteUint8(soulmate)
	p.WriteUint8(status)
	p.WriteUint32(senderAccID)
	p.WriteInt16(receiverSession)
	p.WriteAsdaStringLocale(sender, 21, c.Locale)
	p.WriteCStringLocale(msg, maxChatMessageLen, c.Locale)
	c.Send(p)
}

func readPartyInviteRequest(c *Client, p *PacketIn) (int16, byte) {
	if len(p.Data) < 2 {
		return 0, 0
	}
	// WCell Asda2GroupHandler.SendPartyInviteRequest rewinds to the
	// game-packet body at offset 20 in our stripped payload.
	if len(p.Data) >= 22 {
		sessionID := int16(binary.LittleEndian.Uint16(p.Data[20:]))
		target := getClientBySessionID(sessionID)
		if target != nil && target.Char != nil && target != c {
			inviteType := byte(0)
			if len(p.Data) > 22 {
				inviteType = p.Data[22]
			}
			return sessionID, inviteType
		}
	}
	if sessionID, inviteType, ok := findClientSessionInPacket(c, p.Data); ok {
		return sessionID, inviteType
	}
	p.Seek(0)
	sessionID := p.ReadInt16()
	inviteType := byte(0)
	if p.Remaining() > 0 {
		inviteType = p.ReadUint8()
	}
	return sessionID, inviteType
}

func readPartyInviteAnswer(p *PacketIn) bool {
	if len(p.Data) == 0 {
		return false
	}
	// WCell Asda2GroupHandler.PartyInvireAnswerRequest reads one byte after
	// Position -= 12, which maps to offset 16 after our header/opcode strip.
	if len(p.Data) > 16 {
		return p.Data[16] == 1
	}
	p.Seek(0)
	return p.ReadUint8() == 1
}

func readPartyKickRequest(p *PacketIn) uint32 {
	if len(p.Data) < 4 {
		return 0
	}
	p.Seek(0)
	return p.ReadUint32()
}

func readPartyChatRequest(c *Client, p *PacketIn) string {
	if len(p.Data) == 0 {
		return ""
	}
	p.Seek(0)
	return p.ReadCStringLocale(maxChatMessageLen, c.Locale)
}

func sendPartyInviteRequest(c *Client, senderName string) {
	sendPartyInviteRequestStatus(c, partyInviteRequestInvite, senderName)
}

func sendPartyInviteRequestStatus(c *Client, status byte, senderName string) {
	p := NewPacket(InviteToPartyResponseOrRequestToAnotherPlayer)
	p.WriteUint8(status)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint32(c.Char.AccID)
	p.WriteAsdaStringLocale(senderName, 20, c.Locale)
	p.WriteUint8(1)
	p.WriteInt16(1)
	c.Send(p)
}

func sendPartyInviteStatus(c *Client, inviter *Client, invitee *Client, status byte) {
	if c == nil || inviter == nil || inviter.Char == nil || invitee == nil || invitee.Char == nil {
		return
	}
	p := NewPacket(PartyIniteResponse)
	p.WriteUint8(status)
	p.WriteInt16(inviter.Char.SessionID)
	p.WriteUint32(inviter.Char.AccID)
	p.WriteInt16(invitee.Char.SessionID)
	p.WriteUint32(invitee.Char.AccID)
	p.WriteInt16(1)
	c.Send(p)
}

func sendPartyInfo(party *socialParty) {
	for _, member := range party.members() {
		sendPartyInfoForClient(member)
	}
}

func sendPartyUpdate(party *socialParty) {
	if party == nil {
		return
	}
	sendPartyInfo(party)
	sendPartyInitialInfo(party)
	sendPartyPositionInfo(party)
	sendPartyBuffInfo(party)
}

func sendPartyUpdateForClient(c *Client) {
	if c == nil || c.Char == nil || c.Char.PartyID == 0 {
		return
	}
	party := partyRuntime.partyFor(c.Char.GUID)
	if party == nil {
		return
	}
	sendPartyInfoForClient(c)
	for _, member := range party.members() {
		sendPartyInitialInfoForClient(c, member)
		sendPartyPositionInfoForClient(c, member)
		sendPartyBuffInfoForClient(c, member)
	}
}

func sendPartyInfoForClient(c *Client) {
	if c == nil || c.Char == nil || c.Char.PartyID == 0 {
		return
	}
	members := partyRuntime.members(c.Char.PartyID)
	p := NewPacket(PartyInfo)
	for i := 0; i < 6; i++ {
		if i < len(members) && members[i].Char != nil {
			p.WriteUint32(members[i].Char.AccID)
		} else {
			p.WriteInt32(-1)
		}
	}
	for i := 0; i < 6; i++ {
		name := ""
		if i < len(members) && members[i].Char != nil {
			name = members[i].Char.Name
		}
		p.WriteAsdaStringLocale(name, 20, c.Locale)
	}
	c.Send(p)
}

func sendPartyInitialInfo(party *socialParty) {
	for _, receiver := range party.members() {
		for _, member := range party.members() {
			sendPartyInitialInfoForClient(receiver, member)
		}
	}
}

func sendPartyInitialInfoForClient(receiver *Client, member *Client) {
	if receiver == nil || member == nil || member.Char == nil {
		return
	}
	p := NewPacket(PartyMemberInitialInfo)
	p.WriteUint32(member.Char.AccID)
	p.WriteUint8(member.Char.Level)
	p.WriteUint8(member.Char.ProfessionLevel)
	p.WriteUint8(member.Char.Class)
	p.WriteInt32(member.Char.MaxHP)
	p.WriteInt32(member.Char.HP)
	p.WriteInt16(int16(member.Char.MaxMP))
	p.WriteInt16(int16(member.Char.MP))
	p.WriteInt16(int16(asda2X(member.Char.X, member.Char.MapID)))
	p.WriteInt16(int16(asda2Y(member.Char.Y, member.Char.MapID)))
	p.WriteUint8(byte(member.Char.MapID))
	receiver.Send(p)
}

func sendPartyPositionInfo(party *socialParty) {
	for _, receiver := range party.members() {
		for _, member := range party.members() {
			sendPartyPositionInfoForClient(receiver, member)
		}
	}
}

func sendPartyPositionInfoForClient(receiver *Client, member *Client) {
	if receiver == nil || member == nil || member.Char == nil {
		return
	}
	p := NewPacket(PartyMemberPositionInfo)
	p.WriteInt16(member.Char.SessionID)
	p.WriteUint32(member.Char.AccID)
	p.WriteInt16(int16(member.Char.MapID))
	p.WriteInt16(int16(asda2X(member.Char.X, member.Char.MapID)))
	p.WriteInt16(int16(asda2Y(member.Char.Y, member.Char.MapID)))
	receiver.Send(p)
}

func sendPartyBuffInfo(party *socialParty) {
	for _, receiver := range party.members() {
		for _, member := range party.members() {
			sendPartyBuffInfoForClient(receiver, member)
		}
	}
}

func sendPartyBuffInfoForClient(receiver *Client, member *Client) {
	if receiver == nil || member == nil || member.Char == nil {
		return
	}
	p := NewPacket(PartyMemberBuffInfo)
	p.WriteUint32(member.Char.AccID)
	for i := 0; i < partyMemberBuffSlots; i++ {
		p.WriteUint8(0)
		p.WriteUint8(0)
		p.WriteInt32(-1)
	}
	receiver.Send(p)
}

func sendPartyBroken(c *Client) {
	if c == nil {
		return
	}
	p := NewPacket(PartyHasBroken)
	p.WriteInt16(1)
	c.Send(p)
}

func sendPartyMemberKicked(party *socialParty, target *Client) {
	if party == nil || target == nil || target.Char == nil {
		return
	}
	for _, member := range party.members() {
		p := NewPacket(PartyMemberKicked)
		p.WriteUint32(target.Char.AccID)
		p.WriteAsdaStringLocale(target.Char.Name, 20, member.Locale)
		member.Send(p)
	}
}

func sendPartyChatResponse(receiver *Client, sender *Client, msg string) {
	if receiver == nil || sender == nil || sender.Char == nil {
		return
	}
	p := NewPacket(PartyChatResponse)
	p.WriteInt16(sender.Char.SessionID)
	p.WriteAsdaStringLocale(sender.Char.Name, 20, receiver.Locale)
	p.WriteInt16(sender.Char.SessionID)
	p.WriteUint8(0)
	p.WriteCStringLocale(msg, maxChatMessageLen, receiver.Locale)
	receiver.Send(p)
}

func readFriendInviteRequest(p *PacketIn) int16 {
	if len(p.Data) < 2 {
		return 0
	}
	p.Seek(0)
	return p.ReadInt16()
}

func findClientSessionInPacket(sender *Client, data []byte) (int16, byte, bool) {
	if len(data) < 2 {
		return 0, 0, false
	}
	candidates := gameClientsSnapshot()
	for offset := 0; offset+1 < len(data); offset++ {
		sessionID := int16(binary.LittleEndian.Uint16(data[offset:]))
		for _, target := range candidates {
			if target == nil || target.Char == nil || target == sender {
				continue
			}
			if sender != nil && sender.Char != nil &&
				(target.Channel != sender.Channel || target.Char.MapID != sender.Char.MapID) {
				continue
			}
			if target.Char.SessionID != sessionID {
				continue
			}
			inviteType := byte(0)
			if offset+2 < len(data) {
				inviteType = data[offset+2]
			}
			return sessionID, inviteType, true
		}
	}
	return 0, 0, false
}

func readFriendInviteAnswer(p *PacketIn) (int16, bool) {
	if len(p.Data) >= 19 {
		p.Seek(16)
		sessionID := p.ReadInt16()
		return sessionID, p.ReadUint8() == 1
	}
	if len(p.Data) >= 3 {
		p.Seek(0)
		sessionID := p.ReadInt16()
		return sessionID, p.ReadUint8() == 1
	}
	return 0, false
}

func readDeleteFriendRequest(p *PacketIn) (uint32, byte) {
	if len(p.Data) < 4 {
		return 0, 0
	}
	p.Seek(0)
	accID := p.ReadUint32()
	var charNum byte
	if p.Remaining() > 0 {
		charNum = p.ReadUint8()
	}
	return accID, charNum
}

func sendFriendAdded(c *Client, success bool, friend *Client) {
	p := NewPacket(FriendAdded)
	if success {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	writeFriendSummary(p, c, friendRowFromMaybeClient(friend))
	c.Send(p)
}

func sendFriendInvite(invitee *Client, inviter *Client) {
	p := NewPacket(InviteFromeSomeoneFriend)
	p.WriteInt32(-1)
	p.WriteAsdaStringLocale(inviter.Char.Name, 20, invitee.Locale)
	p.WriteInt16(inviter.Char.SessionID)
	invitee.Send(p)
}

func sendFriendDeleted(c *Client, targetAccID uint32, charNum byte) {
	p := NewPacket(DeletedFromFriendList)
	p.WriteUint32(targetAccID)
	p.WriteUint8(charNum)
	c.Send(p)
}

func sendFriendListEntry(c *Client, row FriendRow, online *Client) {
	if online != nil && online.Char != nil {
		row = friendRowFromClient(online)
		row.Online = true
	}
	p := NewPacket(FriendList)
	writeFriendSummary(p, c, row)
	c.Send(p)
}

func writeFriendSummary(p *PacketOut, c *Client, row FriendRow) {
	p.WriteUint32(row.AccountID)
	p.WriteUint8(row.CharNum)
	p.WriteUint8(1)
	p.WriteUint8(byte(row.MapID))
	p.WriteUint8(row.Level)
	p.WriteUint8(row.ProfessionLevel)
	p.WriteUint8(row.Class)
	if row.Online {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteAsdaStringLocale(row.Name, 20, c.Locale)
}

func friendRowFromMaybeClient(c *Client) FriendRow {
	if c == nil || c.Char == nil {
		return FriendRow{}
	}
	return friendRowFromClient(c)
}
