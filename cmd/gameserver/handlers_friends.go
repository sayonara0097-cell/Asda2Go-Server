package main

import "log"

// ---- Friends ----

func handleAddFriend(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	targetSession := readFriendInviteRequest(p)
	target := getClientBySessionID(targetSession)
	if target == nil || target.Char == nil || target == c {
		sendFriendAdded(c, false, nil)
		return
	}
	friendRuntime.addInvite(c, target)
	sendFriendInvite(target, c)
}

func handleInviteFriendAnswer(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	inviterSession, accepted := readFriendInviteAnswer(p)
	inviter := getClientBySessionID(inviterSession)
	if inviter == nil {
		inviter = friendRuntime.inviterFor(c)
	}
	if inviter == nil || inviter.Char == nil || inviter == c {
		friendRuntime.clearInvite(c)
		return
	}
	if !accepted || !friendRuntime.consumeInvite(inviter, c) {
		friendRuntime.clearInvite(c)
		return
	}

	friendRuntime.addFriend(inviter, c)
	if err := AddFriendship(c.Char.GUID, inviter.Char.GUID); err != nil {
		log.Printf("[Friends] persist friendship %s/%s: %v", c.Char.Name, inviter.Char.Name, err)
	}
	sendFriendAdded(inviter, true, c)
	sendFriendAdded(c, true, inviter)
}

func handleDeleteFromFriendList(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	targetAccID, charNum := readDeleteFriendRequest(p)
	target := getClientByAccID(targetAccID)
	friendRuntime.deleteFriend(c, targetAccID)
	if target != nil && target.Char != nil {
		friendRuntime.deleteFriend(target, c.Char.AccID)
		if err := DeleteFriendship(c.Char.GUID, target.Char.GUID); err != nil {
			log.Printf("[Friends] delete friendship %s/%s: %v", c.Char.Name, target.Char.Name, err)
		}
	}
	sendFriendDeleted(c, targetAccID, charNum)
}

func handleShowFriendList(c *Client, p *PacketIn) {
	if c.Char == nil {
		return
	}
	rows, err := GetFriendsByOwner(c.Char.GUID)
	if err != nil {
		log.Printf("[Friends] load list for %s: %v", c.Char.Name, err)
	}
	sent := map[uint32]struct{}{}
	for _, row := range rows {
		online := getClientByAccID(row.AccountID)
		sendFriendListEntry(c, row, online)
		sent[row.AccountID] = struct{}{}
	}
	for _, friend := range friendRuntime.friends(c) {
		if _, ok := sent[friend.AccountID]; ok {
			continue
		}
		online := getClientByAccID(friend.AccountID)
		sendFriendListEntry(c, friend, online)
	}
	c.Send(NewPacket(FriendListEnded))
}
