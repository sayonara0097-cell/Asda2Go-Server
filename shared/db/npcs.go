package db

import (
	"fmt"
	"log"
	"strings"

	"asda2/shared/types"
)

type NpcTemplateRow = types.NpcTemplateRow
type NpcSpawnRow = types.NpcSpawnRow

// InitNpcDB creates the small Asda2 NPC tables kept for fallback and admin
// tooling. The game runtime prefers static worlddata files.
func InitNpcDB() error {
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2NpcTemplate (
	EntryId SMALLINT UNSIGNED NOT NULL,
	Name VARCHAR(80) NOT NULL,
	Kind TINYINT UNSIGNED NOT NULL DEFAULT 0,
	ClassGroup TINYINT UNSIGNED NOT NULL DEFAULT 0,
	IsTrainer TINYINT(1) NOT NULL DEFAULT 0,
	InteractionKind TINYINT UNSIGNED NOT NULL DEFAULT 0,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	PRIMARY KEY (EntryId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2NpcTemplate: %w", err)
	}
	if err := ensureNpcTemplateInteractionColumn(); err != nil {
		return err
	}

	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2NpcSpawn (
	SpawnId INT UNSIGNED NOT NULL,
	EntryId SMALLINT UNSIGNED NOT NULL,
	MapId SMALLINT UNSIGNED NOT NULL,
	LocalX SMALLINT NOT NULL,
	LocalY SMALLINT NOT NULL,
	Channel SMALLINT NOT NULL DEFAULT -1,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	PRIMARY KEY (SpawnId),
	KEY IX_Asda2NpcSpawn_Map (MapId, IsEnabled),
	KEY IX_Asda2NpcSpawn_Channel (Channel, IsEnabled),
	CONSTRAINT FK_Asda2NpcSpawn_Template
		FOREIGN KEY (EntryId) REFERENCES Asda2NpcTemplate (EntryId)
		ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2NpcSpawn: %w", err)
	}

	log.Printf("[NpcDB] Asda2NpcTemplate and Asda2NpcSpawn ready")
	return nil
}

func LoadNpcTemplates() ([]NpcTemplateRow, error) {
	hasInteractionKind := tableColumnExists("Asda2NpcTemplate", "InteractionKind")
	query := `
SELECT EntryId, Name, Kind, ClassGroup, IsTrainer
  FROM Asda2NpcTemplate
 WHERE IsEnabled = 1
 ORDER BY EntryId ASC`
	if hasInteractionKind {
		query = `
SELECT EntryId, Name, Kind, ClassGroup, IsTrainer, InteractionKind
  FROM Asda2NpcTemplate
 WHERE IsEnabled = 1
 ORDER BY EntryId ASC`
	}
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NpcTemplateRow
	for rows.Next() {
		var kind, classGroup, isTrainer, interactionKind int
		row := NpcTemplateRow{}
		if hasInteractionKind {
			if err := rows.Scan(&row.EntryID, &row.Name, &kind, &classGroup, &isTrainer, &interactionKind); err != nil {
				return nil, err
			}
		} else {
			if err := rows.Scan(&row.EntryID, &row.Name, &kind, &classGroup, &isTrainer); err != nil {
				return nil, err
			}
		}
		row.Kind = byte(kind)
		row.ClassGroup = byte(classGroup)
		row.IsTrainer = isTrainer != 0
		row.InteractionKind = types.NpcInteractionKind(byte(interactionKind))
		out = append(out, types.NormalizeNpcTemplate(row))
	}
	return out, rows.Err()
}

func ensureNpcTemplateInteractionColumn() error {
	if tableColumnExists("Asda2NpcTemplate", "InteractionKind") {
		return nil
	}
	if _, err := DB.Exec(`ALTER TABLE Asda2NpcTemplate ADD COLUMN InteractionKind TINYINT UNSIGNED NOT NULL DEFAULT 0 AFTER IsTrainer`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil
		}
		return fmt.Errorf("add Asda2NpcTemplate.InteractionKind: %w", err)
	}
	log.Printf("[NpcDB] added Asda2NpcTemplate.InteractionKind")
	return nil
}

func LoadNpcSpawns(channel byte) ([]NpcSpawnRow, error) {
	rows, err := DB.Query(`
SELECT SpawnId, EntryId, MapId, LocalX, LocalY, Channel, IsEnabled
  FROM Asda2NpcSpawn
 WHERE IsEnabled = 1
   AND (Channel = -1 OR Channel = ?)
 ORDER BY MapId ASC, SpawnId ASC`, int(channel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NpcSpawnRow
	for rows.Next() {
		var enabled int
		row := NpcSpawnRow{}
		if err := rows.Scan(
			&row.SpawnID, &row.EntryID, &row.MapID, &row.LocalX, &row.LocalY,
			&row.Channel, &enabled,
		); err != nil {
			return nil, err
		}
		row.IsEnabled = enabled != 0
		out = append(out, row)
	}
	return out, rows.Err()
}
