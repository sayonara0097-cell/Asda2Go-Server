package db

import (
	"asda2/shared/types"
	"fmt"
	"log"
	"strings"
)

func LoadItemTemplates() ([]ItemTemplate, error) {
	rows, _, err := loadItemTemplatesWithSource()
	return rows, err
}

func loadItemTemplatesWithSource() ([]ItemTemplate, string, error) {
	if DB == nil {
		return nil, "none", nil
	}
	rows, err := loadCanonicalItemTemplates()
	if err != nil {
		return nil, "Asda2ItemTemplate", err
	}
	if len(rows) > 0 {
		return rows, "Asda2ItemTemplate", nil
	}
	rows, err = loadLegacyItemTemplates()
	if err != nil {
		return nil, "asda2itemtemlate", err
	}
	if len(rows) > 0 {
		return rows, "asda2itemtemlate", nil
	}
	return nil, "fallback", nil
}

func loadCanonicalItemTemplates() ([]ItemTemplate, error) {
	categoryExpr := selectColumnOrDefault("Asda2ItemTemplate", "Category", "0")
	qualityExpr := selectColumnOrDefault("Asda2ItemTemplate", "Quality", "0")
	auctionLevelExpr := selectColumnOrDefault("Asda2ItemTemplate", "AuctionLevelCriterion", "0")
	attackTimeExpr := selectColumnOrDefault("Asda2ItemTemplate", "AttackTime", "0")
	attackRangeExpr := selectColumnOrDefault("Asda2ItemTemplate", "AttackRange", "0")
	sowelTypeExpr := selectColumnOrDefault("Asda2ItemTemplate", "SowelBonusType", "0")
	sowelBonusExpr := selectColumnOrDefault("Asda2ItemTemplate", "SowelBonusValue", "0")
	query := fmt.Sprintf(`SELECT ItemId, Name, Kind, %s, %s, %s, InventoryType, EquipmentSlot,
		        RequiredLevel, RequiredProfession, MaxDurability, Weight,
		        BuyPrice, SellPrice, MaxStack, IsStackable, SowelSockets, ValueOnUse,
		        %s, %s, %s, %s
		   FROM Asda2ItemTemplate
		  WHERE IsEnabled = 1`, categoryExpr, qualityExpr, auctionLevelExpr, attackTimeExpr, attackRangeExpr, sowelTypeExpr, sowelBonusExpr)
	rows, err := DB.Query(
		query,
	)
	if err != nil {
		if missingItemTemplateTable(err, "asda2itemtemplate") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []ItemTemplate
	for rows.Next() {
		var t ItemTemplate
		var kind, quality, auctionLevel, attackRange int
		if err := rows.Scan(
			&t.ItemID, &t.Name, &kind, &t.Category, &quality, &auctionLevel, &t.InventoryType, &t.EquipmentSlot,
			&t.RequiredLevel, &t.RequiredProfession, &t.MaxDurability, &t.Weight,
			&t.BuyPrice, &t.SellPrice, &t.MaxStack, &t.IsStackable, &t.SowelSockets, &t.ValueOnUse,
			&t.AttackTime, &attackRange, &t.SowelBonusType, &t.SowelBonusValue,
		); err != nil {
			return nil, err
		}
		t.Kind = types.ItemKind(kind)
		t.Quality = types.ItemQuality(byteFromDB(quality))
		if auctionLevel > 0 || t.RequiredLevel == 0 {
			t.AuctionLevel = byteFromDB(auctionLevel)
		} else {
			t.AuctionLevel = types.AuctionLevelForRequiredLevel(t.RequiredLevel)
		}
		t.AttackRange = int16FromDB(attackRange)
		out = append(out, t)
	}
	return out, rows.Err()
}

func loadLegacyItemTemplates() ([]ItemTemplate, error) {
	attackTimeExpr := selectColumnOrDefault("asda2itemtemlate", "atack_time", "0")
	attackRangeExpr := selectColumnOrDefault("asda2itemtemlate", "atack_range", "0")
	sowelTypeExpr := selectColumnOrDefault("asda2itemtemlate", "sowel_bonus_type", "0")
	sowelBonusExpr := selectColumnOrDefault("asda2itemtemlate", "sowel_bonus_value", "0")
	qualityExpr := selectColumnOrDefault("asda2itemtemlate", "quality", "0")
	rows, err := DB.Query(
		fmt.Sprintf(`SELECT id, Name, category, inventory_type, equipment_slot,
		        reqaired_level, requaired_profession, max_durability, weight,
		        buy_cost, sell_cost, amount_in_stack, stackable, value_on_use,
		        %s, %s, %s, %s, %s
		   FROM asda2itemtemlate`, attackTimeExpr, attackRangeExpr, sowelTypeExpr, sowelBonusExpr, qualityExpr),
	)
	if err != nil {
		if missingItemTemplateTable(err, "asda2itemtemlate") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []ItemTemplate
	for rows.Next() {
		var t ItemTemplate
		var category, inventoryType, equipmentSlot int
		var requiredLevel, requiredProfession, maxDurability int
		var weight, buyPrice, sellPrice, maxStack, valueOnUse, attackTime, attackRange, sowelType, sowelBonus, quality int
		if err := rows.Scan(
			&t.ItemID, &t.Name, &category, &inventoryType, &equipmentSlot,
			&requiredLevel, &requiredProfession, &maxDurability, &weight,
			&buyPrice, &sellPrice, &maxStack, &t.IsStackable, &valueOnUse,
			&attackTime, &attackRange, &sowelType, &sowelBonus, &quality,
		); err != nil {
			return nil, err
		}
		t.Kind = legacyItemKind(category, equipmentSlot)
		t.Category = category
		t.InventoryType = byteFromDB(inventoryType)
		t.EquipmentSlot = int16FromDB(equipmentSlot)
		t.RequiredLevel = byteFromDB(requiredLevel)
		t.RequiredProfession = byteFromDB(requiredProfession)
		t.MaxDurability = byteFromDB(maxDurability)
		t.Weight = uint16FromDB(weight)
		t.BuyPrice = int64(buyPrice)
		t.SellPrice = int64(sellPrice)
		t.MaxStack = maxStack
		t.ValueOnUse = valueOnUse
		t.Quality = types.ItemQuality(byteFromDB(quality))
		t.AuctionLevel = types.AuctionLevelForRequiredLevel(t.RequiredLevel)
		t.AttackTime = attackTime
		t.AttackRange = int16FromDB(attackRange)
		t.SowelBonusType = sowelType
		t.SowelBonusValue = sowelBonus
		out = append(out, t)
	}
	return out, rows.Err()
}

func selectColumnOrDefault(table string, column string, defaultExpr string) string {
	if tableColumnExists(table, column) {
		return column
	}
	return defaultExpr
}

func tableColumnExists(table string, column string) bool {
	if DB == nil {
		return false
	}
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*)
		   FROM information_schema.COLUMNS
		  WHERE TABLE_SCHEMA = DATABASE()
		    AND LOWER(TABLE_NAME) = LOWER(?)
		    AND LOWER(COLUMN_NAME) = LOWER(?)`,
		table, column,
	).Scan(&count)
	return err == nil && count > 0
}

func missingItemTemplateTable(err error, table string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, strings.ToLower(table)) ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "unknown table")
}

func InitItemTemplateCache() (int, error) {
	templates, source, err := loadItemTemplatesWithSource()
	if err != nil {
		return 0, err
	}
	types.SetItemTemplates(templates)
	log.Printf("[Items] %d item templates loaded source=%s", len(templates), source)
	if len(templates) == 0 {
		log.Printf("[Items] no DB item templates found; using runtime fallback classification")
	}
	return len(templates), nil
}
