package main

import (
	"fmt"
	"log"

	"asda2/shared/relay"
)

func gmGiveItem(cmd relay.GMCommand) error {
	target, err := gmCommandTarget(cmd)
	if err != nil {
		return err
	}
	itemID, err := parseInt64Arg(cmd.Args, "itemId")
	if err != nil {
		return err
	}
	if itemID <= 0 || itemID > maxInt32Value {
		return fmt.Errorf("itemId is invalid")
	}
	amount := int64(1)
	if cmd.Args["amount"] != "" {
		amount, err = parseInt64Arg(cmd.Args, "amount")
		if err != nil {
			return err
		}
	}
	if amount <= 0 || amount > maxInt32Value {
		return fmt.Errorf("amount must be between 1 and %d", maxInt32Value)
	}

	templ := itemTemplateByID(int(itemID))
	item, status, err := createCharacterItemDetailed(target.Char, int(itemID), int(amount), targetInventoryForTemplate(templ), -1, nil, 0)
	if err != nil {
		return err
	}
	if status != inventoryStatusOK || item == nil {
		return fmt.Errorf("could not add item status=%d", status)
	}
	sendSingleInventoryUpdate(target, item)
	sendInventoryWeightUpdate(target, item)

	log.Printf("[GM] %s gave item=%d amount=%d to %q inv=%d slot=%d",
		cmd.RequestedBy, itemID, amount, target.Char.Name, item.InventoryType, item.Slot)
	return nil
}
