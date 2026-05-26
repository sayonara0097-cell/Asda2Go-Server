package db

import (
	"fmt"
	"log"

	"asda2/shared/types"
)

type NpcVendorItemRow = types.NpcVendorItemRow

// InitNpcVendorDB creates the Asda2 regular-shop table used by NPC vendors.
// It mirrors WCell.RealmServer.Asda2 Items/RegularShopRecord.cs.
func InitNpcVendorDB() error {
	if DB == nil {
		return nil
	}
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2NpcVendorItem (
	VendorEntryId SMALLINT UNSIGNED NOT NULL,
	ItemId INT NOT NULL,
	SortOrder SMALLINT UNSIGNED NOT NULL DEFAULT 0,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (VendorEntryId, ItemId),
	KEY IX_Asda2NpcVendorItem_Item (ItemId, IsEnabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2NpcVendorItem: %w", err)
	}
	log.Printf("[NpcVendorDB] Asda2NpcVendorItem ready")
	return nil
}

func LoadNpcVendorItems() ([]NpcVendorItemRow, error) {
	if DB == nil {
		return nil, nil
	}
	rows, err := DB.Query(`
SELECT VendorEntryId, ItemId, SortOrder, IsEnabled
  FROM Asda2NpcVendorItem
 WHERE IsEnabled = 1
 ORDER BY VendorEntryId ASC, SortOrder ASC, ItemId ASC`)
	if err != nil {
		if missingItemTemplateTable(err, "Asda2NpcVendorItem") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []NpcVendorItemRow
	for rows.Next() {
		var row NpcVendorItemRow
		var enabled int
		if err := rows.Scan(&row.VendorEntryID, &row.ItemID, &row.SortOrder, &enabled); err != nil {
			return nil, err
		}
		row.IsEnabled = enabled != 0
		if row.ItemID <= 0 || !row.IsEnabled {
			continue
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
