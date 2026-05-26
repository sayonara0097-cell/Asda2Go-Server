package main

import (
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"asda2/shared/relay"
)

var (
	clients   = map[uint32]*Client{}
	clientsMu sync.RWMutex
	nextID    uint32 = 1
)

func serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("[Game] Listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[Game] accept: %v", err)
			continue
		}
		c := NewClient(conn, ServerKindGame, router, sendToArea)
		addClient(c)
		go func() {
			defer removeClient(c)
			defer c.Close()
			c.Run()
		}()
	}
}

func addClient(c *Client) {
	clientsMu.Lock()
	c.ID = nextID
	nextID++
	clients[c.ID] = c
	clientsMu.Unlock()
	log.Printf("[+] game client %d connected (%s)", c.ID, c.Conn.RemoteAddr())
}

func removeClient(c *Client) {
	handoff := c.IsTeleporting
	if c.Char != nil {
		advanceCharacterMovement(c)
		if !handoff {
			sendTradeRejectedToAll(tradeRuntime.cancelIfActive(c))
			privateShopRuntime.cleanupForDisconnect(c)
			cleanupPartyOnDisconnect(c)
			guildRuntime.detachClient(c)
		}
		if err := SaveCharacter(c.Char); err != nil {
			log.Printf("[DB] failed to save %q on disconnect: %v", c.Char.Name, err)
		}
	}
	clearNpcInteraction(c)
	World.LeaveMap(c)
	if handoff {
		detachGameAccountSessionForHandoff(c)
	} else {
		releaseGameAccountSession(c)
	}
	clientsMu.Lock()
	delete(clients, c.ID)
	clientsMu.Unlock()
	if handoff {
		log.Printf("[Channel] game client %d detached for handoff", c.ID)
	} else {
		log.Printf("[-] game client %d disconnected", c.ID)
	}
}

func sendToArea(c *Client, p *PacketOut) {
	if c.Char == nil {
		return
	}
	for _, other := range World.AreaRecipients(c, true) {
		other.Send(p)
	}
}

func getClientBySessionID(sessionID int16) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	for _, c := range clients {
		if c.Char != nil && c.Char.SessionID == sessionID {
			return c
		}
	}
	return nil
}

func getClientByCharacterName(name string) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	for _, c := range clients {
		if c.Char != nil && strings.EqualFold(c.Char.Name, name) {
			return c
		}
	}
	return nil
}

func getClientByGUID(guid uint32) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	for _, c := range clients {
		if c.Char != nil && c.Char.GUID == guid {
			return c
		}
	}
	return nil
}

func getClientByAccID(accID uint32) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	for _, c := range clients {
		if c.Char != nil && c.Char.AccID == accID {
			return c
		}
	}
	return nil
}

func gameClientsSnapshot() []*Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	out := make([]*Client, 0, len(clients))
	for _, c := range clients {
		if c.Char != nil {
			out = append(out, c)
		}
	}
	return out
}

func countGamePlayersOnChannel(channel byte) int {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	count := 0
	for _, c := range clients {
		if c.Channel == channel && c.Char != nil {
			count++
		}
	}
	return count
}

func listGamePlayersOnChannel(channel byte) []relay.PlayerSnapshot {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	now := time.Now()
	players := make([]relay.PlayerSnapshot, 0, len(clients))
	for _, c := range clients {
		if c.Channel != channel || c.Char == nil {
			continue
		}
		players = append(players, relay.PlayerSnapshot{
			ServerID:  gameServerRelayID,
			Channel:   channel,
			AccountID: c.Char.AccID,
			SessionID: c.Char.SessionID,
			Character: c.Char.Name,
			Level:     c.Char.Level,
			MapID:     c.Char.MapID,
			X:         asda2X(c.Char.X, c.Char.MapID),
			Y:         asda2Y(c.Char.Y, c.Char.MapID),
			UpdatedAt: now,
		})
	}
	return players
}
