package main

import (
	"fmt"
	"log"

	"asda2/shared/relay"
)

func gmAddGold(cmd relay.GMCommand) error {
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
	if amount > maxInt32Value {
		return fmt.Errorf("amount is too large")
	}
	if target.Char.Gold > maxInt32Value-amount {
		return fmt.Errorf("gold would exceed %d", maxInt32Value)
	}

	target.Char.Gold += amount
	if err := SaveCharacter(target.Char); err != nil {
		return fmt.Errorf("save character %q: %w", target.Char.Name, err)
	}
	sendGMGoldPickupResponse(target, amount)

	log.Printf("[GM] %s added %d gold to %q gold=%d",
		cmd.RequestedBy, amount, target.Char.Name, target.Char.Gold)
	return nil
}

func sendGMGoldPickupResponse(c *Client, amount int64) {
	if c == nil || c.Char == nil {
		return
	}
	sendLootPickupResponse(c, pickUpStatusOK, &LootItem{
		ItemID: goldLootItemID,
		Amount: int32(amount),
	})
}
