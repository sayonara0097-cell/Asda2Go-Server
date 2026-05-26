package relay

import (
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChannelEndpoint struct {
	Channel byte
	IP      string
	Port    uint16
}

type PendingLogin struct {
	AccountID uint32
	CharNum   byte
	Channel   byte
	ClientIP  string
	CreatedAt time.Time
}

type Bridge struct {
	mu       sync.Mutex
	pending  map[string]PendingLogin
	channels map[byte]ChannelEndpoint
	fallback ChannelEndpoint
}

func NewBridge() *Bridge {
	return &Bridge{
		pending:  make(map[string]PendingLogin),
		channels: make(map[byte]ChannelEndpoint),
	}
}

func pendingLoginKey(accountID uint32, charNum byte) string {
	return strconv.FormatUint(uint64(accountID), 10) + ":" + strconv.Itoa(int(charNum))
}

func (b *Bridge) RegisterPendingLogin(p PendingLogin) {
	b.mu.Lock()
	p.CreatedAt = time.Now()
	b.pending[pendingLoginKey(p.AccountID, p.CharNum)] = p
	b.mu.Unlock()

	if err := SavePendingLogin(p); err != nil {
		log.Printf("[BridgeDB] failed to save pending login account=%d charSlot=%d: %v", p.AccountID, p.CharNum, err)
	}
	log.Printf("[Bridge] pending login account=%d charSlot=%d channel=%d ip=%s", p.AccountID, p.CharNum, p.Channel, p.ClientIP)
}

func (b *Bridge) ConsumePendingLogin(accountID uint32, charNum byte, clientIP string) (PendingLogin, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := pendingLoginKey(accountID, charNum)
	p, ok := b.pending[key]
	if !ok {
		return PendingLogin{}, false
	}
	delete(b.pending, key)

	if time.Since(p.CreatedAt) > 2*time.Minute {
		log.Printf("[Bridge] expired pending login account=%d charSlot=%d", accountID, charNum)
		return PendingLogin{}, false
	}
	if p.ClientIP != "" && clientIP != "" && p.ClientIP != clientIP {
		log.Printf("[Bridge] pending login ip mismatch account=%d charSlot=%d expected=%s got=%s", accountID, charNum, p.ClientIP, clientIP)
		return PendingLogin{}, false
	}
	return p, true
}

func (b *Bridge) ConsumePendingLoginAny(accountID uint32, charNum byte, clientIP string) (PendingLogin, bool) {
	if p, ok := b.ConsumePendingLogin(accountID, charNum, clientIP); ok {
		return p, true
	}
	p, ok, err := ConsumePendingLoginFromDB(accountID, charNum, clientIP)
	if err != nil {
		log.Printf("[BridgeDB] failed to consume pending login account=%d charSlot=%d: %v", accountID, charNum, err)
		return PendingLogin{}, false
	}
	if ok {
		log.Printf("[BridgeDB] consumed handoff account=%d charSlot=%d channel=%d", accountID, charNum, p.Channel)
	}
	return p, ok
}

func (b *Bridge) ConfigureChannels(defaultEndpoint ChannelEndpoint, spec string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.channels = make(map[byte]ChannelEndpoint)
	defaultEndpoint.Channel = NormalizeGameChannel(defaultEndpoint.Channel)
	b.fallback = defaultEndpoint
	b.channels[defaultEndpoint.Channel] = defaultEndpoint

	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			log.Printf("[Bridge] ignoring bad channel endpoint %q", part)
			continue
		}
		channel64, err := strconv.ParseUint(strings.TrimSpace(pieces[0]), 10, 8)
		if err != nil {
			log.Printf("[Bridge] ignoring bad channel id %q: %v", pieces[0], err)
			continue
		}
		if !ValidGameChannel(byte(channel64)) {
			log.Printf("[Bridge] ignoring unsupported channel id %d", channel64)
			continue
		}
		host, portText, err := net.SplitHostPort(strings.TrimSpace(pieces[1]))
		if err != nil {
			log.Printf("[Bridge] ignoring bad channel address %q: %v", pieces[1], err)
			continue
		}
		port64, err := strconv.ParseUint(portText, 10, 16)
		if err != nil {
			log.Printf("[Bridge] ignoring bad channel port %q: %v", portText, err)
			continue
		}
		endpoint := ChannelEndpoint{
			Channel: byte(channel64),
			IP:      host,
			Port:    uint16(port64),
		}
		b.channels[endpoint.Channel] = endpoint
	}

	for channel, endpoint := range b.channels {
		log.Printf("[Bridge] channel %d -> %s:%d", channel, endpoint.IP, endpoint.Port)
	}
}

func (b *Bridge) UpsertChannel(endpoint ChannelEndpoint) {
	if !ValidGameChannel(endpoint.Channel) {
		log.Printf("[Bridge] ignoring unsupported channel id %d", endpoint.Channel)
		return
	}
	b.mu.Lock()
	b.channels[endpoint.Channel] = endpoint
	b.mu.Unlock()
}

func (b *Bridge) RefreshChannelsFromDB() map[byte]ChannelEndpoint {
	endpoints, err := LoadOnlineChannelEndpoints(ChannelHeartbeatMaxAge)
	if err != nil {
		log.Printf("[BridgeDB] failed to refresh channels: %v", err)
		return nil
	}
	if len(endpoints) == 0 {
		return nil
	}

	online := make(map[byte]ChannelEndpoint, len(endpoints))
	b.mu.Lock()
	for _, endpoint := range endpoints {
		b.channels[endpoint.Channel] = endpoint
		online[endpoint.Channel] = endpoint
	}
	b.mu.Unlock()
	return online
}

func (b *Bridge) SelectOnlineChannel(requested byte) byte {
	requested = NormalizeGameChannel(requested)
	if online := b.RefreshChannelsFromDB(); len(online) > 0 {
		if _, ok := online[requested]; ok {
			return requested
		}
		if _, ok := online[0]; ok {
			return 0
		}
		return lowestChannel(online)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.channels[requested]; ok {
		return requested
	}
	if _, ok := b.channels[0]; ok {
		return 0
	}
	if len(b.channels) > 0 {
		return lowestChannel(b.channels)
	}
	return requested
}

func (b *Bridge) EndpointForChannel(channel byte) ChannelEndpoint {
	channel = NormalizeGameChannel(channel)
	if online := b.RefreshChannelsFromDB(); len(online) > 0 {
		if endpoint, ok := online[channel]; ok {
			return endpoint
		}
		if endpoint, ok := online[0]; ok {
			return endpoint
		}
		return endpointForLowestChannel(online)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if endpoint, ok := b.channels[channel]; ok {
		return endpoint
	}
	if endpoint, ok := b.channels[0]; ok {
		return endpoint
	}
	return b.fallback
}

func lowestChannel(channels map[byte]ChannelEndpoint) byte {
	selected := byte(0)
	found := false
	for channel := range channels {
		if !found || channel < selected {
			selected = channel
			found = true
		}
	}
	return selected
}

func endpointForLowestChannel(channels map[byte]ChannelEndpoint) ChannelEndpoint {
	return channels[lowestChannel(channels)]
}
