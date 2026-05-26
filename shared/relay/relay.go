package relay

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"
)

type MessageType string

const (
	MessageRegisterGameServer  MessageType = "register_game_server"
	MessageGameServerHeartbeat MessageType = "game_server_heartbeat"
	MessageWorldAnnouncement   MessageType = "world_announcement"
	MessageCrossChannelChat    MessageType = "cross_channel_chat"
	MessagePlayerOnlineStatus  MessageType = "player_online_status"
	MessageGMCommand           MessageType = "gm_command"
)

const RelayHeartbeatInterval = 10 * time.Second

type GameServerRegistration struct {
	ServerID  string    `json:"serverId"`
	Channel   byte      `json:"channel"`
	IP        string    `json:"ip"`
	Port      uint16    `json:"port"`
	StartedAt time.Time `json:"startedAt"`
}

type GameServerHeartbeat struct {
	ServerID    string           `json:"serverId"`
	Channel     byte             `json:"channel"`
	PlayerCount int              `json:"playerCount"`
	SentAt      time.Time        `json:"sentAt"`
	Players     []PlayerSnapshot `json:"players,omitempty"`
}

type GameServerStatus struct {
	ServerID      string           `json:"serverId"`
	Channel       byte             `json:"channel"`
	IP            string           `json:"ip"`
	Port          uint16           `json:"port"`
	RemoteAddress string           `json:"remoteAddress"`
	PlayerCount   int              `json:"playerCount"`
	ConnectedAt   time.Time        `json:"connectedAt"`
	StartedAt     time.Time        `json:"startedAt"`
	LastHeartbeat time.Time        `json:"lastHeartbeat"`
	AgeSeconds    int64            `json:"ageSeconds"`
	Players       []PlayerSnapshot `json:"players,omitempty"`
}

