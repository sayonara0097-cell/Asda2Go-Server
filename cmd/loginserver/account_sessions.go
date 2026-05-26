package main

import (
	"log"

	"asda2/shared/relay"
)

func claimLoginAccountSession(c *Client, accountID uint32, clientIP string) bool {
	disconnectExistingLoginAccount(accountID, c.ID)

	token, err := relay.NewAccountSessionToken()
	if err != nil {
		log.Printf("[AccountSession] failed to create token account=%d: %v", accountID, err)
		return false
	}

	if err := relay.ForceClaimAccountSession(accountID, token, relay.AccountSessionLogin, 0, 0, clientIP); err != nil {
		log.Printf("[AccountSession] failed to claim login account=%d: %v", accountID, err)
		return false
	}

	c.AccountSessionToken = token
	c.StopAccountSession = relay.StartAccountSessionHeartbeat(accountID, token, func() {
		_ = c.Conn.Close()
	})
	return true
}

func disconnectExistingLoginAccount(accountID uint32, currentClientID uint32) {
	clientsMu.RLock()
	var duplicates []*Client
	for _, other := range clients {
		if other.ID == currentClientID || other.Account == nil || other.Account.ID != accountID {
			continue
		}
		duplicates = append(duplicates, other)
	}
	clientsMu.RUnlock()

	for _, other := range duplicates {
		log.Printf("[AccountSession] kicking old login client=%d account=%d", other.ID, accountID)
		_ = other.Conn.Close()
	}
}

func promoteLoginAccountSessionToHandoff(c *Client, channel byte, charNum byte) bool {
	if c == nil || c.Account == nil || c.AccountSessionToken == "" {
		return false
	}
	ok, err := relay.ClaimAccountSession(
		c.Account.ID,
		c.AccountSessionToken,
		relay.AccountSessionHandoff,
		channel,
		charNum,
		remoteIP(c.Conn.RemoteAddr()),
		false,
	)
	if err != nil {
		log.Printf("[AccountSession] failed to promote account=%d to handoff: %v", c.Account.ID, err)
		return false
	}
	if !ok {
		log.Printf("[AccountSession] handoff rejected account=%d charSlot=%d", c.Account.ID, charNum)
		return false
	}
	return true
}

func releaseLoginAccountSession(c *Client) {
	if c == nil {
		return
	}
	if c.StopAccountSession != nil {
		c.StopAccountSession()
		c.StopAccountSession = nil
	}
	if c.Account == nil || c.AccountSessionToken == "" {
		return
	}
	if c.Char == nil {
		if err := relay.ReleaseAccountSession(c.Account.ID, c.AccountSessionToken); err != nil {
			log.Printf("[AccountSession] failed to release login account=%d: %v", c.Account.ID, err)
		}
	}
	c.AccountSessionToken = ""
}
