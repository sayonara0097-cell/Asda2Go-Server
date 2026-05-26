package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"asda2/shared/types"
)

type BaseStatsRow = types.BaseStatsRow

// InitBaseStatsDB creates the canonical Asda2 base-stat table and seeds it
// from the editable BaseStats reference files when the table is empty.
func InitBaseStatsDB() error {
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2BaseStat (
	ClassID TINYINT UNSIGNED NOT NULL,
	Level TINYINT UNSIGNED NOT NULL,
	BaseHealth INT NOT NULL DEFAULT 0,
	BasePower INT NOT NULL DEFAULT 0,
	Attr1 INT NOT NULL DEFAULT 0,
	Attr2 INT NOT NULL DEFAULT 0,
	Attr3 INT NOT NULL DEFAULT 0,
	Attr4 INT NOT NULL DEFAULT 0,
	Attr5 INT NOT NULL DEFAULT 0,
	Attr6 INT NOT NULL DEFAULT 0,
	Source VARCHAR(80) NOT NULL DEFAULT 'BaseStats',
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (ClassID, Level)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2BaseStat: %w", err)
	}

	var rowCount int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM Asda2BaseStat`).Scan(&rowCount); err != nil {
		return fmt.Errorf("count Asda2BaseStat: %w", err)
	}

	if rowCount == 0 || envBool("ASDA2_BASE_STATS_RESEED") {
		dir, err := findBaseStatsDir()
		if err != nil {
			log.Printf("[BaseStatsDB] %v; table is ready but not seeded", err)
			return nil
		}
		seeded, err := seedBaseStatsFromDir(dir, rowCount > 0)
		if err != nil {
			return err
		}
		rowCount = seeded
	}

	log.Printf("[BaseStatsDB] Asda2BaseStat ready (%d rows)", rowCount)
	return nil
}

func GetBaseStats(classID, level byte) (*BaseStatsRow, error) {
	row := DB.QueryRow(`
SELECT ClassID, Level, BaseHealth, BasePower, Attr1, Attr2, Attr3, Attr4, Attr5, Attr6
  FROM Asda2BaseStat
 WHERE ClassID = ? AND Level = ?
 LIMIT 1`, classID, level)

	stats := &BaseStatsRow{}
	err := row.Scan(
		&stats.ClassID, &stats.Level, &stats.BaseHealth, &stats.BasePower,
		&stats.Attr1, &stats.Attr2, &stats.Attr3, &stats.Attr4, &stats.Attr5, &stats.Attr6,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return stats, err
}

func GetBaseStatsForAsdaClass(asda2Class, level byte) (*BaseStatsRow, error) {
	if level < 1 {
		level = 1
	}
	return GetBaseStats(types.BaseStatsClassID(asda2Class), level)
}

func ApplyBaseStatsToCharacterRow(r *CharacterRow, fillCurrent bool) (bool, error) {
	if r == nil {
		return false, nil
	}
	level := byte(r.Level)
	if level < 1 {
		level = 1
	}
	stats, err := GetBaseStatsForAsdaClass(r.Asda2Class, level)
	if err != nil || stats == nil {
		return false, err
	}

	changed := r.BaseHealth != stats.BaseHealth ||
		r.BasePower != stats.BasePower ||
		r.BaseStrength != stats.Attr1 ||
		r.BaseAgility != stats.Attr2 ||
		r.BaseStamina != stats.Attr3 ||
		r.BaseSpirit != stats.Attr4 ||
		r.BaseIntellect != stats.Attr5 ||
		r.BaseLuck != stats.Attr6

	r.BaseHealth = stats.BaseHealth
	r.BasePower = stats.BasePower
	r.BaseStrength = stats.Attr1
	r.BaseAgility = stats.Attr2
	r.BaseStamina = stats.Attr3
	r.BaseSpirit = stats.Attr4
	r.BaseIntellect = stats.Attr5
	r.BaseLuck = stats.Attr6

	if fillCurrent || changed || r.Health <= 0 || r.Health > r.BaseHealth {
		r.Health = r.BaseHealth
	}
	if fillCurrent || changed || r.Power <= 0 || r.Power > r.BasePower {
		r.Power = r.BasePower
	}
	return changed, nil
}

func ApplyBaseStatsToCharacter(c *Character, fillCurrent bool) (bool, error) {
	if c == nil {
		return false, nil
	}
	stats, err := GetBaseStatsForAsdaClass(c.Class, c.Level)
	if err != nil || stats == nil {
		return false, err
	}

	changed := c.MaxHP != int32(stats.BaseHealth) ||
		c.MaxMP != int32(stats.BasePower) ||
		c.BaseStrength != int16(stats.Attr1) ||
		c.BaseAgility != int16(stats.Attr2) ||
		c.BaseStamina != int16(stats.Attr3) ||
		c.BaseSpirit != int16(stats.Attr4) ||
		c.BaseIntellect != int16(stats.Attr5) ||
		c.BaseLuck != int16(stats.Attr6)

	c.MaxHP = int32(stats.BaseHealth)
	c.MaxMP = int32(stats.BasePower)
	c.BaseStrength = int16(stats.Attr1)
	c.BaseAgility = int16(stats.Attr2)
	c.BaseStamina = int16(stats.Attr3)
	c.BaseSpirit = int16(stats.Attr4)
	c.BaseIntellect = int16(stats.Attr5)
	c.BaseLuck = int16(stats.Attr6)

	if fillCurrent || changed || c.HP <= 0 || c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
	if fillCurrent || changed || c.MP <= 0 || c.MP > c.MaxMP {
		c.MP = c.MaxMP
	}
	return changed, nil
}

func seedBaseStatsFromDir(dir string, clear bool) (int, error) {
	hpmp, err := parseStandardBaseStats(filepath.Join(dir, "BASE_STATS_REFERENCE.txt"))
	if err != nil {
		return 0, err
	}
	rows, err := parseClassAttributes(filepath.Join(dir, "seed_class_attributes.sql"), hpmp)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("seed Asda2BaseStat: no rows parsed from %s", dir)
	}

	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	if clear {
		if _, err := tx.Exec(`DELETE FROM Asda2BaseStat`); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("clear Asda2BaseStat: %w", err)
		}
	}

	stmt, err := tx.Prepare(`
INSERT INTO Asda2BaseStat
	(ClassID, Level, BaseHealth, BasePower, Attr1, Attr2, Attr3, Attr4, Attr5, Attr6, Source)
VALUES
	(?,?,?,?,?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE
	BaseHealth = VALUES(BaseHealth),
	BasePower = VALUES(BasePower),
	Attr1 = VALUES(Attr1),
	Attr2 = VALUES(Attr2),
	Attr3 = VALUES(Attr3),
	Attr4 = VALUES(Attr4),
	Attr5 = VALUES(Attr5),
	Attr6 = VALUES(Attr6),
	Source = VALUES(Source)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err := stmt.Exec(
			row.ClassID, row.Level, row.BaseHealth, row.BasePower,
			row.Attr1, row.Attr2, row.Attr3, row.Attr4, row.Attr5, row.Attr6,
			"BaseStats",
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("seed Asda2BaseStat class=%d level=%d: %w", row.ClassID, row.Level, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	log.Printf("[BaseStatsDB] seeded %d rows from %s", len(rows), dir)
	return len(rows), nil
}

type hpmpRow struct {
	hp int
	mp int
}

func parseStandardBaseStats(path string) (map[int]hpmpRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Standard base stats %s: %w", path, err)
	}
	defer f.Close()

	out := make(map[int]hpmpRow)
	inTable := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "TABLE 1:") {
			inTable = true
			continue
		}
		if strings.Contains(line, "TABLE 2:") {
			break
		}
		if !inTable {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		level, err1 := strconv.Atoi(fields[0])
		hp, err2 := strconv.Atoi(fields[1])
		mp, err3 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		out[level] = hpmpRow{hp: hp, mp: mp}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("parse Standard base stats: no HP/MP rows found in %s", path)
	}
	return out, nil
}

func parseClassAttributes(path string, hpmp map[int]hpmpRow) ([]BaseStatsRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open ClassStat base stats %s: %w", path, err)
	}
	re := regexp.MustCompile(`\((\d+),\s*(\d+),\s*(-?\d+),\s*(-?\d+),\s*(-?\d+),\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\)`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	rows := make([]BaseStatsRow, 0, len(matches))
	for _, m := range matches {
		nums := make([]int, 8)
		for i := 0; i < 8; i++ {
			v, err := strconv.Atoi(m[i+1])
			if err != nil {
				return nil, err
			}
			nums[i] = v
		}
		hm, ok := hpmp[nums[1]]
		if !ok {
			return nil, fmt.Errorf("ClassStat level %d has no Standard HP/MP row", nums[1])
		}
		rows = append(rows, BaseStatsRow{
			ClassID:    byte(nums[0]),
			Level:      byte(nums[1]),
			BaseHealth: hm.hp,
			BasePower:  hm.mp,
			Attr1:      nums[2],
			Attr2:      nums[3],
			Attr3:      nums[4],
			Attr4:      nums[5],
			Attr5:      nums[6],
			Attr6:      nums[7],
		})
	}
	return rows, nil
}

func findBaseStatsDir() (string, error) {
	var candidates []string
	if dir := strings.TrimSpace(os.Getenv("ASDA2_BASE_STATS_DIR")); dir != "" {
		candidates = append(candidates, dir)
	}
	candidates = append(candidates, "BaseStats", filepath.Join("..", "BaseStats"), filepath.Join("..", "..", "BaseStats"))
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "BaseStats"),
			filepath.Join(exeDir, "..", "BaseStats"),
			filepath.Join(exeDir, "..", "..", "BaseStats"),
		)
	}

	for _, candidate := range candidates {
		dir, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if fileExists(filepath.Join(dir, "BASE_STATS_REFERENCE.txt")) && fileExists(filepath.Join(dir, "seed_class_attributes.sql")) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("BaseStats source files not found; set ASDA2_BASE_STATS_DIR")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
