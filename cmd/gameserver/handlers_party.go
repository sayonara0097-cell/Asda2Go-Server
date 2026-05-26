package main

import (
	"log"
	"strings"
)

// ---- Party ----

func handleSendPartyInvite(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	targetSession, inviteType := readPartyInviteRequest(c, p)
	target := getClientBySessionID(targetSession)
	if failure := partyRuntime.checkInvite(c, target); failure != partyInviteOK {
		log.Printf("[Party] invite rejected sender=%s target=%s targetSession=%d reason=%s payload=% X",
			clientDebugLabel(c), clientDebugLabel(target), targetSession, partyInviteFailureName(failure), p.Data)
		sendPartyInviteFailure(c, target, failure)
		return
	}
	inviteType = partyRuntime.inviteTypeFor(c, inviteType)
	partyRuntime.addInvite(c, target, inviteType)
	sendPartyInviteRequest(target, c.Char.Name)
	sendPartyInviteStatus(c, c, target, partyInviteStatusSent)
	log.Printf("[Party] invite sent sender=%s target=%s type=%d", clientDebugLabel(c), clientDebugLabel(target), inviteType)
}

func handlePartyInviteAnswer(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	accepted := readPartyInviteAnswer(p)
	inviter, inviteType := partyRuntime.consumeInvite(c)
	if inviter == nil || inviter.Char == nil {
		log.Printf("[Party] invite answer ignored invitee=%s accepted=%t reason=no-pending-invite payload=% X",
			clientDebugLabel(c), accepted, p.Data)
		return
	}
	log.Printf("[Party] invite answer invitee=%s inviter=%s accepted=%t type=%d payload=% X",
		clientDebugLabel(c), clientDebugLabel(inviter), accepted, inviteType, p.Data)
	if !accepted {
		sendPartyInviteStatus(inviter, inviter, c, partyInviteStatusDeclined)
		return
	}

	party := partyRuntime.acceptInvite(inviter, c, inviteType)
	if party == nil {
		log.Printf("[Party] invite accept failed invitee=%s inviter=%s reason=state-mismatch",
			clientDebugLabel(c), clientDebugLabel(inviter))
		sendPartyInviteStatus(inviter, inviter, c, partyInviteStatusNoTarget)
		return
	}
	sendPartyInviteStatus(inviter, inviter, c, partyInviteStatusAccepted)
	sendPartyInviteStatus(c, inviter, c, partyInviteStatusAccepted)
	sendPartyUpdate(party)
	log.Printf("[Party] invite accepted leader=%s invitee=%s party=%d members=%d",
		clientDebugLabel(inviter), clientDebugLabel(c), party.ID, len(party.Members))
}

func handleLeaveFromParty(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	removed, broken := partyRuntime.leave(c)
	if removed == nil {
		sendPartyBroken(c)
		return
	}
	if broken {
		for _, member := range removed {
			sendPartyBroken(member)
		}
		return
	}
	for _, member := range removed {
		sendPartyUpdateForClient(member)
	}
	sendPartyBroken(c)
}

func handleExileFromParty(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	targetAccID := readPartyKickRequest(p)
	target := getClientByAccID(targetAccID)
	party, kicked := partyRuntime.kick(c, target)
	if !kicked || target == nil || target.Char == nil {
		return
	}
	sendPartyMemberKicked(party, target)
	sendPartyBroken(target)
	if len(party.Members) <= 1 {
		for _, member := range party.members() {
			sendPartyBroken(member)
		}
		return
	}
	sendPartyUpdate(party)
}

func cleanupPartyOnDisconnect(c *Client) {
	if c == nil || c.Char == nil || c.Char.PartyID == 0 {
		return
	}
	removed, broken := partyRuntime.leave(c)
	if removed == nil {
		return
	}
	if broken {
		for _, member := range removed {
			if member != c {
				sendPartyBroken(member)
			}
		}
		return
	}
	for _, member := range removed {
		sendPartyUpdateForClient(member)
	}
}

func handlePartyChat(c *Client, p *PacketIn) {
	if c.Char == nil || c.Char.ChatBanned || c.Char.PartyID == 0 {
		return
	}
	msg := strings.TrimSpace(readPartyChatRequest(c, p))
	if msg == "" {
		return
	}
	for _, member := range partyRuntime.members(c.Char.PartyID) {
		sendPartyChatResponse(member, c, msg)
	}
}

func sendPartyInviteFailure(sender *Client, target *Client, failure partyInviteFailure) {
	if sender == nil || sender.Char == nil {
		return
	}
	switch failure {
	case partyInviteTargetInGroup, partyInviteTargetAlreadyInvited:
		if target != nil && target.Char != nil {
			sendPartyInviteRequestStatus(sender, partyInviteRequestTargetAlreadyInGroup, target.Char.Name)
			sendPartyInviteStatus(sender, sender, target, partyInviteStatusAlreadyInParty)
			return
		}
	case partyInviteFaction:
		if target != nil && target.Char != nil {
			sendPartyInviteRequestStatus(sender, partyInviteRequestOtherFaction, target.Char.Name)
			sendPartyInviteStatus(sender, sender, target, partyInviteStatusNoTarget)
			return
		}
	case partyInviteInviterNotLeader:
		sendSystemGlobalChatResponse(sender, announcementSender, "Only the party leader can invite players.")
		return
	case partyInviteInviterGroupFull:
		sendSystemGlobalChatResponse(sender, announcementSender, "Your party is full.")
		return
	case partyInviteTooManyPending:
		sendPartyInviteRequestStatus(sender, partyInviteRequestTooManyPending, "")
		return
	case partyInviteTargetRejects:
		if target != nil && target.Char != nil {
			sendSystemGlobalChatResponse(sender, announcementSender, "Sorry, but "+target.Char.Name+" rejects all party requests.")
			return
		}
	}
	sendPartyInviteRequestStatus(sender, partyInviteRequestAlreadyLogout, "")
}

func partyInviteFailureName(failure partyInviteFailure) string {
	switch failure {
	case partyInviteNoTarget:
		return "no-target"
	case partyInviteSelf:
		return "self"
	case partyInviteFaction:
		return "faction"
	case partyInviteTargetRejects:
		return "target-rejects"
	case partyInviteInviterNotLeader:
		return "inviter-not-leader"
	case partyInviteInviterGroupFull:
		return "group-full"
	case partyInviteTargetInGroup:
		return "target-in-group"
	case partyInviteTargetAlreadyInvited:
		return "target-already-invited"
	case partyInviteTooManyPending:
		return "too-many-pending"
	default:
		return "unknown"
	}
}
