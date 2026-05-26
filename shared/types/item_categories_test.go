package types

import "testing"

func TestItemTemplateUseEffectClassifiesReferenceCategories(t *testing.T) {
	cases := []struct {
		name     string
		template ItemTemplate
		want     ItemUseEffect
	}{
		{name: "health potion", template: ItemTemplate{Category: ItemCategoryHealthPotion}, want: ItemUseRecoverHP},
		{name: "mana potion", template: ItemTemplate{Category: ItemCategoryManaPotion}, want: ItemUseRecoverMP},
		{name: "fish", template: ItemTemplate{Category: ItemCategoryFish}, want: ItemUseRecoverMP},
		{name: "package", template: ItemTemplate{Category: ItemCategoryItemPackage}, want: ItemUseContainer},
		{name: "booster", template: ItemTemplate{Category: ItemCategoryBooster}, want: ItemUseBooster},
		{name: "warehouse", template: ItemTemplate{Category: ItemCategoryExpandWarehouse}, want: ItemUseExpandWarehouse},
		{name: "change gender", template: ItemTemplate{Category: ItemCategoryChangeGender}, want: ItemUseChangeGender},
		{name: "repair", template: ItemTemplate{Category: ItemCategoryRepairEquipment}, want: ItemUseRepairEquipment},
		{name: "reset all skills", template: ItemTemplate{Category: ItemCategoryResetAllSkill}, want: ItemUseResetAllSkills},
		{name: "reset one skill", template: ItemTemplate{Category: ItemCategoryResetOneSkill}, want: ItemUseResetOneSkill},
		{name: "premium buff", template: ItemTemplate{Category: ItemCategoryIncExp}, want: ItemUseFunctionalBuff},
		{name: "upgrade protect scroll is contextual", template: ItemTemplate{Category: ItemCategoryUpgradeProtectScroll}, want: ItemUseUnsupported},
		{name: "fallback consumable", template: ItemTemplate{Kind: ItemKindConsumable, ValueOnUse: 10}, want: ItemUseRecoverHP},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.template.UseEffect(); got != tc.want {
				t.Fatalf("UseEffect() = %d, want %d", got, tc.want)
			}
		})
	}
}
