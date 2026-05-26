package main

import (
	"testing"

	"asda2/shared/types"
)

func TestTradeRuntimeTransfersRegularItems(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        31001,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        2,
		MaxStack:      999,
		IsStackable:   true,
	}})

	alice := testSocialClient(1, 1001, "Alice")
	bob := testSocialClient(2, 1002, "Bob")
	item := &ItemRow{
		Guid:          10,
		OwnerID:       alice.Char.GUID,
		ItemID:        31001,
		InventoryType: types.InventoryRegular,
		Slot:          1,
		Amount:        5,
		IsStackable:   true,
		Weight:        2,
	}
	alice.Char.Items = []*ItemRow{item}

	m := &tradeManager{byChar: make(map[uint32]*tradeSession)}
	if !m.begin(alice, bob, tradeTypeRegular) {
		t.Fatal("expected trade to begin")
	}
	if status, _, _ := m.pushItem(alice, types.InventoryRegular, 1, 2); status != pushTradeStatusOK {
		t.Fatalf("push status=%d", status)
	}
	m.showItems(alice)
	m.showItems(bob)
	if session, _, _, err := m.confirm(alice); session != nil || err != nil {
		t.Fatalf("first confirm session=%v err=%v", session, err)
	}
	session, result, canceled, err := m.confirm(bob)
	if err != nil || len(canceled) != 0 || session == nil {
		t.Fatalf("final confirm session=%v canceled=%d err=%v", session, len(canceled), err)
	}
	if item.Amount != 3 {
		t.Fatalf("source amount=%d, want 3", item.Amount)
	}
	if len(bob.Char.Items) != 1 || bob.Char.Items[0].ItemID != 31001 || bob.Char.Items[0].Amount != 2 {
		t.Fatalf("bob items=%#v", bob.Char.Items)
	}
	if result.SecondReceives[0] == nil || result.SecondReceives[0].Amount != 2 {
		t.Fatalf("received=%#v", result.SecondReceives[0])
	}
}

func TestTradeRuntimeLocksPushedItemsUntilCancel(t *testing.T) {
	alice := testSocialClient(1, 1001, "Alice")
	bob := testSocialClient(2, 1002, "Bob")
	item := &ItemRow{Guid: 10, OwnerID: alice.Char.GUID, ItemID: 100, InventoryType: types.InventoryRegular, Slot: 1, Amount: 1}
	alice.Char.Items = []*ItemRow{item}

	m := &tradeManager{byChar: make(map[uint32]*tradeSession)}
	m.begin(alice, bob, tradeTypeRegular)
	if status, _, _ := m.pushItem(alice, types.InventoryRegular, 1, 1); status != pushTradeStatusOK {
		t.Fatalf("push status=%d", status)
	}
	if !m.itemLocked(alice.Char, item) {
		t.Fatal("expected item to be locked")
	}
	m.cancelFor(alice)
	if m.itemLocked(alice.Char, item) {
		t.Fatal("expected item lock to be released")
	}
}

func TestPrivateShopRuntimeBuyTransfersGoldAndItems(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        32001,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        1,
		MaxStack:      999,
		IsStackable:   true,
	}})

	seller := testSocialClient(1, 1001, "Seller")
	buyer := testSocialClient(2, 1002, "Buyer")
	buyer.Char.Gold = 100
	item := &ItemRow{
		Guid:          20,
		OwnerID:       seller.Char.GUID,
		ItemID:        32001,
		InventoryType: types.InventoryRegular,
		Slot:          1,
		Amount:        5,
		IsStackable:   true,
		Weight:        1,
	}
	seller.Char.Items = []*ItemRow{item}

	m := &privateShopManager{
		ownerShops:  make(map[uint32]*privateShop),
		joinedShops: make(map[uint32]*privateShop),
	}
	if status := m.openWindow(seller); status != privateShopWindowOK {
		t.Fatalf("open window status=%d", status)
	}
	var refs [privateShopItemSlots]*tradeItemRef
	refs[0] = &tradeItemRef{Item: item, Amount: 5, Price: 10, InventoryType: types.InventoryRegular, Slot: 1}
	if status, _ := m.start(seller, refs, "materials"); status != privateShopOpenedOK {
		t.Fatalf("start status=%d", status)
	}
	if status, _, _ := m.view(buyer, seller.Char.AccID); status != privateShopInfoOK {
		t.Fatalf("view status=%d", status)
	}
	status, shop, sold, bought, err := m.buy(buyer, []privateShopBuyRequest{{TradeSlot: 0, Amount: 2}})
	if err != nil || status != privateShopBuyOK {
		t.Fatalf("buy status=%d err=%v", status, err)
	}
	if buyer.Char.Gold != 80 || seller.Char.Gold != 20 {
		t.Fatalf("gold buyer=%d seller=%d", buyer.Char.Gold, seller.Char.Gold)
	}
	if item.Amount != 3 || len(buyer.Char.Items) != 1 || buyer.Char.Items[0].Amount != 2 {
		t.Fatalf("seller amount=%d buyer items=%#v", item.Amount, buyer.Char.Items)
	}
	if shop.Items[0].Amount != 3 || len(sold) != 1 || len(bought) != 1 {
		t.Fatalf("shop amount=%d sold=%d bought=%d", shop.Items[0].Amount, len(sold), len(bought))
	}
}
