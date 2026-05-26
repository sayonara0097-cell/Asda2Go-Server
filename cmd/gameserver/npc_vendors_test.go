package main

import (
	"testing"

	"asda2/shared/types"
)

func TestVendorCanSellItemUsesExplicitVendorAndGlobalStock(t *testing.T) {
	t.Cleanup(func() { setNpcVendorItems(nil, "template-fallback", false) })
	setNpcVendorItems([]types.NpcVendorItemRow{
		{VendorEntryID: 3, ItemID: 20001, IsEnabled: true},
		{VendorEntryID: 3, ItemID: 99999, IsEnabled: false},
		{VendorEntryID: 0, ItemID: 30001, IsEnabled: true},
	}, "test", true)

	if !vendorCanSellItem(3, 20001) {
		t.Fatal("vendor 3 should sell its explicit item")
	}
	if !vendorCanSellItem(9, 30001) {
		t.Fatal("global vendor stock should be available to every vendor")
	}
	if vendorCanSellItem(3, 99999) {
		t.Fatal("disabled vendor stock should not be sellable")
	}
	if vendorCanSellItem(4, 20001) {
		t.Fatal("vendor 4 should not sell vendor 3 stock")
	}
}

func TestValidateVendorPurchaseInfersNearbyVendorAndChecksStock(t *testing.T) {
	t.Cleanup(func() { setNpcVendorItems(nil, "template-fallback", false) })
	setNpcVendorItems([]types.NpcVendorItemRow{
		{VendorEntryID: 3, ItemID: 20001, IsEnabled: true},
	}, "test", true)

	c := testVisibilityClient(77, 177, 0, 0, 10, 10)
	npc := &Npc{
		SessionID:       1001,
		EntryID:         3,
		MapID:           0,
		LocalX:          10,
		LocalY:          10,
		Channel:         -1,
		InteractionKind: types.NpcInteractionVendor,
	}
	withTestNpcWorld(t, npc)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := validateVendorPurchase(c, []itemPurchaseRequest{{itemID: 20001, amount: 1}}); got != buyStatusOK {
		t.Fatalf("purchase should infer nearby vendor status=%d, want ok", got)
	}
	state, ok := currentNpcInteraction(c)
	if !ok || state.EntryID != 3 || state.Kind != types.NpcInteractionVendor {
		t.Fatalf("inferred vendor state = %#v, ok=%t", state, ok)
	}

	rememberNpcInteraction(c, npc, uint16(npc.SessionID), types.NpcInteractionVendor)
	if got := validateVendorPurchase(c, []itemPurchaseRequest{{itemID: 20001, amount: 1}}); got != buyStatusOK {
		t.Fatalf("stocked vendor purchase status=%d, want ok", got)
	}
	if got := validateVendorPurchase(c, []itemPurchaseRequest{{itemID: 20002, amount: 1}}); got != buyStatusBadItemID {
		t.Fatalf("unstocked vendor purchase status=%d, want bad item", got)
	}
	if got := validateVendorSell(c); got != itemStatusOK {
		t.Fatalf("vendor sell status=%d, want ok", got)
	}
}

func TestValidateVendorPurchaseRejectsWhenNoVendorIsNearby(t *testing.T) {
	t.Cleanup(func() { setNpcVendorItems(nil, "template-fallback", false) })
	setNpcVendorItems([]types.NpcVendorItemRow{
		{VendorEntryID: 3, ItemID: 20001, IsEnabled: true},
	}, "test", true)

	c := testVisibilityClient(78, 178, 0, 0, 10, 10)
	npc := &Npc{
		SessionID:       1001,
		EntryID:         3,
		MapID:           0,
		LocalX:          100,
		LocalY:          100,
		Channel:         -1,
		InteractionKind: types.NpcInteractionVendor,
	}
	withTestNpcWorld(t, npc)
	t.Cleanup(func() { clearNpcInteraction(c) })

	if got := validateVendorPurchase(c, []itemPurchaseRequest{{itemID: 20001, amount: 1}}); got != buyStatusFail {
		t.Fatalf("purchase without nearby vendor status=%d, want fail", got)
	}
}

func TestVendorCanSellItemFallsBackWhenNoStockLoaded(t *testing.T) {
	t.Cleanup(func() { setNpcVendorItems(nil, "template-fallback", false) })
	setNpcVendorItems(nil, "template-fallback", false)

	if !vendorCanSellItem(3, 99999) {
		t.Fatal("fallback vendor stock should preserve current item-template buying behavior")
	}
}

func withTestNpcWorld(t *testing.T, npcs ...*Npc) {
	t.Helper()
	oldWorld := World
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	for _, npc := range npcs {
		gm.npcs[npc.SessionID] = npc
	}
	World = &worldMgr{
		templates: map[uint16]*MapTemplate{0: gm.Template},
		maps:      map[uint16]*GameMap{0: gm},
	}
	t.Cleanup(func() { World = oldWorld })
}
