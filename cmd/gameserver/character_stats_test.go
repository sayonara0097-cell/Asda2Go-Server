package main

import (
	"testing"

	"asda2/shared/types"
)

func TestCharacterStatsUseClassPrimaryAttackAttributes(t *testing.T) {
	warriorStrength := calculateCharacterStats(&Character{
		Class:        byte(types.Asda2ClassOHS),
		BaseStrength: 100,
	})
	warriorAgility := calculateCharacterStats(&Character{
		Class:       byte(types.Asda2ClassOHS),
		BaseAgility: 100,
	})
	if warriorStrength.MinDamage <= warriorAgility.MinDamage {
		t.Fatalf("warrior min damage with STR=%d, with AGI=%d; want STR to be the stronger attack stat",
			warriorStrength.MinDamage, warriorAgility.MinDamage)
	}

	archerAgility := calculateCharacterStats(&Character{
		Class:       byte(types.Asda2ClassBow),
		BaseAgility: 100,
	})
	archerStrength := calculateCharacterStats(&Character{
		Class:        byte(types.Asda2ClassBow),
		BaseStrength: 100,
	})
	if archerAgility.MinDamage <= archerStrength.MinDamage {
		t.Fatalf("archer min damage with AGI=%d, with STR=%d; want AGI to be the stronger attack stat",
			archerAgility.MinDamage, archerStrength.MinDamage)
	}
}

func TestCharacterStatsUseIntellectForMageMagicDamage(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 101, Kind: types.ItemKindWeapon, Category: types.ItemCategoryStaff, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 201, SowelBonusType: sowelBonusWeaponMAtk, SowelBonusValue: 5},
	})
	defer setItemTemplates(nil)

	mageIntellect := calculateCharacterStats(&Character{
		Class:         byte(types.Asda2ClassAttackMage),
		BaseIntellect: 100,
		Items: []*ItemRow{{
			ItemID:        101,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       201,
			Amount:        1,
		}},
	})
	mageStrength := calculateCharacterStats(&Character{
		Class:        byte(types.Asda2ClassAttackMage),
		BaseStrength: 100,
		Items: []*ItemRow{{
			ItemID:        101,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       201,
			Amount:        1,
		}},
	})

	if mageIntellect.MinDamage != 0 || mageIntellect.MaxDamage != 0 {
		t.Fatalf("mage staff physical damage = %d-%d, want staff damage routed to magic fields",
			mageIntellect.MinDamage, mageIntellect.MaxDamage)
	}
	if mageIntellect.MinMagicDamage <= mageStrength.MinMagicDamage {
		t.Fatalf("mage magic min damage with INT=%d, with STR=%d; want INT to increase magic attack",
			mageIntellect.MinMagicDamage, mageStrength.MinMagicDamage)
	}
}

func TestCharacterStatsApplyEquipmentParams(t *testing.T) {
	withoutGear := calculateCharacterStats(&Character{Class: byte(types.Asda2ClassOHS)})
	withGear := calculateCharacterStats(&Character{
		Class: byte(types.Asda2ClassOHS),
		Items: []*ItemRow{{
			ItemID:        301,
			InventoryType: types.InventoryEquipment,
			Slot:          1,
			Param1Type:    charItemBonusAttack,
			Param1Value:   8,
			Param2Type:    charItemBonusMagicDefence,
			Param2Value:   11,
			Param3Type:    charItemBonusBlockRatePercent,
			Param3Value:   3,
			Amount:        1,
		}},
	})

	if withGear.MinDamage-withoutGear.MinDamage != 8 {
		t.Fatalf("damage delta = %d, want +8 from attack option", withGear.MinDamage-withoutGear.MinDamage)
	}
	if withGear.MagicDefence != withoutGear.MagicDefence+11 {
		t.Fatalf("magic defence delta = %d, want +11 from magic defence option",
			withGear.MagicDefence-withoutGear.MagicDefence)
	}
	if withGear.BlockChance != 3 {
		t.Fatalf("block chance = %d, want 3 from block option", withGear.BlockChance)
	}
}

func TestCharacterStatsApplyDefenceRangeOptionsLikeReference(t *testing.T) {
	stats := calculateCharacterStats(&Character{
		Items: []*ItemRow{{
			ItemID:        302,
			InventoryType: types.InventoryEquipment,
			Slot:          1,
			Param1Type:    charItemBonusMinDefence,
			Param1Value:   10,
			Param2Type:    charItemBonusMaxDefence,
			Param2Value:   20,
			Amount:        1,
		}},
	})

	if stats.DefenceMin != 14 || stats.DefenceMax != 28 {
		t.Fatalf("defence range = %d-%d, want 14-28 from min/max defence options", stats.DefenceMin, stats.DefenceMax)
	}
}

func TestCharacterStatsApplySowelAttributeBonuses(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 401, Kind: types.ItemKindArmor, Category: 42, EquipmentSlot: 1},
		{ItemID: 501, SowelBonusType: sowelBonusStrength, SowelBonusValue: 10},
	})
	defer setItemTemplates(nil)

	stats := calculateCharacterStats(&Character{
		BaseStrength: 5,
		Items: []*ItemRow{{
			ItemID:        401,
			InventoryType: types.InventoryEquipment,
			Slot:          1,
			Soul2ID:       501,
			Amount:        1,
		}},
	})

	if stats.Total.Strength != 15 || stats.Bonus.Strength != 10 {
		t.Fatalf("strength total/bonus = %d/%d, want 15/10 from strength sowel", stats.Total.Strength, stats.Bonus.Strength)
	}
}
