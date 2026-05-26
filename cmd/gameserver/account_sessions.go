package main

import (
	"log"

	"asda2/shared/relay"
)

func claimGameAccountSession(c *Client, accountID uint32, channel byte, charNum byte, clientIP string) bool {
	disconnectExistingGameAccount(accountID, c.ID)

	token, err := relay.NewAccountSessionToken()
	if err != nil {
		log.Printf("[AccountSession] failed to create token account=%d: %v", accountID, err)
		return false
	}

	ok, err := relay.ClaimAccountSession(accountID, token, relay.AccountSessionGame, channel, charNum, clientIP, true)
	if err != nil {
		log.Printf("[AccountSession] failed to claim game account=%d: %v", accountID, err)
		return false
	}
	if !ok {
		log.Printf("[AccountSession] duplicate game login rejected account=%d charSlot=%d", accountID, charNum)
		return false
	}

	c.AccountSessionToken = token
	c.StopAccountSession = relay.StartAccountSessionHeartbeat(accountID, token, func() {
		_ = c.Conn.Close()
	})
	return true
}

func disconnectExistingGameAccount(accountID uint32, currentClientID uint32) {
	clientsMu.RLock()
	var duplicates []*Client
	for _, other := range clients {
		if other.ID == currentClientID {
			continue
		}
		if other.Char != nil && other.Char.AccID == accountID {
			duplicates = append(duplicates, other)
			continue
		}
		if other.Account != nil && other.Account.ID == accountID {
			duplicates = append(duplicates, other)
		}
	}
	clientsMu.RUnlock()

	for _, other := range duplicates {
		log.Printf("[AccountSession] kicking old game client=%d account=%d", other.ID, accountID)
		_ = other.Conn.Close()
	}
}

func releaseGameAccountSession(c *Client) {
	if c == nil {
		return
	}
	if c.StopAccountSession != nil {
		c.StopAccountSession()
		c.StopAccountSession = nil
	}
	if c.AccountSessionToken == "" {
		return
	}

	accountID := uint32(0)
	if c.Char != nil {
		accountID = c.Char.AccID
	} else if c.Account != nil {
		accountID = c.Account.ID
	}
	if err := relay.ReleaseAccountSession(accountID, c.AccountSessionToken); err != nil {
		log.Printf("[AccountSession] failed to release game account=%d: %v", accountID, err)
	}
	c.AccountSessionToken = ""
}

func detachGameAccountSessionForHandoff(c *Client) {
	if c == nil {
		return
	}
	if c.StopAccountSession != nil {
		c.StopAccountSession()
		c.StopAccountSession = nil
	}
	c.AccountSessionToken = ""
}
