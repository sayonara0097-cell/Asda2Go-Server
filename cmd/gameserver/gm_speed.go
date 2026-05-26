package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"asda2/shared/relay"
)

const maxGMSpeedMultiplier float32 = 20

func gmSetSpeed(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	multiplier, err := parseGMSpeedMultiplier(cmd.Args["multiplier"])
	if err != nil {
		return err
	}

	advanceCharacterMovement(target)
	target.Char.RunSpeed = defaultCharacterRunSpeed * multiplier
	sendSpeedChangedToArea(target)

	log.Printf("[GM] %s set %q speed=x%.2g runSpeed=%.3f",
		cmd.RequestedBy, target.Char.Name, multiplier, target.Char.RunSpeed)
	return nil
}

func parseGMSpeedMultiplier(value string) (float32, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "x")
	if value == "" {
		return 0, fmt.Errorf("multiplier is required")
	}

	parsed, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, fmt.Errorf("multiplier: %w", err)
	}
	multiplier := float32(parsed)
	if multiplier < 1 || multiplier > maxGMSpeedMultiplier {
		return 0, fmt.Errorf("multiplier must be between x1 and x20")
	}
	return multiplier, nil
}

func sendSpeedChangedToArea(c *Client) {
	if c == nil || c.Char == nil {
		return
	}

	p := NewPacket(SpeedChanged)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint32(c.Char.AccID)
	p.WriteFloat32(c.Char.RunSpeed)
	for _, receiver := range World.AreaRecipients(c, true) {
		receiver.Send(p)
	}
}
