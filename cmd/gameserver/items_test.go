package main

import (
	"encoding/binary"
	"testing"

	"asda2/shared/types"
)

func withItemPersistenceStubs(t *testing.T) {
	t.Helper()
	oldSaveItem := SaveItem
	oldDeleteItem := DeleteItem
	oldNextItemGUID := NextItemGUID
	var nextGUID int64 = 1000
	SaveItem = func(*ItemRow) error { return nil }
	DeleteItem = func(int64) error { return nil }
	NextItemGUID = func() (int64, error) {
		nextGUID++
		return nextGUID, nil
	}
	t.Cleanup(func() {
		SaveItem = oldSaveItem
		DeleteItem = oldDeleteItem
		NextItemGUID = oldNextItemGUID
	})
}

func withItemTemplates(t *testing.T, rows []ItemTemplate) {
	t.Helper()
	setItemTemplates(rows)
	t.Cleanup(func() { setItemTemplates(nil) })
}

func TestFreeRegularSlotSkipsReservedSlotZero(t *testing.T) {
	slot, ok := freeInventorySlot(&Character{}, types.InventoryRegular)
	if !ok || slot != 1 {
		t.Fatalf("free regular slot = %d,%v, want 1,true", slot, ok)
	}
}

func TestMoveRegularItemToEmptyRegularSlot(t *testing.T) {
	withItemPersistenceStubs(t)
	item := &ItemRow{
		Guid:          1,
		OwnerID:       1,
		ItemID:        20001,
		InventoryType: types.InventoryRegular,
		Slot:          1,
		Amount:        5,
		Weight:        2,
	}
	chr := &Character{GUID: 1, Items: []*ItemRow{item}}

	status, moved, swapped := moveOrSwapItem(chr, types.InventoryRegular, 1, types.InventoryRegular, 5)
	if status != inventoryStatusOK || moved != item || swapped != nil {
		t.Fatalf("move status,moved,swapped = %d,%#v,%#v; want ok,item,nil", status, moved, swapped)
	}
	if item.InventoryType != types.InventoryRegular || item.Slot != 5 || item.Amount != 5 {
		t.Fatalf("item after move = inv %d slot %d amount %d; want regular slot 5 amount 5", item.InventoryType, item.Slot, item.Amount)
	}
}

func TestSwapRegularItems(t *testing.T) {
	withItemPersistenceStubs(t)
	first := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 20001, InventoryType: types.InventoryRegular, Slot: 1, Amount: 2}
	second := &ItemRow{Guid: 2, OwnerID: 1, ItemID: 20002, InventoryType: types.InventoryRegular, Slot: 5, Amount: 7}
	chr := &Character{GUID: 1, Items: []*ItemRow{first, second}}

	status, moved, swapped := moveOrSwapItem(chr, types.InventoryRegular, 1, types.InventoryRegular, 5)
	if status != inventoryStatusOK || moved != first || swapped != second {
		t.Fatalf("swap status,moved,swapped = %d,%#v,%#v; want ok,first,second", status, moved, swapped)
	}
	if first.InventoryType != types.InventoryRegular || first.Slot != 5 {
		t.Fatalf("first after swap = inv %d slot %d; want regular slot 5", first.InventoryType, first.Slot)
	}
	if second.InventoryType != types.InventoryRegular || second.Slot != 1 {
		t.Fatalf("second after swap = inv %d slot %d; want regular slot 1", second.InventoryType, second.Slot)
	}
}

func TestMoveRegularItemRejectsReservedSlotZero(t *testing.T) {
	withItemPersistenceStubs(t)
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 20001, InventoryType: types.InventoryRegular, Slot: 1, Amount: 1}
	chr := &Character{GUID: 1, Items: []*ItemRow{item}}

	status, moved, swapped := moveOrSwapItem(chr, types.InventoryRegular, 1, types.InventoryRegular, types.ReservedRegularInventorySlot)
	if status != inventoryStatusFail || moved != item || swapped != nil {
		t.Fatalf("move to reserved status,moved,swapped = %d,%#v,%#v; want fail,item,nil", status, moved, swapped)
	}
	if item.InventoryType != types.InventoryRegular || item.Slot != 1 {
		t.Fatalf("item moved despite reserved slot rejection: inv=%d slot=%d", item.InventoryType, item.Slot)
	}
}

