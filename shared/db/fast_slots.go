package db

// ---- Asda2FastItemSlot table ----
// Mirrors WCell.RealmServer.Database.Asda2FastItemSlotRecord (table: "Asda2FastItemSlot")

// GetFastSlotsByOwner loads all quick-bar bindings for a character.
func GetFastSlotsByOwner(ownerID uint32) ([]*FastSlotRow, error) {
	rows, err := DB.Query(
		`SELECT Guid, OwnerId, PanelNum, PanelSlot, InventoryType,
		        ItemOrSkillId, InventorySlot, SrcInfo, Amount
		 FROM Asda2FastItemSlot
		 WHERE OwnerId = ?
		 ORDER BY PanelNum ASC, PanelSlot ASC, Guid ASC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FastSlotRow
	for rows.Next() {
		s := &FastSlotRow{}
		if err := rows.Scan(
			&s.Guid, &s.OwnerID, &s.PanelNum, &s.PanelSlot, &s.InventoryType,
			&s.ItemOrSkillID, &s.InventorySlot, &s.SrcInfo, &s.Amount,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ReplaceFastSlotsForOwnerPanel mirrors SetFastItemSlotRequest: the client sends
// one whole panel, so WCell deletes the previous panel records and creates the
// non-empty slots again.
func ReplaceFastSlotsForOwnerPanel(ownerID uint32, panel byte, slots []*FastSlotRow) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM Asda2FastItemSlot WHERE OwnerId = ? AND PanelNum = ?`,
		ownerID, panel,
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	var nextGuid int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(Guid), 0) + 1 FROM Asda2FastItemSlot`).Scan(&nextGuid); err != nil {
		_ = tx.Rollback()
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO Asda2FastItemSlot
		    (Guid, OwnerId, PanelNum, PanelSlot, InventoryType,
		     ItemOrSkillId, InventorySlot, SrcInfo, Amount)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, s := range slots {
		if s == nil {
			continue
		}
		s.Guid = nextGuid
		nextGuid++
		s.OwnerID = ownerID
		s.PanelNum = panel
		if _, err := stmt.Exec(
			s.Guid, s.OwnerID, s.PanelNum, s.PanelSlot, s.InventoryType,
			s.ItemOrSkillID, s.InventorySlot, s.SrcInfo, s.Amount,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// SaveFastSlot upserts a quick-bar row.
func SaveFastSlot(s *FastSlotRow) error {
	_, err := DB.Exec(
		`INSERT INTO Asda2FastItemSlot
		    (Guid, OwnerId, PanelNum, PanelSlot, InventoryType,
		     ItemOrSkillId, InventorySlot, SrcInfo, Amount)
		 VALUES (?,?,?,?,?,?,?,?,?)
		 ON DUPLICATE KEY UPDATE
		    PanelNum=VALUES(PanelNum), PanelSlot=VALUES(PanelSlot),
		    InventoryType=VALUES(InventoryType),
		    ItemOrSkillId=VALUES(ItemOrSkillId),
		    InventorySlot=VALUES(InventorySlot),
		    SrcInfo=VALUES(SrcInfo), Amount=VALUES(Amount)`,
		s.Guid, s.OwnerID, s.PanelNum, s.PanelSlot, s.InventoryType,
		s.ItemOrSkillID, s.InventorySlot, s.SrcInfo, s.Amount,
	)
	return err
}
