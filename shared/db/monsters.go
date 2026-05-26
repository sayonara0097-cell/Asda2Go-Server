package db

import (
	"fmt"
	"log"

	"asda2/shared/types"
)

type MonsterTemplateRow = types.MonsterTemplateRow
type MonsterSpawnRow = types.MonsterSpawnRow

// InitMonsterDB creates the small Asda2 monster tables kept for fallback and
// admin tooling. The game runtime prefers static worlddata files.
func InitMonsterDB() error {
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2MonsterTemplate (
	EntryId SMALLINT UNSIGNED NOT NULL,
	Name VARCHAR(80) NOT NULL,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	MaxHealth INT NOT NULL DEFAULT 100,
	MoveMs SMALLINT NOT NULL DEFAULT 150,
	WalkSpeed DOUBLE NOT NULL DEFAULT 1,
	RunSpeed DOUBLE NOT NULL DEFAULT 3.5,
	MinDamage DOUBLE NOT NULL DEFAULT 5,
	MaxDamage DOUBLE NOT NULL DEFAULT 5,
	BaseAttackMs INT NOT NULL DEFAULT 2000,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	PRIMARY KEY (EntryId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2MonsterTemplate: %w", err)
	}
	if err := ensureMonsterTemplateCombatColumns(); err != nil {
		return err
	}

	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2MonsterSpawn (
	SpawnId INT UNSIGNED NOT NULL,
	EntryId SMALLINT UNSIGNED NOT NULL,
	MapId SMALLINT UNSIGNED NOT NULL,
	LocalX SMALLINT NOT NULL,
	LocalY SMALLINT NOT NULL,
	RespawnSeconds INT NOT NULL DEFAULT 30,
	Channel SMALLINT NOT NULL DEFAULT -1,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	PRIMARY KEY (SpawnId),
	KEY IX_Asda2MonsterSpawn_Map (MapId, IsEnabled),
	KEY IX_Asda2MonsterSpawn_Channel (Channel, IsEnabled),
	CONSTRAINT FK_Asda2MonsterSpawn_Template
		FOREIGN KEY (EntryId) REFERENCES Asda2MonsterTemplate (EntryId)
		ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2MonsterSpawn: %w", err)
	}

	log.Printf("[MonsterDB] Asda2MonsterTemplate and Asda2MonsterSpawn ready")
	return nil
}

func LoadMonsterTemplates() ([]MonsterTemplateRow, error) {
	walkSpeedExpr := selectColumnOrDefault("Asda2MonsterTemplate", "WalkSpeed", "0")
	runSpeedExpr := selectColumnOrDefault("Asda2MonsterTemplate", "RunSpeed", "0")
	minDamageExpr := selectColumnOrDefault("Asda2MonsterTemplate", "MinDamage", "0")
	maxDamageExpr := selectColumnOrDefault("Asda2MonsterTemplate", "MaxDamage", "0")
	baseAttackExpr := selectColumnOrDefault("Asda2MonsterTemplate", "BaseAttackMs", "0")
	rows, err := DB.Query(`
SELECT EntryId, Name, Level, MaxHealth, MoveMs, ` + walkSpeedExpr + `, ` + runSpeedExpr + `,
       ` + minDamageExpr + `, ` + maxDamageExpr + `, ` + baseAttackExpr + `
  FROM Asda2MonsterTemplate
 WHERE IsEnabled = 1
 ORDER BY EntryId ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MonsterTemplateRow
	for rows.Next() {
		var level int
		row := MonsterTemplateRow{}
		if err := rows.Scan(
			&row.EntryID, &row.Name, &level, &row.MaxHealth, &row.MoveMS,
			&row.WalkSpeed, &row.RunSpeed, &row.MinDamage, &row.MaxDamage, &row.BaseAttackMS,
		); err != nil {
			return nil, err
		}
		row.Level = byte(level)
		out = append(out, row)
	}
	return out, rows.Err()
}

func ensureMonsterTemplateCombatColumns() error {
	columns := []struct {
		name string
		def  string
	}{
		{name: "WalkSpeed", def: "DOUBLE NOT NULL DEFAULT 1"},
		{name: "RunSpeed", def: "DOUBLE NOT NULL DEFAULT 3.5"},
		{name: "MinDamage", def: "DOUBLE NOT NULL DEFAULT 5"},
		{name: "MaxDamage", def: "DOUBLE NOT NULL DEFAULT 5"},
		{name: "BaseAttackMs", def: "INT NOT NULL DEFAULT 2000"},
	}
	for _, column := range columns {
		if tableColumnExists("Asda2MonsterTemplate", column.name) {
			continue
		}
		if _, err := DB.Exec(fmt.Sprintf("ALTER TABLE Asda2MonsterTemplate ADD COLUMN %s %s", column.name, column.def)); err != nil {
			return fmt.Errorf("add Asda2MonsterTemplate.%s: %w", column.name, err)
		}
	}
	return nil
}

func LoadMonsterSpawns(channel byte) ([]MonsterSpawnRow, error) {
	rows, err := DB.Query(`
SELECT SpawnId, EntryId, MapId, LocalX, LocalY, RespawnSeconds, Channel, IsEnabled
  FROM Asda2MonsterSpawn
 WHERE IsEnabled = 1
   AND (Channel = -1 OR Channel = ?)
 ORDER BY MapId ASC, SpawnId ASC`, int(channel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MonsterSpawnRow
	for rows.Next() {
		var enabled int
		row := MonsterSpawnRow{}
		if err := rows.Scan(
			&row.SpawnID, &row.EntryID, &row.MapID, &row.LocalX, &row.LocalY,
			&row.RespawnSeconds, &row.Channel, &enabled,
		); err != nil {
			return nil, err
		}
		row.IsEnabled = enabled != 0
		out = append(out, row)
	}
	return out, rows.Err()
}