func TestUnequipToReservedRegularSlotAutoSelectsFreeSlot(t *testing.T) {
	withItemPersistenceStubs(t)
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 20001, InventoryType: types.InventoryEquipment, Slot: equipmentSlotWeapon, Amount: 1}
	chr := &Character{GUID: 1, Items: []*ItemRow{item}}

	status, moved, swapped := moveOrSwapItem(chr, types.InventoryEquipment, equipmentSlotWeapon, types.InventoryRegular, types.ReservedRegularInventorySlot)
	if status != inventoryStatusOK || moved != item || swapped != nil {
		t.Fatalf("unequip status,moved,swapped = %d,%#v,%#v; want ok,item,nil", status, moved, swapped)
	}
	if item.InventoryType != types.InventoryRegular || item.Slot != types.ReservedRegularInventorySlot+1 {
		t.Fatalf("item after unequip = inv %d slot %d; want regular slot 1", item.InventoryType, item.Slot)
	}
}

func TestReadPurchaseRequestsUsesReferenceSignedAmount(t *testing.T) {
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint16(raw[0:], 19573)
	binary.LittleEndian.PutUint16(raw[2:], 0xFFFF)
	binary.LittleEndian.PutUint16(raw[4:], 35415)
	binary.LittleEndian.PutUint16(raw[6:], 20001)
	binary.LittleEndian.PutUint16(raw[8:], 0)
	binary.LittleEndian.PutUint16(raw[10:], 3)

	got := readPurchaseRequests(raw)
	if len(got) != 2 {
		t.Fatalf("request count = %d, want 2", len(got))
	}
	if got[0].itemID != 19573 || got[0].amount != -30121 {
		t.Fatalf("first request = %#v, want signed negative amount", got[0])
	}
	if got[1].itemID != 20001 || got[1].amount != 3 {
		t.Fatalf("second request = %#v, want amount 3", got[1])
	}
}

func TestNormalizePurchaseAmountMatchesReferenceClamp(t *testing.T) {
	stackable := ItemTemplate{ItemID: 20001, IsStackable: true, MaxStack: 99}
	if got := normalizePurchaseAmount(stackable, -5); got != 1 {
		t.Fatalf("stackable negative amount = %d, want 1", got)
	}
	if got := normalizePurchaseAmount(stackable, 3); got != 3 {
		t.Fatalf("stackable positive amount = %d, want 3", got)
	}
	if got := normalizePurchaseAmount(stackable, 16287); got != 1 {
		t.Fatalf("stackable implausible amount = %d, want 1", got)
	}
	nonStackable := ItemTemplate{ItemID: 10001, IsStackable: false}
	if got := normalizePurchaseAmount(nonStackable, 3); got != 1 {
		t.Fatalf("non-stackable amount = %d, want 1", got)
	}
}

func TestReadPurchaseRequestsUsesMiddleAmountWhenReferenceAmountIsGarbage(t *testing.T) {
	withItemTemplates(t, []ItemTemplate{{
		ItemID:      20001,
		IsStackable: true,
		MaxStack:    99,
	}})
	raw := make([]byte, 6)
	binary.LittleEndian.PutUint16(raw[0:], 20001)
	binary.LittleEndian.PutUint16(raw[2:], 2)
	binary.LittleEndian.PutUint16(raw[4:], 16287)

	got := readPurchaseRequests(raw)
	if len(got) != 1 {
		t.Fatalf("request count = %d, want 1", len(got))
	}
	if got[0].itemID != 20001 || got[0].amount != 2 {
		t.Fatalf("request = %#v, want middle amount 2", got[0])
	}
}

func TestReadPurchaseRequestsDetectsClientHeaderBeforeStubs(t *testing.T) {
	payload := []byte{
		0x29, 0x4E, 0xB5, 0xE9, 0x5B, 0x8A, 0x00, 0x00,
		0xB6, 0x98, 0xB4, 0x28, 0x7A, 0x96, 0x6D, 0x2B,
		0x8C, 0x9F, 0x58, 0xA0, 0xDA, 0x9D, 0x0A, 0x4D,
		0xB7, 0x00, 0x00, 0x00,
		0xFA, 0x72, 0x00, 0x00, 0x01, 0x00,
		0xFA, 0x72, 0x00, 0x00, 0x01, 0x00,
		0xFA, 0x72, 0x00, 0x00, 0x01, 0x00,
	}

	got := readPurchaseRequests(payload)
	if len(got) != 3 {
		t.Fatalf("request count = %d, want 3: %#v", len(got), got)
	}
	for i, req := range got {
		if req.itemID != 29434 || req.amount != 1 {
			t.Fatalf("request %d = %#v, want item 29434 amount 1", i, req)
		}
	}
}

