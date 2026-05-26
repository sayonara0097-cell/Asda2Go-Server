package main

import (
	"fmt"
	"log"
	"strings"

	"asda2/shared/relay"
)

const maxInt32Value = int64(1<<31 - 1)

func gmHealPlayer(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	amount, err := gmHealthAmount(cmd, target.Char.MaxHP)
	if err != nil {
		return err
	}

	healCharacter(target, amount, "gm:"+cmd.RequestedBy)
	if err := SaveCharacter(target.Char); err != nil {
		return fmt.Errorf("save character %q: %w", target.Char.Name, err)
	}
	log.Printf("[GM] %s healed %q amount=%d hp=%d/%d",
		cmd.RequestedBy, target.Char.Name, amount, target.Char.HP, target.Char.MaxHP)
	return nil
}

func gmDamagePlayer(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	amount, err := gmHealthAmount(cmd, defaultMonsterAttackDamage)
	if err != nil {
		return err
	}

	damageCharacter(target, amount, "gm:"+cmd.RequestedBy)
	if err := SaveCharacter(target.Char); err != nil {
		return fmt.Errorf("save character %q: %w", target.Char.Name, err)
	}
	log.Printf("[GM] %s damaged %q amount=%d hp=%d/%d",
		cmd.RequestedBy, target.Char.Name, amount, target.Char.HP, target.Char.MaxHP)
	return nil
}

func gmHealthAmount(cmd relay.GMCommand, defaultAmount int32) (int32, error) {
	if value := strings.TrimSpace(cmd.Args["amount"]); value == "" || value == "0" {
		return defaultAmount, nil
	}
	amount, err := parseInt64Arg(cmd.Args, "amount")
	if err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	if amount > maxInt32Value {
		return 0, fmt.Errorf("amount is too large")
	}
	return int32(amount), nil
}
