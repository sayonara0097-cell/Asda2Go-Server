package main

import (
	"testing"

	"asda2/shared/types"
)

func TestNormalAttackDamageRangeUsesWeaponSoulAndStats(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 100, Kind: types.ItemKindWeapon, Category: types.ItemCategoryBow, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 200, SowelBonusValue: 10},
	})
	defer setItemTemplates(nil)

	chr := &Character{
		Class:        byte(types.Asda2ClassBow),
		BaseAgility:  10,
		BaseStrength: 5,
		Items: []*ItemRow{{
			ItemID:        100,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       200,
			Amount:        1,
		}},
	}

	minDamage, maxDamage, magical := normalAttackDamageRange(chr)
	if magical {
		t.Fatal("bow attack should be physical")
	}
	if minDamage <= defaultWeaponMinDamage || maxDamage <= defaultWeaponMaxDamage {
		t.Fatalf("damage range = %d-%d, want weapon soul/stat damage above fallback", minDamage, maxDamage)
	}
	if maxDamage < minDamage {
		t.Fatalf("damage range = %d-%d, max should not be lower than min", minDamage, maxDamage)
	}
}

func TestNormalAttackDisplayStatsRoutesStaffToMagicDamage(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 101, Kind: types.ItemKindWeapon, Category: types.ItemCategoryStaff, EquipmentSlot: equipmentSlotWeapon},
		{ItemID: 201, SowelBonusValue: 8},
	})
	defer setItemTemplates(nil)

	chr := &Character{
		Class:         byte(types.Asda2ClassAttackMage),
		BaseIntellect: 20,
		Items: []*ItemRow{{
			ItemID:        101,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Soul1ID:       201,
			Amount:        1,
		}},
	}

	minDamage, maxDamage, minMagicDamage, maxMagicDamage := normalAttackDisplayStats(chr)
	if minDamage != 0 || maxDamage != 0 {
		t.Fatalf("physical stats = %d-%d, want staff damage in magic fields", minDamage, maxDamage)
	}
	if minMagicDamage <= 0 || maxMagicDamage < minMagicDamage {
		t.Fatalf("magic stats = %d-%d, want positive magic damage range", minMagicDamage, maxMagicDamage)
	}
}

func TestNormalAttackStartStatusRequiresAmmoForRangedWeapons(t *testing.T) {
	setItemTemplates([]ItemTemplate{
		{ItemID: 100, Kind: types.ItemKindWeapon, Category: types.ItemCategoryCrossbow, EquipmentSlot: equipmentSlotWeapon},
	})
	defer setItemTemplates(nil)

	c := &Client{Char: &Character{
		HP:    100,
		MapID: 0,
		Items: []*ItemRow{{
			ItemID:        100,
			InventoryType: types.InventoryEquipment,
			Slot:          equipmentSlotWeapon,
			Amount:        1,
		}},
	}}
	target := &Monster{SessionID: 20020, MapID: 0, State: MonsterStateOK, Health: 100}

	if got := normalAttackStartStatus(c, target); got != normalAttackStatusDontHaveEnoughArrows {
		t.Fatalf("start status = %d, want missing ammo status %d", got, normalAttackStatusDontHaveEnoughArrows)
	}
}

func TestDamageMonsterSubtractsDisplayedDamage(t *testing.T) {
	gm := &GameMap{monsters: map[int16]*Monster{}}
	monster := &Monster{SessionID: 20020, State: MonsterStateOK, Health: 50}
	gm.monsters[monster.SessionID] = monster
	attacker := &Client{Char: &Character{SessionID: 101, Name: "attacker"}}

	displayedDamage := int32(17)
	killed := gm.DamageMonster(attacker, monster, displayedDamage)
	if killed {
		t.Fatal("monster should survive the first hit")
	}
	if monster.Health != 33 {
		t.Fatalf("monster hp = %d, want 33 after displayed damage %d", monster.Health, displayedDamage)
	}
}