func TestReadSellRequestsDetectsClientHeaderBeforeStubs(t *testing.T) {
	chr := &Character{Items: []*ItemRow{{
		ItemID:        20001,
		InventoryType: types.InventoryRegular,
		Slot:          3,
		Amount:        5,
	}}}
	payload := make([]byte, 28+11)
	binary.LittleEndian.PutUint16(payload[28:], 3)
	payload[32] = types.InventoryRegular
	binary.LittleEndian.PutUint32(payload[33:], 2)

	got := readSellRequests(chr, payload)
	if len(got) != 1 {
		t.Fatalf("sell request count = %d, want 1: %#v", len(got), got)
	}
	if got[0].slot != 3 || got[0].inv != types.InventoryRegular || got[0].amount != 2 {
		t.Fatalf("sell request = %#v, want slot 3 regular amount 2", got[0])
	}
}

func TestReadMoveItemRequestDetectsNativeClientPrefix(t *testing.T) {
	item := &ItemRow{
		ItemID:        20001,
		InventoryType: types.InventoryRegular,
		Slot:          38,
		Amount:        5,
		Weight:        2,
	}
	chr := &Character{Items: []*ItemRow{item}}
	payload := make([]byte, 28+22)
	offset := 28
	binary.LittleEndian.PutUint16(payload[offset:], uint16(item.Slot))
	payload[offset+4] = types.InventoryRegular
	binary.LittleEndian.PutUint32(payload[offset+5:], uint32(item.Amount))
	binary.LittleEndian.PutUint16(payload[offset+9:], uint16(item.Weight))
	binary.LittleEndian.PutUint16(payload[offset+11:], uint16(equipmentSlotAmmo))
	payload[offset+15] = types.InventoryEquipment
	binary.LittleEndian.PutUint32(payload[offset+16:], 0)
	binary.LittleEndian.PutUint16(payload[offset+20:], 0)

	req, ok := readMoveItemRequest(chr, payload)
	if !ok {
		t.Fatalf("move request not found")
	}
	if req.srcInv != types.InventoryRegular || req.srcSlot != 38 || req.destInv != types.InventoryEquipment || req.destSlot != equipmentSlotAmmo {
		t.Fatalf("move request = %#v, want regular:38 -> equipment:%d", req, equipmentSlotAmmo)
	}
}

func TestReadUseItemRequestDetectsNativeClientPrefix(t *testing.T) {
	item := &ItemRow{
		ItemID:        32314,
		InventoryType: types.InventoryRegular,
		Slot:          4,
		Amount:        3,
	}
	chr := &Character{Items: []*ItemRow{item}}
	payload := make([]byte, 32+5)
	offset := 32
	payload[offset] = types.InventoryRegular
	binary.LittleEndian.PutUint32(payload[offset+1:], uint32(item.Slot))

	req, ok := readUseItemRequest(chr, payload)
	if !ok {
		t.Fatalf("use request not found")
	}
	if req.inv != types.InventoryRegular || req.slot != 4 {
		t.Fatalf("use request = %#v, want regular:4", req)
	}
}

func TestReadRemoveItemRequestDetectsNativeClientPrefix(t *testing.T) {
	item := &ItemRow{
		ItemID:        20001,
		InventoryType: types.InventoryRegular,
		Slot:          32,
		Amount:        3,
	}
	chr := &Character{Items: []*ItemRow{item}}
	payload := make([]byte, 32+5)
	offset := 32
	payload[offset] = types.InventoryRegular
	binary.LittleEndian.PutUint16(payload[offset+1:], uint16(item.Slot))
	binary.LittleEndian.PutUint16(payload[offset+3:], 1)

	req, ok := readRemoveItemRequest(chr, payload)
	if !ok {
		t.Fatalf("remove request not found")
	}
	if req.inv != types.InventoryRegular || req.slot != 32 || req.amount != 1 {
		t.Fatalf("remove request = %#v, want regular:32 amount 1", req)
	}
}