type PlayerSnapshot struct {
	ServerID  string    `json:"serverId"`
	Channel   byte      `json:"channel"`
	AccountID uint32    `json:"accountId"`
	SessionID int16     `json:"sessionId"`
	Character string    `json:"character"`
	Level     byte      `json:"level"`
	MapID     uint16    `json:"mapId"`
	X         float32   `json:"x"`
	Y         float32   `json:"y"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WorldAnnouncement struct {
	Message        string    `json:"message"`
	SentAt         time.Time `json:"sentAt"`
	SourceServerID string    `json:"sourceServerId,omitempty"`
}

type CrossChannelChat struct {
	Channel   byte      `json:"channel"`
	Sender    string    `json:"sender"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type PlayerOnlineStatus struct {
	AccountID uint32    `json:"accountId"`
	Character string    `json:"character"`
	Channel   byte      `json:"channel"`
	Online    bool      `json:"online"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type GMCommand struct {
	Action         string            `json:"action"`
	Args           map[string]string `json:"args,omitempty"`
	TargetServerID string            `json:"targetServerId,omitempty"`
	RequestedBy    string            `json:"requestedBy"`
	AccountID      uint32            `json:"accountId"`
	CreatedAt      time.Time         `json:"createdAt"`
}

type Envelope struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type MessageHandler func(Envelope)

type Client struct {
	addr     string
	serverID string
	conn     net.Conn
	mu       sync.Mutex
}

func Dial(addr string, serverID string) (*Client, error) {
	if addr == "" {
		return &Client{serverID: serverID}, nil
	}
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	log.Printf("[Relay] connected to %s as %s", addr, serverID)
	return &Client{addr: addr, serverID: serverID, conn: conn}, nil
}

func StartClient(addr string, serverID string) func() {
	return StartDialLoop(addr, serverID, 10*time.Second)
}

func StartDialLoop(addr string, serverID string, retryEvery time.Duration) func() {
	stop := make(chan struct{})
	if addr == "" {
		return func() {}
	}

	go func() {
		var client *Client
		defer func() {
			if client != nil {
				client.Close()
			}
		}()

		for {
			var err error
			client, err = Dial(addr, serverID)
			if err == nil {
				<-stop
				return
			}
			log.Printf("[Relay] unavailable at %s: %v; retrying in %s", addr, err, retryEvery)

			select {
			case <-stop:
				return
			case <-time.After(retryEvery):
			}
		}
	}()

	return func() {
		close(stop)
	}
}

func StartGameServerClient(addr string, registration GameServerRegistration, countPlayers PlayerCounter, handlers ...MessageHandler) func() {
	return StartGameServerClientWithOutbox(addr, registration, countPlayers, nil, handlers...)
}

func StartGameServerClientWithOutbox(addr string, registration GameServerRegistration, countPlayers PlayerCounter, outgoing <-chan Envelope, handlers ...MessageHandler) func() {
	return StartGameServerClientWithOutboxAndPlayers(addr, registration, countPlayers, nil, outgoing, handlers...)
}

func StartGameServerClientWithOutboxAndPlayers(addr string, registration GameServerRegistration, countPlayers PlayerCounter, listPlayers PlayerLister, outgoing <-chan Envelope, handlers ...MessageHandler) func() {
	return StartGameServerClientLoop(addr, registration, countPlayers, listPlayers, RelayHeartbeatInterval, firstHandler(handlers), outgoing)
}

func StartGameServerClientLoop(addr string, registration GameServerRegistration, countPlayers PlayerCounter, listPlayers PlayerLister, heartbeatEvery time.Duration, handle MessageHandler, outgoing <-chan Envelope) func() {
	stop := make(chan struct{})
	if addr == "" {
		return func() {}
	}

	go func() {
		for {
			client, err := Dial(addr, registration.ServerID)
			if err != nil {
				if !waitForRelayRetry(stop, addr, err, 10*time.Second) {
					return
				}
				continue
			}

			reg := registration
			if reg.StartedAt.IsZero() {
				reg.StartedAt = time.Now()
			}
			if err := client.SendPayload(MessageRegisterGameServer, reg); err != nil {
				log.Printf("[Relay] failed to register %s: %v", reg.ServerID, err)
				client.Close()
				if !waitForRelayRetry(stop, addr, err, 10*time.Second) {
					return
				}
				continue
			}
			if err := sendGameServerHeartbeat(client, reg, countPlayers, listPlayers); err != nil {
				log.Printf("[Relay] initial heartbeat to %s failed: %v", addr, err)
				client.Close()
				if !waitForRelayRetry(stop, addr, err, 10*time.Second) {
					return
				}
				continue
			}

			ticker := time.NewTicker(heartbeatEvery)
			readDone := make(chan error, 1)
			go func() {
				readDone <- client.ReadLoop(handle)
			}()

			connected := true
			for connected {
				select {
				case <-stop:
					ticker.Stop()
					client.Close()
					return
				case err := <-readDone:
					if err != nil {
						log.Printf("[Relay] read loop from %s failed: %v", addr, err)
					} else {
						log.Printf("[Relay] connection to %s closed", addr)
					}
					connected = false
				case env, ok := <-outgoing:
					if !ok {
						outgoing = nil
						continue
					}
					if err := client.Send(env); err != nil {
						log.Printf("[Relay] send to %s failed: %v", addr, err)
						connected = false
					}
				case <-ticker.C:
					if err := sendGameServerHeartbeat(client, reg, countPlayers, listPlayers); err != nil {
						log.Printf("[Relay] heartbeat to %s failed: %v", addr, err)
						connected = false
					}
				}
			}
			ticker.Stop()
			client.Close()
		}
	}()

	return func() {
		close(stop)
	}
}

func firstHandler(handlers []MessageHandler) MessageHandler {
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func waitForRelayRetry(stop <-chan struct{}, addr string, err error, retryEvery time.Duration) bool {
	log.Printf("[Relay] unavailable at %s: %v; retrying in %s", addr, err, retryEvery)
	select {
	case <-stop:
		return false
	case <-time.After(retryEvery):
		return true
	}
}

func playerCount(channel byte, countPlayers PlayerCounter) int {
	if countPlayers == nil {
		return 0
	}
	return countPlayers(channel)
}

func sendGameServerHeartbeat(client *Client, reg GameServerRegistration, countPlayers PlayerCounter, listPlayers PlayerLister) error {
	heartbeat := GameServerHeartbeat{
		ServerID:    reg.ServerID,
		Channel:     reg.Channel,
		PlayerCount: playerCount(reg.Channel, countPlayers),
		SentAt:      time.Now(),
		Players:     playerSnapshots(reg.ServerID, reg.Channel, listPlayers),
	}
	return client.SendPayload(MessageGameServerHeartbeat, heartbeat)
}

func playerSnapshots(serverID string, channel byte, listPlayers PlayerLister) []PlayerSnapshot {
	if listPlayers == nil {
		return nil
	}
	players := listPlayers(channel)
	now := time.Now()
	for i := range players {
		if players[i].ServerID == "" {
			players[i].ServerID = serverID
		}
		players[i].Channel = channel
		if players[i].UpdatedAt.IsZero() {
			players[i].UpdatedAt = now
		}
	}
	return players
}

func (c *Client) Close() {
	if c == nil || c.conn == nil {
		return
	}
	_ = c.conn.Close()
}

func (c *Client) SendPayload(messageType MessageType, payload any) error {
	env, err := NewEnvelope(messageType, payload)
	if err != nil {
		return err
	}
	return c.Send(env)
}

func (c *Client) ReadLoop(handle MessageHandler) error {
	if c == nil || c.conn == nil {
		return nil
	}
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			log.Printf("[Relay] bad incoming envelope: %v", err)
			continue
		}
		if handle != nil {
			handle(env)
		}
	}
	return scanner.Err()
}

func (c *Client) Send(env Envelope) error {
	if c == nil || c.conn == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.NewEncoder(c.conn).Encode(env)
}

func NewEnvelope(messageType MessageType, payload any) (Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: messageType, Payload: data}, nil
}
