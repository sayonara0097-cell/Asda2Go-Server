package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"asda2/shared/types"
)

type TeleportPointRow = types.TeleportPointRow

func InitTeleportDB() error {
	if err := ensureBindLocationColumns(); err != nil {
		return err
	}
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2TeleportingPointRecord (
	Guid BIGINT NOT NULL,
	OwnerId INT UNSIGNED NOT NULL,
	Slot TINYINT UNSIGNED NOT NULL,
	Name VARCHAR(32) NOT NULL DEFAULT '',
	MapId SMALLINT UNSIGNED NOT NULL,
	X SMALLINT NOT NULL,
	Y SMALLINT NOT NULL,
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (Guid),
	UNIQUE KEY UX_Asda2TeleportingPointRecord_OwnerSlot (OwnerId, Slot),
	KEY IX_Asda2TeleportingPointRecord_Owner (OwnerId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2TeleportingPointRecord: %w", err)
	}
	if err := ensureTeleportPointSlotColumn(); err != nil {
		return err
	}
	log.Printf("[TeleportDB] Asda2TeleportingPointRecord ready")
	return nil
}

func ensureBindLocationColumns() error {
	columns := []struct {
		name string
		def  string
	}{
		{name: "BindX", def: "FLOAT NOT NULL DEFAULT 3130.64"},
		{name: "BindY", def: "FLOAT NOT NULL DEFAULT 3398.69"},
		{name: "BindZ", def: "FLOAT NOT NULL DEFAULT 0"},
		{name: "BindMap", def: "INT NOT NULL DEFAULT 3"},
		{name: "BindZone", def: "INT NOT NULL DEFAULT 0"},
	}
	for _, col := range columns {
		if err := ensureCharacterRecordColumn(col.name, col.def); err != nil {
			return err
		}
	}
	return nil
}

func ensureCharacterRecordColumn(name string, definition string) error {
	var columnName string
	err := DB.QueryRow(`
SELECT COLUMN_NAME
  FROM INFORMATION_SCHEMA.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE()
   AND TABLE_NAME = 'CharacterRecord'
   AND COLUMN_NAME = ?
 LIMIT 1`, name).Scan(&columnName)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check CharacterRecord.%s: %w", name, err)
	}
	if _, err := DB.Exec(`ALTER TABLE CharacterRecord ADD COLUMN ` + name + ` ` + definition); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil
		}
		return fmt.Errorf("add CharacterRecord.%s: %w", name, err)
	}
	log.Printf("[TeleportDB] added CharacterRecord.%s", name)
	return nil
}

func GetTeleportPointsByOwner(ownerID uint32) ([10]*TeleportPointRow, error) {
	var out [10]*TeleportPointRow
	if !tableColumnExists("Asda2TeleportingPointRecord", "Slot") {
		return getLegacyTeleportPointsByOwner(ownerID)
	}
	rows, err := DB.Query(`
SELECT Guid, OwnerId, Slot, Name, MapId, X, Y
  FROM Asda2TeleportingPointRecord
 WHERE OwnerId = ?
 ORDER BY Slot ASC`, ownerID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "asda2teleportingpointrecord") {
			return out, nil
		}
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		rec := &TeleportPointRow{}
		if err := rows.Scan(&rec.Guid, &rec.OwnerID, &rec.Slot, &rec.Name, &rec.MapID, &rec.X, &rec.Y); err != nil {
			return out, err
		}
		if rec.Slot < 10 {
			out[rec.Slot] = rec
		}
	}
	return out, rows.Err()
}

func getLegacyTeleportPointsByOwner(ownerID uint32) ([10]*TeleportPointRow, error) {
	var out [10]*TeleportPointRow
	rows, err := DB.Query(`
SELECT Guid, OwnerId, Name, MapId, X, Y
  FROM Asda2TeleportingPointRecord
 WHERE OwnerId = ?
 ORDER BY Guid ASC`, ownerID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "asda2teleportingpointrecord") {
			return out, nil
		}
		return out, err
	}
	defer rows.Close()

	slot := byte(0)
	for rows.Next() {
		if slot >= 10 {
			break
		}
		rec := &TeleportPointRow{Slot: slot}
		if err := rows.Scan(&rec.Guid, &rec.OwnerID, &rec.Name, &rec.MapID, &rec.X, &rec.Y); err != nil {
			return out, err
		}
		out[slot] = rec
		slot++
	}
	return out, rows.Err()
}

func SaveTeleportPoint(rec *TeleportPointRow) error {
	if rec == nil {
		return nil
	}
	if rec.Guid == 0 {
		guid, err := NextTeleportPointGUID()
		if err != nil {
			return err
		}
		rec.Guid = guid
	}
	_, err := DB.Exec(`
INSERT INTO Asda2TeleportingPointRecord (Guid, OwnerId, Slot, Name, MapId, X, Y)
VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		Name = VALUES(Name),
		MapId = VALUES(MapId),
		X = VALUES(X),
		Y = VALUES(Y)`,
		rec.Guid, rec.OwnerID, rec.Slot, rec.Name, rec.MapID, rec.X, rec.Y)
	return err
}

func DeleteTeleportPoint(ownerID uint32, slot byte) error {
	_, err := DB.Exec(`DELETE FROM Asda2TeleportingPointRecord WHERE OwnerId = ? AND Slot = ?`, ownerID, slot)
	return err
}

func NextTeleportPointGUID() (int64, error) {
	var next int64
	err := DB.QueryRow(`SELECT COALESCE(MAX(Guid), 0) + 1 FROM Asda2TeleportingPointRecord`).Scan(&next)
	return next, err
}

func ensureTeleportPointSlotColumn() error {
	if tableColumnExists("Asda2TeleportingPointRecord", "Slot") {
		return nil
	}
	if _, err := DB.Exec(`ALTER TABLE Asda2TeleportingPointRecord ADD COLUMN Slot TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER OwnerId`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil
		}
		return fmt.Errorf("add Asda2TeleportingPointRecord.Slot: %w", err)
	}
	log.Printf("[TeleportDB] added Asda2TeleportingPointRecord.Slot")
	return nil
}

func SaveBindLocation(c *Character) error {
	if c == nil {
		return nil
	}
	_, err := DB.Exec(`
UPDATE CharacterRecord
   SET BindX = ?, BindY = ?, BindZ = 0, BindMap = ?, BindZone = 0
 WHERE EntityLowId = ?`, c.X, c.Y, c.MapID, c.GUID)
	return err
}