func TestMoveToEquipmentRejectsWrongSlot(t *testing.T) {
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        100,
		Kind:          types.ItemKindWeapon,
		InventoryType: types.InventoryRegular,
		EquipmentSlot: 9,
		Weight:        1,
		MaxStack:      1,
	}})
	chr := &Character{
		Level: 10,
		Items: []*ItemRow{{
			Guid:          1,
			OwnerID:       1,
			ItemID:        100,
			InventoryType: types.InventoryRegular,
			Slot:          1,
			Amount:        1,
			Weight:        1,
		}},
	}

	status, _, _ := moveOrSwapItem(chr, types.InventoryRegular, 1, types.InventoryEquipment, 8)
	if status != inventoryStatusItemIsNotForEquip {
		t.Fatalf("move status = %d, want %d", status, inventoryStatusItemIsNotForEquip)
	}
	if chr.Items[0].InventoryType != types.InventoryRegular || chr.Items[0].Slot != 1 {
		t.Fatalf("item moved despite failed equip: inv=%d slot=%d", chr.Items[0].InventoryType, chr.Items[0].Slot)
	}
}

func TestCreateCharacterItemRejectsOverweight(t *testing.T) {
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        200,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        uint16(types.DefaultCharacterMaxWeight + 1),
		MaxStack:      1,
	}})
	item, status, err := createCharacterItemDetailed(&Character{}, 200, 1, types.InventoryRegular, -1, nil, 0)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if status != inventoryStatusWeightExceeds || item != nil {
		t.Fatalf("create item,status = %#v,%d, want nil,%d", item, status, inventoryStatusWeightExceeds)
	}
}

func TestRemoveCharacterItemUsesTemplateStackability(t *testing.T) {
	withItemPersistenceStubs(t)
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        250,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        1,
		MaxStack:      999,
		IsStackable:   true,
	}})
	item := &ItemRow{
		Guid:          1,
		OwnerID:       1,
		ItemID:        250,
		InventoryType: types.InventoryRegular,
		Slot:          1,
		Amount:        5,
		IsStackable:   false,
		Weight:        1,
	}
	chr := &Character{GUID: 1, Items: []*ItemRow{item}}

	if err := removeCharacterItem(chr, item, 2); err != nil {
		t.Fatalf("remove returned error: %v", err)
	}
	if item.Amount != 3 || len(chr.Items) != 1 {
		t.Fatalf("amount/items = %d/%d, want 3/1", item.Amount, len(chr.Items))
	}
}

func TestPushItemsToWarehouseSplitsStackAndCopiesState(t *testing.T) {
	withItemPersistenceStubs(t)
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        300,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        2,
		MaxStack:      999,
		IsStackable:   true,
	}})
	source := &ItemRow{
		Guid:          1,
		OwnerID:       1,
		ItemID:        300,
		InventoryType: types.InventoryRegular,
		Slot:          1,
		Amount:        5,
		IsStackable:   true,
		Weight:        2,
		Enchant:       3,
	}
	chr := &Character{GUID: 1, Items: []*ItemRow{source}}

	status, sourceStubs, destStubs := pushItemsToWarehouse(chr, []warehouseItemStub{{
		slot:   1,
		inv:    types.InventoryRegular,
		amount: 2,
	}}, types.InventoryWarehouse)
	if status != warehouseStatusOK {
		t.Fatalf("warehouse status = %d, want %d", status, warehouseStatusOK)
	}
	if source.Amount != 3 || len(chr.Items) != 2 {
		t.Fatalf("source amount/items = %d/%d, want 3/2", source.Amount, len(chr.Items))
	}
	dest := chr.Items[1]
	if dest.InventoryType != types.InventoryWarehouse || dest.Amount != 2 || dest.Enchant != source.Enchant {
		t.Fatalf("dest = %#v, want warehouse copy amount=2 enchant=%d", dest, source.Enchant)
	}
	if len(sourceStubs) != 1 || sourceStubs[0].amount != 3 || len(destStubs) != 1 || destStubs[0].amount != 2 {
		t.Fatalf("stubs source=%#v dest=%#v", sourceStubs, destStubs)
	}
}

