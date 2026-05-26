package main

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"asda2/shared/relay"
)

const relayOutboxSize = 64

var (
	gameServerRelayID string
	relayOutbox       = make(chan relay.Envelope, relayOutboxSize)
)

func handleRelayMessage(env relay.Envelope) {
	switch env.Type {
	case relay.MessageWorldAnnouncement:
		var msg relay.WorldAnnouncement
		if err := json.Unmarshal(env.Payload, &msg); err != nil {
			log.Printf("[Relay] bad world announcement payload: %v", err)
			return
		}
		if msg.SourceServerID != "" && msg.SourceServerID == gameServerRelayID {
			return
		}
		sent := sendWorldAnnouncementChat(msg.Message)
		log.Printf("[Relay] world announcement delivered to %d players: %s", sent, msg.Message)
	case relay.MessageGMCommand:
		var cmd relay.GMCommand
		if err := json.Unmarshal(env.Payload, &cmd); err != nil {
			log.Printf("[Relay] bad GM command payload: %v", err)
			return
		}
		handleGMCommand(cmd)
	default:
		log.Printf("[Relay] unhandled incoming message %q", env.Type)
	}
}

func publishWorldAnnouncement(message string) error {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return errors.New("announcement message is empty")
	}
	env, err := relay.NewEnvelope(relay.MessageWorldAnnouncement, relay.WorldAnnouncement{
		Message:        msg,
		SentAt:         time.Now(),
		SourceServerID: gameServerRelayID,
	})
	if err != nil {
		return err
	}

	select {
	case relayOutbox <- env:
		return nil
	default:
		return errors.New("relay outbound queue is full")
	}
}
