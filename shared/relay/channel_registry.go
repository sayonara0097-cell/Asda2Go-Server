package relay

import (
	"log"
	"time"
)

const (
	ChannelHeartbeatInterval = 5 * time.Second
	ChannelHeartbeatMaxAge   = 30 * time.Second
)

type PlayerCounter func(channel byte) int
type PlayerLister func(channel byte) []PlayerSnapshot

func StartChannelHeartbeat(endpoint ChannelEndpoint, countPlayers PlayerCounter) {
	go func() {
		log.Printf("[Channel] heartbeat started for channel %d at %s:%d", endpoint.Channel, endpoint.IP, endpoint.Port)
		sendChannelHeartbeat(endpoint, countPlayers)

		ticker := time.NewTicker(ChannelHeartbeatInterval)
		defer ticker.Stop()
		for range ticker.C {
			sendChannelHeartbeat(endpoint, countPlayers)
		}
	}()
}

func sendChannelHeartbeat(endpoint ChannelEndpoint, countPlayers PlayerCounter) {
	playerCount := 0
	if countPlayers != nil {
		playerCount = countPlayers(endpoint.Channel)
	}
	if err := SaveChannelHeartbeat(endpoint, playerCount); err != nil {
		log.Printf("[Channel] heartbeat failed for channel %d: %v", endpoint.Channel, err)
	}
}