func TestTakeItemsFromWarehouseChargesGoldAndReturnsToRegular(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        400,
		Kind:          types.ItemKindMaterial,
		InventoryType: types.InventoryRegular,
		Weight:        1,
		MaxStack:      999,
		IsStackable:   true,
	}})
	source := &ItemRow{
		Guid:          1,
		OwnerID:       1,
		ItemID:        400,
		InventoryType: types.InventoryWarehouse,
		Slot:          0,
		Amount:        2,
		IsStackable:   true,
		Weight:        1,
	}
	chr := &Character{GUID: 1, Gold: 100, Items: []*ItemRow{source}}

	status, sourceStubs, destStubs := takeItemsFromWarehouse(chr, []warehouseItemStub{{
		slot:   0,
		inv:    types.InventoryWarehouse,
		amount: 1,
	}}, types.InventoryWarehouse)
	if status != warehouseStatusOK {
		t.Fatalf("warehouse take status = %d, want %d", status, warehouseStatusOK)
	}
	if chr.Gold != 70 {
		t.Fatalf("gold = %d, want 70", chr.Gold)
	}
	if source.Amount != 1 || len(chr.Items) != 2 {
		t.Fatalf("source amount/items = %d/%d, want 1/2", source.Amount, len(chr.Items))
	}
	dest := chr.Items[1]
	if dest.InventoryType != types.InventoryRegular || dest.Slot != 1 || dest.Amount != 1 {
		t.Fatalf("dest = %#v, want regular slot 1 amount 1", dest)
	}
	if len(sourceStubs) != 1 || sourceStubs[0].amount != 1 || len(destStubs) != 1 || destStubs[0].amount != 1 {
		t.Fatalf("stubs source=%#v dest=%#v", sourceStubs, destStubs)
	}
}

func TestUseHealthPotionConsumesAndHeals(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        500,
		Kind:          types.ItemKindConsumable,
		Category:      types.ItemCategoryHealthPotion,
		InventoryType: types.InventoryRegular,
		Weight:        1,
		MaxStack:      99,
		IsStackable:   true,
		ValueOnUse:    40,
	}})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 500, InventoryType: types.InventoryRegular, Slot: 1, Amount: 3, IsStackable: true}
	chr := &Character{GUID: 1, Level: 10, HP: 50, MaxHP: 80, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusOK || !out.healthChanged || item.Amount != 2 || chr.HP != 80 {
		t.Fatalf("out=%#v amount=%d hp=%d", out, item.Amount, chr.HP)
	}
}

func TestUseManaPotionConsumesAndRestoresPower(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        501,
		Kind:          types.ItemKindConsumable,
		Category:      types.ItemCategoryManaPotion,
		InventoryType: types.InventoryRegular,
		Weight:        1,
		MaxStack:      99,
		IsStackable:   true,
		ValueOnUse:    25,
	}})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 501, InventoryType: types.InventoryRegular, Slot: 1, Amount: 2, IsStackable: true}
	chr := &Character{GUID: 1, Level: 10, MP: 10, MaxMP: 30, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusOK || !out.powerChanged || item.Amount != 1 || chr.MP != 30 {
		t.Fatalf("out=%#v amount=%d mp=%d", out, item.Amount, chr.MP)
	}
}

func TestUsePackageCreatesRewardAndConsumesContainer(t *testing.T) {
	withItemPersistenceStubs(t)
	withItemTemplates(t, []ItemTemplate{
		{ItemID: 600, Kind: types.ItemKindConsumable, Category: types.ItemCategoryItemPackage, InventoryType: types.InventoryRegular, Weight: 1, MaxStack: 1, ValueOnUse: 601},
		{ItemID: 601, Kind: types.ItemKindMaterial, InventoryType: types.InventoryRegular, Weight: 1, MaxStack: 99, IsStackable: true},
	})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 600, InventoryType: types.InventoryRegular, Slot: 1, Amount: 1}
	chr := &Character{GUID: 1, Level: 10, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusOK || out.added == nil || out.added.ItemID != 601 {
		t.Fatalf("out=%#v", out)
	}
	if len(chr.Items) != 1 || chr.Items[0].ItemID != 601 {
		t.Fatalf("items=%#v, want only reward item", chr.Items)
	}
}

func TestUseExpandWarehouseConsumesAndIncrementsBagCount(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        700,
		Kind:          types.ItemKindConsumable,
		Category:      types.ItemCategoryExpandWarehouse,
		InventoryType: types.InventoryShop,
		Weight:        1,
		MaxStack:      1,
	}})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 700, InventoryType: types.InventoryShop, Slot: 1, Amount: 1}
	chr := &Character{GUID: 1, Level: 10, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusOK || !out.warehouseExpanded || chr.PremiumWarehouseBagsCount != 1 || len(chr.Items) != 0 {
		t.Fatalf("out=%#v bags=%d items=%d", out, chr.PremiumWarehouseBagsCount, len(chr.Items))
	}
}

