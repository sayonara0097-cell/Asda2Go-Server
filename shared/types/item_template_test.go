package types

import "testing"

func TestFallbackItemTemplateClassifiesGoldAndStackables(t *testing.T) {
	gold := ItemTemplateByID(goldItemID)
	if gold.Kind != ItemKindCurrency || !gold.IsStackable || gold.Weight != 0 {
		t.Fatalf("gold template = %#v", gold)
	}

	material := ItemTemplateByID(31853)
	if material.Kind != ItemKindMaterial || !material.IsStackable || material.MaxStack <= 1 {
		t.Fatalf("material template = %#v", material)
	}
}

func TestItemAmountForPacketUsesTemplateStackability(t *testing.T) {
	SetItemTemplates([]ItemTemplate{{
		ItemID:      9001,
		Kind:        ItemKindMaterial,
		MaxStack:    999,
		IsStackable: true,
	}})
	defer SetItemTemplates(nil)

	item := &ItemRow{ItemID: 9001, Amount: 7, IsStackable: false}
	if got := itemAmountForPacket(item, nil, false); got != 7 {
		t.Fatalf("packet amount = %d, want 7", got)
	}
}

func TestApplyItemTemplateToRowHydratesReadableFields(t *testing.T) {
	SetItemTemplates([]ItemTemplate{{
		ItemID:      9101,
		Kind:        ItemKindMaterial,
		Weight:      4,
		MaxStack:    999,
		IsStackable: true,
	}})
	defer SetItemTemplates(nil)

	item := &ItemRow{ItemID: 9101, Amount: 3}
	ApplyItemTemplateToRow(item)
	if !item.IsStackable {
		t.Fatal("item should inherit stackability from template")
	}
	if item.Weight != 4 {
		t.Fatalf("item weight = %d, want template weight 4", item.Weight)
	}
}

func TestApplyItemTemplateToRowHealsZeroStackAmount(t *testing.T) {
	SetItemTemplates([]ItemTemplate{{
		ItemID:      9102,
		Kind:        ItemKindMaterial,
		MaxStack:    999,
		IsStackable: true,
	}})
	defer SetItemTemplates(nil)

	item := &ItemRow{ItemID: 9102, Amount: 0}
	ApplyItemTemplateToRow(item)
	if item.Amount != 1 {
		t.Fatalf("item amount = %d, want 1", item.Amount)
	}
	if got := itemAmountForPacket(item, nil, false); got != 1 {
		t.Fatalf("packet amount = %d, want 1", got)
	}
}

func TestItemAmountForPacketKeepsNonStackableEquipmentAtZero(t *testing.T) {
	SetItemTemplates([]ItemTemplate{{
		ItemID:        9002,
		Kind:          ItemKindWeapon,
		InventoryType: InventoryRegular,
		EquipmentSlot: 9,
		MaxStack:      1,
	}})
	defer SetItemTemplates(nil)

	item := &ItemRow{ItemID: 9002, Amount: 1}
	if got := itemAmountForPacket(item, nil, false); got != 0 {
		t.Fatalf("packet amount = %d, want 0 for non-stackable equipment", got)
	}
}

func TestInventorySlotValidation(t *testing.T) {
	if !IsValidInventorySlot(InventoryRegular, RegularInventorySlots-1) {
		t.Fatal("last regular slot should be valid")
	}
	if IsValidInventorySlot(InventoryRegular, RegularInventorySlots) {
		t.Fatal("slot past regular inventory should be invalid")
	}
	if !IsEquipmentInventory(InventoryEquipment) {
		t.Fatal("equipment inventory not recognized")
	}
	if !IsValidInventorySlot(InventoryWarehouse, WarehouseInventorySlots-1) {
		t.Fatal("last warehouse slot should be valid")
	}
	if IsValidInventorySlot(InventoryWarehouse, WarehouseInventorySlots) {
		t.Fatal("slot past warehouse inventory should be invalid")
	}
	if !IsValidInventorySlot(InventoryEquipment, EquipmentInventorySlots-1) {
		t.Fatal("last equipment slot should be valid")
	}
}
