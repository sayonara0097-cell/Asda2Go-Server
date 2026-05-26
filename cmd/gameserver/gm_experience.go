package main

import (
	"fmt"
	"log"
	"strings"

	"asda2/shared/relay"
)

func gmAddExp(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	amount, err := parseInt64Arg(cmd.Args, "amount")
	if err != nil {
		return err
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	grantManualExp(target, amount, "gm:"+cmd.RequestedBy)
	if err := SaveCharacter(target.Char); err != nil {
		return fmt.Errorf("save character %q: %w", target.Char.Name, err)
	}
	log.Printf("[GM] %s added %d xp to %q level=%d exp=%d",
		cmd.RequestedBy, amount, target.Char.Name, target.Char.Level, target.Char.Exp)
	return nil
}

func gmSetLevel(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	level, err := parseUint16Arg(cmd.Args, "level")
	if err != nil {
		return err
	}
	if level == 0 {
		return fmt.Errorf("level must be at least 1")
	}
	if level > maxAsda2Level {
		level = maxAsda2Level
	}

	setCharacterLevel(target, byte(level), "gm:"+cmd.RequestedBy)
	if err := SaveCharacter(target.Char); err != nil {
		return fmt.Errorf("save character %q: %w", target.Char.Name, err)
	}
	log.Printf("[GM] %s set %q level=%d exp=%d",
		cmd.RequestedBy, target.Char.Name, target.Char.Level, target.Char.Exp)
	return nil
}

func gmCommandTarget(cmd relay.GMCommand) (*Client, error) {
	targetName := strings.TrimSpace(cmd.Args["character"])
	if targetName == "" {
		return nil, fmt.Errorf("character is required")
	}
	target := getClientByCharacterName(targetName)
	if target == nil || target.Char == nil {
		return nil, fmt.Errorf("character %q is not online on this game server", targetName)
	}
	return target, nil
}
