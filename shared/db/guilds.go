package db

import (
	"asda2/shared/types"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	GuildCrestLength      = 40
	GuildDefaultLevel     = 1
	GuildDefaultMaxMember = 30
)

type GuildData struct {
	ID                uint32
	Name              string
	Level             byte
	MaxMembers        byte
	Points            int32
	WaveLimit         byte
	Crest             []byte
	MOTD              string
	NoticeWriter      string
	NoticeTime        time.Time
	LeaderCharacterID uint32
	Ranks             []GuildRankData
	Members           []GuildMemberData
	Skills            []GuildSkillData
	History           []GuildHistoryData
}

type GuildRankData struct {
	GuildID    uint32
	RankIndex  byte
	Name       string
	Privileges uint16
}

type GuildMemberData struct {
	CharacterID     uint32
	GuildID         uint32
	AccountID       uint32
	CharNum         byte
	Name            string
	Level           byte
	ProfessionLevel byte
	Class           byte
	RankIndex       byte
	PublicNote      string
	LastLogin       time.Time
	LastMapID       uint16
	GuildPoints     int32
}

type GuildSkillData struct {
	GuildID         uint32
	SkillID         int16
	Level           byte
	IsActivated     bool
	LastMaintenance time.Time
}

type GuildHistoryData struct {
	ID          int64
	GuildID     uint32
	Type        byte
	Value       int32
	TriggerName string
	EventTime   string
	CreatedAt   time.Time
}

