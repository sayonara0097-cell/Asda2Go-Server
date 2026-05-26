package main

import (
	"math"
	"testing"

	"asda2/shared/types"
)

func TestCanUseUpgradeStoneMatchesReferenceLevelCaps(t *testing.T) {
	types.SetItemTemplates([]types.ItemTemplate{
		{ItemID: 1001, Kind: types.ItemKindWeapon, RequiredLevel: 20},
		{ItemID: 1002, Kind: types.ItemKindWeapon, RequiredLevel: 21},
		{ItemID: 1003, Kind: types.ItemKindArmor, RequiredLevel: 80},
		{ItemID: 2001, Category: types.ItemCategoryEnchantWeaponStoneD},
		{ItemID: 2002, Category: types.ItemCategoryEnchantArmorStoneA},
		{ItemID: 2003, Category: types.ItemCategoryEnchantUniversalStoneA},
	})
	t.Cleanup(func() { types.SetItemTemplates(nil) })

	if !canUseUpgradeStone(&ItemRow{ItemID: 1001}, &ItemRow{ItemID: 2001}) {
		t.Fatal("weapon D stone should work on level 20 weapon")
	}
	if canUseUpgradeStone(&ItemRow{ItemID: 1002}, &ItemRow{ItemID: 2001}) {
		t.Fatal("weapon D stone should reject level 21 weapon")
	}
	if !canUseUpgradeStone(&ItemRow{ItemID: 1003}, &ItemRow{ItemID: 2002}) {
		t.Fatal("armor A stone should work on level 80 armor")
	}
	if !canUseUpgradeStone(&ItemRow{ItemID: 1003}, &ItemRow{ItemID: 2003}) {
		t.Fatal("universal A stone should work on level 80 armor")
	}
}

func TestAdvancedEnchantRequirementsMatchReference(t *testing.T) {
	types.SetItemTemplates([]types.ItemTemplate{
		{ItemID: 3001, Kind: types.ItemKindWeapon, Quality: types.ItemQualityGreen, AuctionLevel: 3},
		{ItemID: 3002, Kind: types.ItemKindWeapon, Quality: types.ItemQualityPurple, AuctionLevel: 4},
	})
	t.Cleanup(func() { types.SetItemTemplates(nil) })

	cost, first, second, third, ok := advancedEnchantCostAndMaterials(&ItemRow{ItemID: 3001})
	if !ok || cost != 400_000 || first != 2 || second != 4 || third != 12 {
		t.Fatalf("green auction level 3 requirements = cost:%d %d/%d/%d ok:%v", cost, first, second, third, ok)
	}
	if !advancedEnchantMaterialsMatch(&ItemRow{ItemID: 3001}, &ItemRow{ItemID: 33706}, &ItemRow{ItemID: 20681}, &ItemRow{ItemID: 33705}) {
		t.Fatal("green advanced enchant materials should match reference ids")
	}

	cost, first, second, third, ok = advancedEnchantCostAndMaterials(&ItemRow{ItemID: 3002})
	if !ok || cost != 800_000 || first != 3 || second != 5 || third != 15 {
		t.Fatalf("purple auction level 4 requirements = cost:%d %d/%d/%d ok:%v", cost, first, second, third, ok)
	}
	if !advancedEnchantMaterialsMatch(&ItemRow{ItemID: 3002}, &ItemRow{ItemID: 20681}, &ItemRow{ItemID: 20680}, &ItemRow{ItemID: 33705}) {
		t.Fatal("purple advanced enchant materials should match reference ids")
	}
}

func TestCalculateEnchantMultiplierNotDamageItemStatsUsesReferencePower(t *testing.T) {
	got := calculateEnchantMultiplierNotDamageItemStats(20)
	want := math.Pow(20, 0.065)
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("multiplier = %f, want %f", got, want)
	}
}

func TestReserveCraftMaterialsAcrossStacks(t *testing.T) {
	chr := &Character{
		Items: []*ItemRow{
			{Guid: 1, ItemID: 5001, InventoryType: types.InventoryRegular, Slot: 1, Amount: 3},
			{Guid: 2, ItemID: 5001, InventoryType: types.InventoryRegular, Slot: 2, Amount: 4},
			{Guid: 3, ItemID: 5002, InventoryType: types.InventoryRegular, Slot: 3, Amount: 2},
		},
	}
	materials, amounts, ok := reserveCraftMaterials(chr, []CraftMaterialRow{
		{ItemID: 5001, Amount: 5},
		{ItemID: 5002, Amount: 2},
	})
	if !ok {
		t.Fatal("expected material reservation to succeed")
	}
	if len(materials) != 3 || len(amounts) != 3 {
		t.Fatalf("reserved stacks = %d amounts = %d, want 3", len(materials), len(amounts))
	}
	if materials[0].Guid != 1 || amounts[0] != 3 || materials[1].Guid != 2 || amounts[1] != 2 || materials[2].Guid != 3 || amounts[2] != 2 {
		t.Fatalf("reservation = %#v amounts=%#v", materials, amounts)
	}
}
