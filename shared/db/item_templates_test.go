package db

import (
	"testing"

	"asda2/shared/types"
)

func TestLegacyItemKindMapsEquipmentAndStackables(t *testing.T) {
	cases := []struct {
		name          string
		category      int
		equipmentSlot int
		want          types.ItemKind
	}{
		{name: "weapon", category: 1, equipmentSlot: 9, want: types.ItemKindWeapon},
		{name: "armor", category: 64, equipmentSlot: 0, want: types.ItemKindArmor},
		{name: "avatar", category: 54, equipmentSlot: 11, want: types.ItemKindAvatar},
		{name: "accessory", category: 44, equipmentSlot: 5, want: types.ItemKindAccessory},
		{name: "material", category: 91, equipmentSlot: -1, want: types.ItemKindMaterial},
		{name: "consumable", category: 29, equipmentSlot: -1, want: types.ItemKindConsumable},
		{name: "sowel", category: 107, equipmentSlot: -1, want: types.ItemKindSowel},
		{name: "preserve unknown equipment slot", category: 999, equipmentSlot: 3, want: types.ItemKindAccessory},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyItemKind(tc.category, tc.equipmentSlot); got != tc.want {
				t.Fatalf("legacyItemKind(%d, %d) = %d, want %d", tc.category, tc.equipmentSlot, got, tc.want)
			}
		})
	}
}

func TestDBTemplateClampHelpers(t *testing.T) {
	if got := byteFromDB(300); got != 255 {
		t.Fatalf("byteFromDB(300) = %d, want 255", got)
	}
	if got := uint16FromDB(70000); got != 65535 {
		t.Fatalf("uint16FromDB(70000) = %d, want 65535", got)
	}
	if got := int16FromDB(-40000); got != -32768 {
		t.Fatalf("int16FromDB(-40000) = %d, want -32768", got)
	}
}