func InitGuildDB() error {
	if err := ensureGuildCharacterColumns(); err != nil {
		return err
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS Asda2Guild (
	ID INT UNSIGNED NOT NULL AUTO_INCREMENT,
	Name VARCHAR(17) NOT NULL,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	MaxMembers TINYINT UNSIGNED NOT NULL DEFAULT 30,
	Points INT NOT NULL DEFAULT 0,
	WaveLimit TINYINT UNSIGNED NOT NULL DEFAULT 0,
	Crest VARBINARY(40) NOT NULL DEFAULT '',
	MOTD VARCHAR(293) NOT NULL DEFAULT 'Default MOTD',
	NoticeWriter VARCHAR(20) NOT NULL DEFAULT '',
	NoticeTime DATETIME NOT NULL DEFAULT '1970-01-01 00:00:01',
	LeaderCharacterId INT UNSIGNED NOT NULL DEFAULT 0,
	CreatedAt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:01',
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (ID),
	UNIQUE KEY UX_Asda2Guild_Name (Name),
	KEY IX_Asda2Guild_Leader (LeaderCharacterId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`,
		`CREATE TABLE IF NOT EXISTS Asda2GuildRank (
	GuildId INT UNSIGNED NOT NULL,
	RankIndex TINYINT UNSIGNED NOT NULL,
	Name VARCHAR(20) NOT NULL DEFAULT '',
	Privileges SMALLINT UNSIGNED NOT NULL DEFAULT 0,
	PRIMARY KEY (GuildId, RankIndex),
	CONSTRAINT FK_Asda2GuildRank_Guild
		FOREIGN KEY (GuildId) REFERENCES Asda2Guild (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`,
		`CREATE TABLE IF NOT EXISTS Asda2GuildMember (
	CharacterId INT UNSIGNED NOT NULL,
	GuildId INT UNSIGNED NOT NULL,
	AccountId INT UNSIGNED NOT NULL,
	CharNum TINYINT UNSIGNED NOT NULL DEFAULT 0,
	Name VARCHAR(20) NOT NULL DEFAULT '',
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	ProfessionLevel TINYINT UNSIGNED NOT NULL DEFAULT 0,
	Class TINYINT UNSIGNED NOT NULL DEFAULT 0,
	RankIndex TINYINT UNSIGNED NOT NULL DEFAULT 4,
	PublicNote VARCHAR(60) NOT NULL DEFAULT '',
	LastLogin DATETIME NOT NULL DEFAULT '1970-01-01 00:00:01',
	LastMapId SMALLINT UNSIGNED NOT NULL DEFAULT 0,
	CreatedAt DATETIME NOT NULL DEFAULT '1970-01-01 00:00:01',
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (CharacterId),
	KEY IX_Asda2GuildMember_Guild (GuildId, RankIndex, Name),
	KEY IX_Asda2GuildMember_Account (AccountId),
	CONSTRAINT FK_Asda2GuildMember_Guild
		FOREIGN KEY (GuildId) REFERENCES Asda2Guild (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`,
		`CREATE TABLE IF NOT EXISTS Asda2GuildSkill (
	GuildId INT UNSIGNED NOT NULL,
	SkillId SMALLINT NOT NULL,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 1,
	IsActivated TINYINT(1) NOT NULL DEFAULT 0,
	LastMaintenance DATETIME NOT NULL DEFAULT '1970-01-01 00:00:01',
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (GuildId, SkillId),
	CONSTRAINT FK_Asda2GuildSkill_Guild
		FOREIGN KEY (GuildId) REFERENCES Asda2Guild (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`,
		`CREATE TABLE IF NOT EXISTS Asda2GuildHistory (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	GuildId INT UNSIGNED NOT NULL,
	Type TINYINT UNSIGNED NOT NULL,
	Value INT NOT NULL DEFAULT 0,
	TriggerName VARCHAR(20) NOT NULL DEFAULT '',
	EventTime VARCHAR(17) NOT NULL DEFAULT '',
	CreatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (ID),
	KEY IX_Asda2GuildHistory_Guild (GuildId, ID),
	CONSTRAINT FK_Asda2GuildHistory_Guild
		FOREIGN KEY (GuildId) REFERENCES Asda2Guild (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8`,
	}
	for _, stmt := range statements {
		if _, err := DB.Exec(stmt); err != nil {
			return fmt.Errorf("init guild db: %w", err)
		}
	}
	return nil
}

func ensureGuildCharacterColumns() error {
	if tableColumnExists("CharacterRecord", "GuildPoints") {
		return nil
	}
	if _, err := DB.Exec(`ALTER TABLE CharacterRecord ADD COLUMN GuildPoints INT NOT NULL DEFAULT 0`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil
		}
		return fmt.Errorf("add CharacterRecord.GuildPoints: %w", err)
	}
	return nil
}

func CreateGuild(name string, leader *types.Character) (*GuildData, error) {
	if DB == nil {
		return nil, fmt.Errorf("db is not initialized")
	}
	if leader == nil {
		return nil, fmt.Errorf("leader is nil")
	}
	name = strings.TrimSpace(name)
	tx, err := DB.Begin()
	if err != nil {
		return nil, err
	}
	defer rollbackUnlessCommitted(tx)

	var existing int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM Asda2Guild WHERE Name = ?`, name).Scan(&existing); err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, ErrGuildNameExists
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM Asda2GuildMember WHERE CharacterId = ?`, leader.GUID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing > 0 || leader.GuildID != 0 {
		return nil, ErrGuildCharacterAlreadyMember
	}

	res, err := tx.Exec(
		`INSERT INTO Asda2Guild
			(Name, Level, MaxMembers, Points, WaveLimit, Crest, MOTD, NoticeWriter, NoticeTime, LeaderCharacterId)
		 VALUES (?, 1, 30, 0, 0, ?, 'Default MOTD', '', NOW(), ?)`,
		name, make([]byte, GuildCrestLength), leader.GUID,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	guildID := uint32(id)

	for _, rank := range defaultGuildRanks(guildID) {
		if _, err := tx.Exec(
			`INSERT INTO Asda2GuildRank (GuildId, RankIndex, Name, Privileges)
			 VALUES (?, ?, ?, ?)`,
			rank.GuildID, rank.RankIndex, rank.Name, rank.Privileges,
		); err != nil {
			return nil, err
		}
	}

	if err := insertGuildMemberTx(tx, guildID, leader, 0); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE CharacterRecord SET GuildId = ? WHERE EntityLowId = ?`, guildID, leader.GUID); err != nil {
		return nil, err
	}
	if err := addGuildHistoryTx(tx, guildID, 1, 0, leader.Name, time.Now().Format("15:04:05")); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return LoadGuildByID(guildID)
}

func GuildNameExists(name string) (bool, error) {
	if DB == nil {
		return false, nil
	}
	var count int
	err := DB.QueryRow(`SELECT COUNT(*) FROM Asda2Guild WHERE Name = ?`, strings.TrimSpace(name)).Scan(&count)
	return count > 0, err
}

func LoadGuildByID(guildID uint32) (*GuildData, error) {
	if DB == nil || guildID == 0 {
		return nil, nil
	}
	g := &GuildData{}
	var crest []byte
	err := DB.QueryRow(
		`SELECT ID, Name, Level, MaxMembers, Points, WaveLimit, Crest,
		        MOTD, NoticeWriter, NoticeTime, LeaderCharacterId
		   FROM Asda2Guild
		  WHERE ID = ?
		  LIMIT 1`,
		guildID,
	).Scan(&g.ID, &g.Name, &g.Level, &g.MaxMembers, &g.Points, &g.WaveLimit, &crest,
		&g.MOTD, &g.NoticeWriter, &g.NoticeTime, &g.LeaderCharacterID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	g.Crest = normalizeGuildCrest(crest)
	if g.Ranks, err = LoadGuildRanks(guildID); err != nil {
		return nil, err
	}
	if len(g.Ranks) == 0 {
		if err := EnsureDefaultGuildRanks(guildID); err != nil {
			return nil, err
		}
		g.Ranks, err = LoadGuildRanks(guildID)
		if err != nil {
			return nil, err
		}
	}
	if g.Members, err = LoadGuildMembers(guildID); err != nil {
		return nil, err
	}
	if g.Skills, err = LoadGuildSkills(guildID); err != nil {
		return nil, err
	}
	if g.History, err = LoadGuildHistory(guildID, 12); err != nil {
		return nil, err
	}
	return g, nil
}

func LoadGuildForCharacter(characterID uint32) (*GuildData, error) {
	if DB == nil || characterID == 0 {
		return nil, nil
	}
	var guildID uint32
	err := DB.QueryRow(
		`SELECT GuildId FROM Asda2GuildMember WHERE CharacterId = ? LIMIT 1`,
		characterID,
	).Scan(&guildID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return LoadGuildByID(guildID)
}

func LoadGuildRanks(guildID uint32) ([]GuildRankData, error) {
	rows, err := DB.Query(
		`SELECT GuildId, RankIndex, Name, Privileges
		   FROM Asda2GuildRank
		  WHERE GuildId = ?
		  ORDER BY RankIndex ASC`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildRankData
	for rows.Next() {
		var r GuildRankData
		if err := rows.Scan(&r.GuildID, &r.RankIndex, &r.Name, &r.Privileges); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func LoadGuildMembers(guildID uint32) ([]GuildMemberData, error) {
	rows, err := DB.Query(
		`SELECT m.CharacterId, m.GuildId, m.AccountId, m.CharNum,
		        COALESCE(c.Name, m.Name) AS Name,
		        COALESCE(c.Level, m.Level) AS Level,
		        COALESCE(c.ProfessionLevel, m.ProfessionLevel) AS ProfessionLevel,
		        COALESCE(c.Asda2Class, m.Class) AS Class,
		        m.RankIndex, m.PublicNote, m.LastLogin,
		        COALESCE(c.Map, m.LastMapId) AS LastMapId,
		        COALESCE(c.GuildPoints, 0) AS GuildPoints
		   FROM Asda2GuildMember m
		   LEFT JOIN CharacterRecord c ON c.EntityLowId = m.CharacterId
		  WHERE m.GuildId = ?
		  ORDER BY m.RankIndex ASC, Name ASC`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildMemberData
	for rows.Next() {
		var m GuildMemberData
		var level, professionLevel, class, lastMap int
		if err := rows.Scan(
			&m.CharacterID, &m.GuildID, &m.AccountID, &m.CharNum, &m.Name,
			&level, &professionLevel, &class, &m.RankIndex, &m.PublicNote,
			&m.LastLogin, &lastMap, &m.GuildPoints,
		); err != nil {
			return nil, err
		}
		m.Level = byteFromDB(level)
		m.ProfessionLevel = byteFromDB(professionLevel)
		m.Class = byteFromDB(class)
		m.LastMapID = uint16FromDB(lastMap)
		out = append(out, m)
	}
	return out, rows.Err()
}

func LoadGuildSkills(guildID uint32) ([]GuildSkillData, error) {
	rows, err := DB.Query(
		`SELECT GuildId, SkillId, Level, IsActivated, LastMaintenance
		   FROM Asda2GuildSkill
		  WHERE GuildId = ?
		  ORDER BY SkillId ASC`,
		guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildSkillData
	for rows.Next() {
		var s GuildSkillData
		if err := rows.Scan(&s.GuildID, &s.SkillID, &s.Level, &s.IsActivated, &s.LastMaintenance); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func LoadGuildHistory(guildID uint32, limit int) ([]GuildHistoryData, error) {
	if limit <= 0 {
		limit = 12
	}
	rows, err := DB.Query(
		`SELECT ID, GuildId, Type, Value, TriggerName, EventTime, CreatedAt
		   FROM Asda2GuildHistory
		  WHERE GuildId = ?
		  ORDER BY ID DESC
		  LIMIT ?`,
		guildID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GuildHistoryData
	for rows.Next() {
		var h GuildHistoryData
		if err := rows.Scan(&h.ID, &h.GuildID, &h.Type, &h.Value, &h.TriggerName, &h.EventTime, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func AddGuildMember(guildID uint32, chr *types.Character, rankIndex byte) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)
	if err := insertGuildMemberTx(tx, guildID, chr, rankIndex); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE CharacterRecord SET GuildId = ? WHERE EntityLowId = ?`, guildID, chr.GUID); err != nil {
		return err
	}
	if err := addGuildHistoryTx(tx, guildID, 1, 0, chr.Name, time.Now().Format("15:04:05")); err != nil {
		return err
	}
	return tx.Commit()
}

func RemoveGuildMember(guildID uint32, characterID uint32, historyType byte, triggerName string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)
	if _, err := tx.Exec(`DELETE FROM Asda2GuildMember WHERE GuildId = ? AND CharacterId = ?`, guildID, characterID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE CharacterRecord SET GuildId = 0 WHERE EntityLowId = ?`, characterID); err != nil {
		return err
	}
	if historyType != 0 {
		if err := addGuildHistoryTx(tx, guildID, historyType, 0, triggerName, time.Now().Format("15:04:05")); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func DeleteGuild(guildID uint32) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)
	if _, err := tx.Exec(`UPDATE CharacterRecord SET GuildId = 0 WHERE GuildId = ?`, guildID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM Asda2Guild WHERE ID = ?`, guildID); err != nil {
		return err
	}
	return tx.Commit()
}

func UpdateGuildMemberRank(guildID uint32, characterID uint32, rankIndex byte) error {
	_, err := DB.Exec(
		`UPDATE Asda2GuildMember SET RankIndex = ? WHERE GuildId = ? AND CharacterId = ?`,
		rankIndex, guildID, characterID,
	)
	return err
}

func UpdateGuildMemberSnapshot(chr *types.Character) error {
	if DB == nil || chr == nil || chr.GuildID == 0 {
		return nil
	}
	_, err := DB.Exec(
		`UPDATE Asda2GuildMember
		    SET Name = ?, Level = ?, ProfessionLevel = ?, Class = ?, LastLogin = NOW(), LastMapId = ?
		  WHERE CharacterId = ? AND GuildId = ?`,
		chr.Name, chr.Level, chr.ProfessionLevel, chr.Class, chr.MapID, chr.GUID, chr.GuildID,
	)
	return err
}

func UpdateGuildMemberPublicNote(guildID uint32, characterID uint32, note string) error {
	_, err := DB.Exec(
		`UPDATE Asda2GuildMember SET PublicNote = ? WHERE GuildId = ? AND CharacterId = ?`,
		truncateString(note, 60), guildID, characterID,
	)
	return err
}

func UpdateGuildRanks(guildID uint32, privileges map[byte]uint16) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)
	for rankIndex, privilege := range privileges {
		if _, err := tx.Exec(
			`UPDATE Asda2GuildRank SET Privileges = ? WHERE GuildId = ? AND RankIndex = ?`,
			privilege, guildID, rankIndex,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func UpdateGuildMOTD(guildID uint32, motd string, writer string) error {
	_, err := DB.Exec(
		`UPDATE Asda2Guild
		    SET MOTD = ?, NoticeWriter = ?, NoticeTime = NOW()
		  WHERE ID = ?`,
		truncateString(motd, 293), truncateString(writer, 20), guildID,
	)
	return err
}

func UpdateGuildCrest(guildID uint32, crest []byte) error {
	_, err := DB.Exec(
		`UPDATE Asda2Guild SET Crest = ? WHERE ID = ?`,
		normalizeGuildCrest(crest), guildID,
	)
	return err
}

func UpdateGuildPoints(guildID uint32, points int32) error {
	_, err := DB.Exec(`UPDATE Asda2Guild SET Points = ? WHERE ID = ?`, points, guildID)
	return err
}

func UpdateGuildLevelAndPoints(guildID uint32, level byte, points int32) error {
	_, err := DB.Exec(`UPDATE Asda2Guild SET Level = ?, Points = ? WHERE ID = ?`, level, points, guildID)
	return err
}

func SaveGuildSkill(skill GuildSkillData) error {
	_, err := DB.Exec(
		`INSERT INTO Asda2GuildSkill (GuildId, SkillId, Level, IsActivated, LastMaintenance)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		    Level = VALUES(Level),
		    IsActivated = VALUES(IsActivated),
		    LastMaintenance = VALUES(LastMaintenance)`,
		skill.GuildID, skill.SkillID, skill.Level, skill.IsActivated, skill.LastMaintenance,
	)
	return err
}

func AddGuildHistory(guildID uint32, historyType byte, value int32, triggerName string, eventTime string) error {
	return addGuildHistoryTx(nil, guildID, historyType, value, triggerName, eventTime)
}

func EnsureDefaultGuildRanks(guildID uint32) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)
	for _, rank := range defaultGuildRanks(guildID) {
		if _, err := tx.Exec(
			`INSERT IGNORE INTO Asda2GuildRank (GuildId, RankIndex, Name, Privileges)
			 VALUES (?, ?, ?, ?)`,
			rank.GuildID, rank.RankIndex, rank.Name, rank.Privileges,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

var (
	ErrGuildNameExists             = fmt.Errorf("guild name already exists")
	ErrGuildCharacterAlreadyMember = fmt.Errorf("character is already in a guild")
)

func defaultGuildRanks(guildID uint32) []GuildRankData {
	return []GuildRankData{
		{GuildID: guildID, RankIndex: 0, Name: "Guild Master", Privileges: 127},
		{GuildID: guildID, RankIndex: 1, Name: "Officer", Privileges: 2},
		{GuildID: guildID, RankIndex: 2, Name: "Veteran", Privileges: 0},
		{GuildID: guildID, RankIndex: 3, Name: "Member", Privileges: 0},
		{GuildID: guildID, RankIndex: 4, Name: "Initiate", Privileges: 0},
	}
}

func insertGuildMemberTx(tx *sql.Tx, guildID uint32, chr *types.Character, rankIndex byte) error {
	if chr == nil {
		return fmt.Errorf("character is nil")
	}
	_, err := tx.Exec(
		`INSERT INTO Asda2GuildMember
		    (CharacterId, GuildId, AccountId, CharNum, Name, Level, ProfessionLevel,
		     Class, RankIndex, PublicNote, LastLogin, LastMapId)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', NOW(), ?)`,
		chr.GUID, guildID, chr.AccID, chr.CharNum, chr.Name, chr.Level, chr.ProfessionLevel,
		chr.Class, rankIndex, chr.MapID,
	)
	return err
}

func addGuildHistoryTx(tx *sql.Tx, guildID uint32, historyType byte, value int32, triggerName string, eventTime string) error {
	triggerName = truncateString(triggerName, 20)
	eventTime = truncateString(eventTime, 17)
	if tx != nil {
		_, err := tx.Exec(
			`INSERT INTO Asda2GuildHistory (GuildId, Type, Value, TriggerName, EventTime)
			 VALUES (?, ?, ?, ?, ?)`,
			guildID, historyType, value, triggerName, eventTime,
		)
		return err
	}
	_, err := DB.Exec(
		`INSERT INTO Asda2GuildHistory (GuildId, Type, Value, TriggerName, EventTime)
		 VALUES (?, ?, ?, ?, ?)`,
		guildID, historyType, value, triggerName, eventTime,
	)
	return err
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func normalizeGuildCrest(crest []byte) []byte {
	out := make([]byte, GuildCrestLength)
	copy(out, crest)
	return out
}

func truncateString(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}