func TestUseExpandWarehouseFailsAtMaxWithoutConsuming(t *testing.T) {
	withItemPersistenceStubs(t)
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        701,
		Kind:          types.ItemKindConsumable,
		Category:      types.ItemCategoryExpandWarehouse,
		InventoryType: types.InventoryShop,
		Weight:        1,
		MaxStack:      1,
	}})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 701, InventoryType: types.InventoryShop, Slot: 1, Amount: 1}
	chr := &Character{GUID: 1, Level: 10, PremiumWarehouseBagsCount: maxPremiumWarehouseBagsCount, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusFail || item.Amount != 1 || len(chr.Items) != 1 {
		t.Fatalf("out=%#v amount=%d items=%d", out, item.Amount, len(chr.Items))
	}
}

func TestUseChangeGenderTogglesAndConsumes(t *testing.T) {
	withItemPersistenceStubs(t)
	oldSaveCharacter := SaveCharacter
	SaveCharacter = func(*Character) error { return nil }
	t.Cleanup(func() { SaveCharacter = oldSaveCharacter })
	withItemTemplates(t, []ItemTemplate{{
		ItemID:        800,
		Kind:          types.ItemKindConsumable,
		Category:      types.ItemCategoryChangeGender,
		InventoryType: types.InventoryShop,
		Weight:        1,
		MaxStack:      1,
	}})
	item := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 800, InventoryType: types.InventoryShop, Slot: 1, Amount: 1}
	chr := &Character{GUID: 1, Level: 10, Gender: 1, Items: []*ItemRow{item}}

	out := applyInventoryItemUse(chr, item)
	if out.status != useItemStatusOK || !out.characterChanged || chr.Gender != 2 || len(chr.Items) != 0 {
		t.Fatalf("out=%#v gender=%d items=%d", out, chr.Gender, len(chr.Items))
	}
}

func TestUseRepairEquipmentRepairsAndConsumes(t *testing.T) {
	withItemPersistenceStubs(t)
	withItemTemplates(t, []ItemTemplate{
		{ItemID: 900, Kind: types.ItemKindConsumable, Category: types.ItemCategoryRepairEquipment, InventoryType: types.InventoryShop, Weight: 1, MaxStack: 1},
		{ItemID: 901, Kind: types.ItemKindWeapon, InventoryType: types.InventoryRegular, EquipmentSlot: 9, Weight: 1, MaxStack: 1, MaxDurability: 80},
	})
	scroll := &ItemRow{Guid: 1, OwnerID: 1, ItemID: 900, InventoryType: types.InventoryShop, Slot: 1, Amount: 1}
	weapon := &ItemRow{Guid: 2, OwnerID: 1, ItemID: 901, InventoryType: types.InventoryEquipment, Slot: 9, Amount: 1, Durability: 10}
	chr := &Character{GUID: 1, Level: 10, Items: []*ItemRow{scroll, weapon}}

	out := applyInventoryItemUse(chr, scroll)
	if out.status != useItemStatusOK || !out.equipmentRepaired || weapon.Durability != 80 || len(chr.Items) != 1 {
		t.Fatalf("out=%#v durability=%d items=%d", out, weapon.Durability, len(chr.Items))
	}
}

func TestReadUseShopItemRequestUsesReferenceOffsets(t *testing.T) {
	data := make([]byte, 259)
	binary.LittleEndian.PutUint16(data[0:], 99)
	binary.LittleEndian.PutUint16(data[6:], 12)
	binary.LittleEndian.PutUint32(data[255:], 701)

	slot, param, ok := readUseShopItemRequest(data)
	if !ok || slot != 12 || param != 701 {
		t.Fatalf("slot,param,ok = %d,%d,%t; want 12,701,true", slot, param, ok)
	}
}

func TestReadUseShopItemRequestSupportsShortLegacyPacket(t *testing.T) {
	data := make([]byte, 2)
	binary.LittleEndian.PutUint16(data, 7)

	slot, param, ok := readUseShopItemRequest(data)
	if !ok || slot != 7 || param != 0 {
		t.Fatalf("slot,param,ok = %d,%d,%t; want 7,0,true", slot, param, ok)
	}
}
