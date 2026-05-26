package db

import (
	"fmt"
	"log"

	"asda2/shared/types"
)

type PortalRow = types.PortalRow

// InitPortalDB keeps the Asda2Portal reference table available without
// importing the broader WCell game-object schema.
func InitPortalDB() error {
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2Portal (
	Id INT UNSIGNED NOT NULL,
	FromX SMALLINT NOT NULL,
	FromY SMALLINT NOT NULL,
	FromMap SMALLINT UNSIGNED NOT NULL,
	ToX SMALLINT NOT NULL,
	ToY SMALLINT NOT NULL,
	ToMap SMALLINT UNSIGNED NOT NULL,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	PRIMARY KEY (Id),
	KEY IX_Asda2Portal_FromMap (FromMap, IsEnabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2Portal: %w", err)
	}
	log.Printf("[PortalDB] Asda2Portal ready")
	return nil
}

func LoadPortals() ([]PortalRow, error) {
	query := `
SELECT Id, FromX, FromY, FromMap, ToX, ToY, ToMap
  FROM Asda2Portal
 WHERE IsEnabled = 1
 ORDER BY FromMap ASC, Id ASC`
	if !tableColumnExists("Asda2Portal", "IsEnabled") {
		query = `
SELECT Id, FromX, FromY, FromMap, ToX, ToY, ToMap
  FROM Asda2Portal
 ORDER BY FromMap ASC, Id ASC`
	}
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PortalRow
	for rows.Next() {
		var row PortalRow
		if err := rows.Scan(&row.ID, &row.FromX, &row.FromY, &row.FromMap, &row.ToX, &row.ToY, &row.ToMap); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
