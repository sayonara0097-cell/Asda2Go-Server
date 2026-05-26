package main

import (
	"fmt"
	"log"
	"sync"

	"asda2/shared/types"
	"asda2/shared/worlddata"
)

type npcVendorStock struct {
	sync.RWMutex
	byVendor map[uint16]map[int]types.NpcVendorItemRow
	explicit bool
	source   string
}

var npcVendorItems = npcVendorStock{
	byVendor: make(map[uint16]map[int]types.NpcVendorItemRow),
	source:   "template-fallback",
}

func initNpcVendorRuntime() error {
	rows, source, explicit, err := loadNpcVendorRuntimeData()
	if err != nil {
		return err
	}
	setNpcVendorItems(rows, source, explicit)
	log.Printf("[NpcVendor] %d item rows loaded source=%s explicit=%t", len(rows), source, explicit)
	if !explicit {
		log.Printf("[NpcVendor] no vendor stock rows found; regular NPC vendors use item-template fallback")
	}
	return nil
}

func loadNpcVendorRuntimeData() ([]types.NpcVendorItemRow, string, bool, error) {
	rows, source, ok, err := worlddata.LoadNpcVendorItems("")
	if err != nil {
		return nil, "", false, err
	}
	if ok {
		return rows, source, true, nil
	}

	rows, err = LoadNpcVendorItems()
	if err != nil {
		return nil, "", false, fmt.Errorf("load npc vendor items: %w", err)
	}
	if len(rows) > 0 {
		return rows, "db", true, nil
	}
	return nil, "template-fallback", false, nil
}

func setNpcVendorItems(rows []types.NpcVendorItemRow, source string, explicit bool) {
	byVendor := make(map[uint16]map[int]types.NpcVendorItemRow)
	for _, row := range rows {
		if row.ItemID <= 0 || !row.IsEnabled {
			continue
		}
		items := byVendor[row.VendorEntryID]
		if items == nil {
			items = make(map[int]types.NpcVendorItemRow)
			byVendor[row.VendorEntryID] = items
		}
		items[row.ItemID] = row
	}

	npcVendorItems.Lock()
	npcVendorItems.byVendor = byVendor
	npcVendorItems.explicit = explicit
	npcVendorItems.source = source
	npcVendorItems.Unlock()
}

func vendorCanSellItem(vendorEntryID uint16, itemID int) bool {
	if itemID <= 0 {
		return false
	}
	npcVendorItems.RLock()
	defer npcVendorItems.RUnlock()
	if !npcVendorItems.explicit {
		return true
	}
	if items := npcVendorItems.byVendor[vendorEntryID]; items != nil {
		if _, ok := items[itemID]; ok {
			return true
		}
	}
	if items := npcVendorItems.byVendor[0]; items != nil {
		_, ok := items[itemID]
		return ok
	}
	return false
}

func vendorStockCount(vendorEntryID uint16) int {
	npcVendorItems.RLock()
	defer npcVendorItems.RUnlock()
	return len(npcVendorItems.byVendor[vendorEntryID])
}

func vendorStockUsesFallback() bool {
	npcVendorItems.RLock()
	defer npcVendorItems.RUnlock()
	return !npcVendorItems.explicit
}

func activeVendorNpc(c *Client) (*Npc, bool) {
	state, ok := currentNpcInteraction(c)
	if ok && state.Kind == types.NpcInteractionVendor {
		npc := resolveNpcInteractionNpc(c, state)
		if npc != nil && npcInteractionKind(npc) == types.NpcInteractionVendor && clientCanInteractWithNpc(c, npc) {
			return npc, true
		}
		clearNpcInteraction(c)
	}

	npc := nearestInteractableVendor(c)
	if npc == nil {
		return nil, false
	}
	rememberNpcInteraction(c, npc, uint16(npc.SessionID), types.NpcInteractionVendor)
	if c != nil && c.Char != nil {
		debugNpcInteractionf("inferred vendor char=%q session=%d entry=%d",
			c.Char.Name, npc.SessionID, npc.EntryID)
	}
	return npc, true
}

func nearestInteractableVendor(c *Client) *Npc {
	if c == nil || c.Char == nil {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}

	playerX := float64(asda2X(c.Char.X, c.Char.MapID))
	playerY := float64(asda2Y(c.Char.Y, c.Char.MapID))
	var nearest *Npc
	nearestDistance := 0.0
	for _, npc := range gm.Npcs() {
		if npc == nil || npcInteractionKind(npc) != types.NpcInteractionVendor || !clientCanInteractWithNpc(c, npc) {
			continue
		}
		d := distance2D(float64(npc.LocalX), float64(npc.LocalY), playerX, playerY)
		if nearest == nil || d < nearestDistance {
			nearest = npc
			nearestDistance = d
		}
	}
	return nearest
}
