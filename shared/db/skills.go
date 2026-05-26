package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

type Asda2SkillTemplateRow struct {
	ID                      int32
	RealID                  int16
	Level                   byte
	LearnLevel              byte
	RequiredProfessionLevel byte
	ClassGroup              byte
	ClassMask               uint16
	Cost                    int64
	PowerCost               int16
	CooldownMillis          int32
	MaxRange                byte
	Damage                  int32
	EffectID                int32
	Effect0Misc             int16
	Effect0Type             int16
	Effect0BasePoints       int32
	Effect1Type             int16
	Effect1BasePoints       int32
	TargetFlags             uint32
	RequiredTargetType      byte
	SoulGuardLevel          byte
	CastTimeMillis          int32
	DurationMillis          int32
	IsPassive               bool
}

// InitSkillDB creates the small Asda2-only persistence needed by the current
// game skill runtime. It intentionally stores learned Asda2 skill real IDs
// rather than importing WCell's full SpellRecord/SpellCollection system.
func InitSkillDB() error {
	if err := ensureBonusSkillPointsColumn(); err != nil {
		return err
	}
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2SkillTemplate (
	RealId SMALLINT NOT NULL,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	LearnLevel TINYINT UNSIGNED NOT NULL DEFAULT 0,
	RequiredProfessionLevel TINYINT UNSIGNED NOT NULL DEFAULT 0,
	ClassGroup TINYINT UNSIGNED NOT NULL DEFAULT 0,
	ClassMask SMALLINT UNSIGNED NOT NULL DEFAULT 0,
	Cost BIGINT NOT NULL DEFAULT 0,
	PowerCost SMALLINT NOT NULL DEFAULT 0,
	CooldownMillis INT NOT NULL DEFAULT 0,
	MaxRange TINYINT UNSIGNED NOT NULL DEFAULT 0,
	Damage INT NOT NULL DEFAULT 0,
	EffectId INT NOT NULL DEFAULT 271,
	Effect0Misc SMALLINT NOT NULL DEFAULT 0,
	Effect0Type SMALLINT NOT NULL DEFAULT 0,
	Effect0BasePoints INT NOT NULL DEFAULT 0,
	Effect1Type SMALLINT NOT NULL DEFAULT 0,
	Effect1BasePoints INT NOT NULL DEFAULT 0,
	TargetFlags INT UNSIGNED NOT NULL DEFAULT 0,
	RequiredTargetType TINYINT UNSIGNED NOT NULL DEFAULT 0,
	SoulGuardLevel TINYINT UNSIGNED NOT NULL DEFAULT 0,
	CastTimeMillis INT NOT NULL DEFAULT 0,
	DurationMillis INT NOT NULL DEFAULT 0,
	IsPassive TINYINT(1) NOT NULL DEFAULT 0,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (RealId, Level),
	KEY IX_Asda2SkillTemplate_Class (ClassGroup, ClassMask, IsEnabled),
	KEY IX_Asda2SkillTemplate_SoulGuard (SoulGuardLevel, IsEnabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2SkillTemplate: %w", err)
	}
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2CharacterSkill (
	OwnerId INT UNSIGNED NOT NULL,
	SkillId SMALLINT NOT NULL,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (OwnerId, SkillId),
	KEY IX_Asda2CharacterSkill_Owner (OwnerId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2CharacterSkill: %w", err)
	}
	log.Printf("[SkillDB] Asda2SkillTemplate and Asda2CharacterSkill ready")
	return nil
}

func LoadAsda2SkillTemplates() ([]Asda2SkillTemplateRow, error) {
	rows, err := loadCanonicalSkillTemplates()
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	return loadLegacySkillTemplates()
}

func loadCanonicalSkillTemplates() ([]Asda2SkillTemplateRow, error) {
	rows, err := DB.Query(`
SELECT 0, RealId, Level, LearnLevel, RequiredProfessionLevel, ClassGroup, ClassMask,
       Cost, PowerCost, CooldownMillis, MaxRange, Damage, EffectId, Effect0Misc,
       Effect0Type, Effect0BasePoints, Effect1Type, Effect1BasePoints, TargetFlags,
       RequiredTargetType, SoulGuardLevel, CastTimeMillis, DurationMillis, IsPassive
  FROM Asda2SkillTemplate
 WHERE IsEnabled = 1 AND RealId > 0
 ORDER BY RealId ASC, Level ASC`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "asda2skilltemplate") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	return scanSkillTemplateRows(rows)
}

func loadLegacySkillTemplates() ([]Asda2SkillTemplateRow, error) {
	effect0TypeExpr := selectColumnOrDefault("SkillTemplate", "Effect0_EffectType", "0")
	effect0BaseExpr := selectColumnOrDefault("SkillTemplate", "Effect0_BasePoints", "0")
	effect1TypeExpr := selectColumnOrDefault("SkillTemplate", "Effect1_EffectType", "0")
	effect1BaseExpr := selectColumnOrDefault("SkillTemplate", "Effect1_BasePoints", "0")
	effectIDExpr := selectColumnOrDefault("SkillTemplate", "Effect0_MiscValue", "271")
	targetFlagsExpr := selectColumnOrDefault("SkillTemplate", "TargetFlags", "0")
	requiredTargetExpr := selectColumnOrDefault("SkillTemplate", "RequiredTargetType", "0")
	soulGuardExpr := selectColumnOrDefault("SkillTemplate", "SoulGuardProffLevel", "0")
	castTimeExpr := selectColumnOrDefault("SkillTemplate", "CastTime", "0")
	durationExpr := selectColumnOrDefault("SkillTemplate", "Duration", "0")
	isPassiveExpr := selectColumnOrDefault("SkillTemplate", "IsPassive", "0")
	query := fmt.Sprintf(`
SELECT Id, RealId, Level, LearnLevel, ProffNum, 0, ClassMask,
       Cost, PowerCost, CooldownTime, MaxRange, 0, %s, Effect0_MiscValue,
       %s, %s, %s, %s, %s, %s, %s, %s, %s, %s
  FROM SkillTemplate
 WHERE RealId > 0
 ORDER BY RealId ASC, Level ASC`,
		effectIDExpr,
		effect0TypeExpr, effect0BaseExpr, effect1TypeExpr, effect1BaseExpr,
		targetFlagsExpr, requiredTargetExpr, soulGuardExpr, castTimeExpr, durationExpr, isPassiveExpr)
	rows, err := DB.Query(query)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "skilltemplate") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	return scanSkillTemplateRows(rows)
}

func scanSkillTemplateRows(rows *sql.Rows) ([]Asda2SkillTemplateRow, error) {
	out := make([]Asda2SkillTemplateRow, 0, 2048)
	for rows.Next() {
		var row Asda2SkillTemplateRow
		var isPassiveRaw int
		if err := rows.Scan(
			&row.ID,
			&row.RealID,
			&row.Level,
			&row.LearnLevel,
			&row.RequiredProfessionLevel,
			&row.ClassGroup,
			&row.ClassMask,
			&row.Cost,
			&row.PowerCost,
			&row.CooldownMillis,
			&row.MaxRange,
			&row.Damage,
			&row.EffectID,
			&row.Effect0Misc,
			&row.Effect0Type,
			&row.Effect0BasePoints,
			&row.Effect1Type,
			&row.Effect1BasePoints,
			&row.TargetFlags,
			&row.RequiredTargetType,
			&row.SoulGuardLevel,
			&row.CastTimeMillis,
			&row.DurationMillis,
			&isPassiveRaw,
		); err != nil {
			return nil, err
		}
		if row.Level == 0 {
			continue
		}
		row.IsPassive = isPassiveRaw != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

func ensureBonusSkillPointsColumn() error {
	var columnName string
	err := DB.QueryRow(`
SELECT COLUMN_NAME
  FROM INFORMATION_SCHEMA.COLUMNS
 WHERE TABLE_SCHEMA = DATABASE()
   AND TABLE_NAME = 'CharacterRecord'
   AND COLUMN_NAME = 'BonusSkillPoints'
 LIMIT 1`).Scan(&columnName)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check CharacterRecord.BonusSkillPoints: %w", err)
	}
	if _, err := DB.Exec(`ALTER TABLE CharacterRecord ADD COLUMN BonusSkillPoints INT NOT NULL DEFAULT 0`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil
		}
		return fmt.Errorf("add CharacterRecord.BonusSkillPoints: %w", err)
	}
	log.Printf("[SkillDB] added CharacterRecord.BonusSkillPoints")
	return nil
}

// GetCharacterSkillsByOwner loads learned Asda2 skill real IDs for a character.
func GetCharacterSkillsByOwner(ownerID uint32) (map[int16]byte, error) {
	rows, err := DB.Query(`
SELECT SkillId, Level
  FROM Asda2CharacterSkill
 WHERE OwnerId = ?
 ORDER BY SkillId ASC`, ownerID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "asda2characterskill") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	out := make(map[int16]byte)
	for rows.Next() {
		var skillID int16
		var level byte
		if err := rows.Scan(&skillID, &level); err != nil {
			return nil, err
		}
		if level > 0 {
			out[skillID] = level
		}
	}
	return out, rows.Err()
}

// SaveCharacterSkill upserts one learned Asda2 skill real ID.
func SaveCharacterSkill(ownerID uint32, skillID int16, level byte) error {
	if level == 0 {
		level = 1
	}
	_, err := DB.Exec(`
INSERT INTO Asda2CharacterSkill (OwnerId, SkillId, Level)
VALUES (?, ?, ?)
	ON DUPLICATE KEY UPDATE Level = VALUES(Level)`, ownerID, skillID, level)
	return err
}

func DeleteCharacterSkills(ownerID uint32, skillIDs []int16) error {
	for _, skillID := range skillIDs {
		if _, err := DB.Exec(`DELETE FROM Asda2CharacterSkill WHERE OwnerId = ? AND SkillId = ?`, ownerID, skillID); err != nil {
			return err
		}
	}
	return nil
}
