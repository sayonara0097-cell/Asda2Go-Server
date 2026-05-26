package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"

	"asda2/shared/relay"
)

type relayConnection struct {
	serverID string
	remote   string
	conn     net.Conn
	mu       sync.Mutex
}

type relayState struct {
	mu          sync.RWMutex
	servers     map[string]relay.GameServerStatus
	connections map[string]*relayConnection
}

var gameServers = &relayState{
	servers:     make(map[string]relay.GameServerStatus),
	connections: make(map[string]*relayConnection),
}

func serveRelay(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("[Relay] Listening for game servers on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[Relay] accept: %v", err)
			continue
		}
		go handleRelayConn(conn)
	}
}

func handleRelayConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	serverID := ""
	log.Printf("[Relay] game server connected: %s", remote)
	defer func() {
		if serverID != "" {
			gameServers.remove(serverID, remote)
		}
		log.Printf("[Relay] game server disconnected: %s", remote)
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var env relay.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			log.Printf("[Relay] bad envelope from %s: %v", remote, err)
			continue
		}
		switch env.Type {
		case relay.MessageRegisterGameServer:
			var msg relay.GameServerRegistration
			if err := json.Unmarshal(env.Payload, &msg); err != nil {
				log.Printf("[Relay] bad register payload from %s: %v", remote, err)
				continue
			}
			serverID = msg.ServerID
			gameServers.register(remote, conn, msg)
			log.Printf("[Relay] registered %s channel=%d endpoint=%s:%d", msg.ServerID, msg.Channel, msg.IP, msg.Port)
		case relay.MessageGameServerHeartbeat:
			var msg relay.GameServerHeartbeat
			if err := json.Unmarshal(env.Payload, &msg); err != nil {
				log.Printf("[Relay] bad heartbeat payload from %s: %v", remote, err)
				continue
			}
			if serverID == "" {
				serverID = msg.ServerID
			}
			gameServers.heartbeat(remote, msg)
		case relay.MessageWorldAnnouncement:
			var msg relay.WorldAnnouncement
			if err := json.Unmarshal(env.Payload, &msg); err != nil {
				log.Printf("[Relay] bad world announcement payload from %s: %v", remote, err)
				continue
			}
			if msg.SentAt.IsZero() {
				msg.SentAt = time.Now()
			}
			if msg.SourceServerID == "" {
				msg.SourceServerID = serverID
			}
			env, err := relay.NewEnvelope(relay.MessageWorldAnnouncement, msg)
			if err != nil {
				log.Printf("[Relay] failed to rebuild world announcement from %s: %v", remote, err)
				continue
			}
			sent, errs := gameServers.broadcastExcept(msg.SourceServerID, env)
			for _, err := range errs {
				log.Printf("[Relay] world announcement forward failed: %v", err)
			}
			log.Printf("[Relay] world announcement from %s forwarded to %d game servers: %q", msg.SourceServerID, sent, msg.Message)
		default:
			log.Printf("[Relay] unhandled message %q from %s", env.Type, remote)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[Relay] connection %s error: %v", remote, err)
	}
}

func (s *relayState) register(remote string, conn net.Conn, msg relay.GameServerRegistration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.servers[msg.ServerID] = relay.GameServerStatus{
		ServerID:      msg.ServerID,
		Channel:       msg.Channel,
		IP:            msg.IP,
		Port:          msg.Port,
		RemoteAddress: remote,
		ConnectedAt:   now,
		StartedAt:     msg.StartedAt,
		LastHeartbeat: now,
	}
	s.connections[msg.ServerID] = &relayConnection{
		serverID: msg.ServerID,
		remote:   remote,
		conn:     conn,
	}
}

func (s *relayState) heartbeat(remote string, msg relay.GameServerHeartbeat) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := s.servers[msg.ServerID]
	status.ServerID = msg.ServerID
	status.Channel = msg.Channel
	status.RemoteAddress = remote
	status.PlayerCount = msg.PlayerCount
	if status.ConnectedAt.IsZero() {
		status.ConnectedAt = time.Now()
	}
	if msg.SentAt.IsZero() {
		status.LastHeartbeat = time.Now()
	} else {
		status.LastHeartbeat = msg.SentAt
	}
	status.Players = normalizePlayerSnapshots(msg.ServerID, msg.Channel, msg.Players, status.LastHeartbeat)
	if status.PlayerCount == 0 && len(status.Players) > 0 {
		status.PlayerCount = len(status.Players)
	}
	s.servers[msg.ServerID] = status
}

func (s *relayState) remove(serverID string, remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.servers[serverID]
	if ok && current.RemoteAddress == remote {
		delete(s.servers, serverID)
	}
	currentConn, ok := s.connections[serverID]
	if ok && currentConn.remote == remote {
		delete(s.connections, serverID)
	}
}

func (s *relayState) snapshot() []relay.GameServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	out := make([]relay.GameServerStatus, 0, len(s.servers))
	for _, status := range s.servers {
		if !status.LastHeartbeat.IsZero() {
			status.AgeSeconds = int64(now.Sub(status.LastHeartbeat).Seconds())
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel == out[j].Channel {
			return out[i].ServerID < out[j].ServerID
		}
		return out[i].Channel < out[j].Channel
	})
	return out
}

func (s *relayState) playersSnapshot() []relay.PlayerSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []relay.PlayerSnapshot
	for _, status := range s.servers {
		out = append(out, status.Players...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ServerID == out[j].ServerID {
			return out[i].Character < out[j].Character
		}
		return out[i].ServerID < out[j].ServerID
	})
	return out
}

func normalizePlayerSnapshots(serverID string, channel byte, players []relay.PlayerSnapshot, updatedAt time.Time) []relay.PlayerSnapshot {
	if len(players) == 0 {
		return nil
	}
	out := make([]relay.PlayerSnapshot, len(players))
	copy(out, players)
	for i := range out {
		if out[i].ServerID == "" {
			out[i].ServerID = serverID
		}
		out[i].Channel = channel
		if out[i].UpdatedAt.IsZero() {
			out[i].UpdatedAt = updatedAt
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Character < out[j].Character
	})
	return out
}

func (s *relayState) broadcast(env relay.Envelope) (int, []error) {
	return s.broadcastExcept("", env)
}

func (s *relayState) broadcastExcept(skipServerID string, env relay.Envelope) (int, []error) {
	return s.broadcastTo("", skipServerID, env)
}

func (s *relayState) sendTo(targetServerID string, env relay.Envelope) (int, []error) {
	return s.broadcastTo(targetServerID, "", env)
}

func (s *relayState) broadcastTo(targetServerID string, skipServerID string, env relay.Envelope) (int, []error) {
	s.mu.RLock()
	peers := make([]*relayConnection, 0, len(s.connections))
	for _, peer := range s.connections {
		if targetServerID != "" && peer.serverID != targetServerID {
			continue
		}
		if skipServerID != "" && peer.serverID == skipServerID {
			continue
		}
		peers = append(peers, peer)
	}
	s.mu.RUnlock()

	sent := 0
	var errs []error
	for _, peer := range peers {
		peer.mu.Lock()
		err := json.NewEncoder(peer.conn).Encode(env)
		peer.mu.Unlock()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", peer.serverID, err))
			_ = peer.conn.Close()
			s.remove(peer.serverID, peer.remote)
			continue
		}
		sent++
	}
	return sent, errs
}
