package db

import "asda2/shared/types"

// ---- Asda2Item table ----
// Mirrors WCell.RealmServer.Database.Asda2ItemRecord (table: "Asda2Item")

// GetItemsByOwner loads all items for a character, mirroring Asda2ItemRecord.LoadItems.
func GetItemsByOwner(ownerID uint32) ([]*ItemRow, error) {
	rows, err := DB.Query(
		`SELECT Guid, OwnerId, ItemId, InventoryType, Slot, CreatorId,
		        Durability, Duration, IsSoulBound, IsAuctioned, MailId,
		        Soul1Id, Soul2Id, Soul3Id, Soul4Id,
		        Enchant, EnchantResetCount,
		        Parametr1Type, Parametr1Value,
		        Parametr2Type, Parametr2Value,
		        Parametr3Type, Parametr3Value,
		        Parametr4Type, Parametr4Value,
		        Parametr5Type, Parametr5Value,
		        IsStackable, CreatorEntityId, Weight, SealCount,
		        Amount, AuctionPrice, AuctionEndTime, OwnerName, IsCrafted
		 FROM Asda2Item WHERE OwnerId = ?
		 ORDER BY InventoryType ASC, Slot ASC, Guid ASC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*ItemRow
	for rows.Next() {
		it := &ItemRow{}
		if err := rows.Scan(
			&it.Guid, &it.OwnerID, &it.ItemID, &it.InventoryType, &it.Slot, &it.CreatorID,
			&it.Durability, &it.Duration, &it.IsSoulBound, &it.IsAuctioned, &it.MailID,
			&it.Soul1ID, &it.Soul2ID, &it.Soul3ID, &it.Soul4ID,
			&it.Enchant, &it.EnchantResetCount,
			&it.Param1Type, &it.Param1Value,
			&it.Param2Type, &it.Param2Value,
			&it.Param3Type, &it.Param3Value,
			&it.Param4Type, &it.Param4Value,
			&it.Param5Type, &it.Param5Value,
			&it.IsStackable, &it.CreatorEntityID, &it.Weight, &it.SealCount,
			&it.Amount, &it.AuctionPrice, &it.AuctionEndTime, &it.OwnerName, &it.IsCrafted,
		); err != nil {
			return nil, err
		}
		types.ApplyItemTemplateToRow(it)
		out = append(out, it)
	}
	return out, rows.Err()
}

// SaveItem upserts a single item row.
func SaveItem(it *ItemRow) error {
	types.ApplyItemTemplateToRow(it)
	_, err := DB.Exec(
		`INSERT INTO Asda2Item
		    (Guid, OwnerId, ItemId, InventoryType, Slot, CreatorId,
		     Durability, Duration, IsSoulBound, IsAuctioned, MailId,
		    Soul1Id, Soul2Id, Soul3Id, Soul4Id, Enchant, EnchantResetCount,
		     Parametr1Type, Parametr1Value, Parametr2Type, Parametr2Value,
		     Parametr3Type, Parametr3Value, Parametr4Type, Parametr4Value,
		     Parametr5Type, Parametr5Value,
		     IsStackable, CreatorEntityId, Weight, SealCount,
		     Amount, AuctionPrice, AuctionEndTime, OwnerName, IsCrafted)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE
		    Slot=VALUES(Slot), InventoryType=VALUES(InventoryType),
		    Durability=VALUES(Durability), Duration=VALUES(Duration),
		    IsSoulBound=VALUES(IsSoulBound), IsAuctioned=VALUES(IsAuctioned),
		    MailId=VALUES(MailId),
		    Soul1Id=VALUES(Soul1Id), Soul2Id=VALUES(Soul2Id),
		    Soul3Id=VALUES(Soul3Id), Soul4Id=VALUES(Soul4Id),
		    Enchant=VALUES(Enchant), EnchantResetCount=VALUES(EnchantResetCount),
		    Parametr1Type=VALUES(Parametr1Type), Parametr1Value=VALUES(Parametr1Value),
		    Parametr2Type=VALUES(Parametr2Type), Parametr2Value=VALUES(Parametr2Value),
		    Parametr3Type=VALUES(Parametr3Type), Parametr3Value=VALUES(Parametr3Value),
		    Parametr4Type=VALUES(Parametr4Type), Parametr4Value=VALUES(Parametr4Value),
		    Parametr5Type=VALUES(Parametr5Type), Parametr5Value=VALUES(Parametr5Value),
		    IsStackable=VALUES(IsStackable), CreatorEntityId=VALUES(CreatorEntityId),
		    Weight=VALUES(Weight), SealCount=VALUES(SealCount),
		    Amount=VALUES(Amount), AuctionPrice=VALUES(AuctionPrice),
		    AuctionEndTime=VALUES(AuctionEndTime), OwnerName=VALUES(OwnerName),
		    IsCrafted=VALUES(IsCrafted)`,
		it.Guid, it.OwnerID, it.ItemID, it.InventoryType, it.Slot, it.CreatorID,
		it.Durability, it.Duration, it.IsSoulBound, it.IsAuctioned, it.MailID,
		it.Soul1ID, it.Soul2ID, it.Soul3ID, it.Soul4ID, it.Enchant, it.EnchantResetCount,
		it.Param1Type, it.Param1Value, it.Param2Type, it.Param2Value,
		it.Param3Type, it.Param3Value, it.Param4Type, it.Param4Value,
		it.Param5Type, it.Param5Value,
		it.IsStackable, it.CreatorEntityID, it.Weight, it.SealCount,
		it.Amount, it.AuctionPrice, it.AuctionEndTime, it.OwnerName, it.IsCrafted,
	)
	return err
}

// DeleteItem removes a single item by GUID.
func DeleteItem(guid int64) error {
	_, err := DB.Exec(`DELETE FROM Asda2Item WHERE Guid = ?`, guid)
	return err
}

func NextItemGUID() (int64, error) {
	var next int64
	err := DB.QueryRow(`SELECT COALESCE(MAX(Guid), 0) + 1 FROM Asda2Item`).Scan(&next)
	return next, err
}
