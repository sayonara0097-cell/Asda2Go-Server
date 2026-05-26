package main

import "log"

const (
	pickUpStatusFail          = byte(0)
	pickUpStatusOK            = byte(1)
	pickUpStatusNoSpace       = byte(2)
	pickUpStatusWeightExceeds = byte(3)
)

// ---- Loot ----

func handlePickUpItem(c *Client, p *PacketIn) {
	if c.Char == nil {
		sendLootPickupResponse(c, pickUpStatusFail, nil)
		return
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		sendLootPickupResponse(c, pickUpStatusFail, nil)
		return
	}
	// WCell reads two short fields before X/Y after its packet cursor has
	// already passed the client preamble. Our decoded payload still includes
	// that preamble, so resolve against live loot coordinates instead of
	// relying on a fragile fixed offset.
	x, y, ok := gm.pickupCoordinatesFromPayload(p.Data)
	if !ok {
		log.Printf("[Loot] %q pickup failed: no loot coordinates in payload=% X", c.Char.Name, p.Data)
		sendLootPickupResponse(c, pickUpStatusFail, nil)
		return
	}
	loot, ok := gm.takeLootAt(x, y, c)
	if !ok {
		log.Printf("[Loot] %q pickup failed: no loot at %d,%d", c.Char.Name, x, y)
		sendLootPickupResponse(c, pickUpStatusFail, nil)
		return
	}
	if loot.ItemID == goldLootItemID {
		c.Char.Gold += int64(loot.Amount)
		_ = SaveCharacter(c.Char)
	} else {
		item, addStatus, err := createCharacterItemDetailed(c.Char, int(loot.ItemID), int(loot.Amount), 0, -1, nil, 0)
		if err != nil || addStatus != inventoryStatusOK || item == nil {
			log.Printf("[Loot] %q pickup failed: cannot add item=%d amount=%d status=%d: %v", c.Char.Name, loot.ItemID, loot.Amount, addStatus, err)
			gm.addLoot(loot)
			switch addStatus {
			case inventoryStatusNoSpace:
				sendLootPickupResponse(c, pickUpStatusNoSpace, nil)
			case inventoryStatusWeightExceeds:
				sendLootPickupResponse(c, pickUpStatusWeightExceeds, nil)
			default:
				sendLootPickupResponse(c, pickUpStatusFail, nil)
			}
			return
		}
		loot.InventoryItem = item
	}
	sendLootPickupResponse(c, pickUpStatusOK, loot)
	log.Printf("[Loot] %q picked item=%d amount=%d at %d,%d",
		c.Char.Name, loot.ItemID, loot.Amount, loot.X, loot.Y)
}
