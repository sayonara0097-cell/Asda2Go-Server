package db

import (
	"asda2/shared/types"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DB is the global connection pool — call initDB() at startup.
var DB *sql.DB

// DBConfig matches the connection string in RealmServerConfig.xml:
//
//	Server=127.0.0.1;Port=3306;Database=asda2_db;CharSet=utf8;Uid=root;Pwd=123456;
type DBConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

var DefaultDB = DBConfig{
	Host:     "127.0.0.1",
	Port:     3306,
	Database: "asda2_db",
	User:     "root",
	Password: "123456",
}

type AccountRow = types.AccountRow
type Character = types.Character
type CharacterRow = types.CharacterRow
type ItemRow = types.ItemRow
type FastSlotRow = types.FastSlotRow
type ItemTemplate = types.ItemTemplate

// initDB opens the pool and verifies connectivity.
func Init(cfg DBConfig) error {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=true&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	DB = db
	log.Printf("[DB] connected to %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	return nil
}

// ---- Account table ----
// Mirrors WCell.RealmServer.Auth.Accounts.Account (ActiveRecord table: "Account")

// GetAccountByName loads an account row by login name (case-insensitive on MySQL).
func GetAccountByName(name string) (*AccountRow, error) {
	row := DB.QueryRow(
		`SELECT AccountId, Name, Password, IsActive, RoleGroupName,
		        LastLogin, LastIP, Created
		 FROM Account WHERE Name = ? LIMIT 1`, name)
	a := &AccountRow{}
	err := row.Scan(
		&a.AccountID, &a.Name, &a.Password, &a.IsActive, &a.RoleGroup,
		&a.LastLogin, &a.LastIP, &a.Created,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// UpdateAccountLogin stamps the last-login time and IP.
func UpdateAccountLogin(accountID int, ip []byte) error {
	_, err := DB.Exec(
		`UPDATE Account SET LastLogin = NOW(), LastIP = ? WHERE AccountId = ?`,
		ip, accountID,
	)
	return err
}

// ---- CharacterRecord table ----
// Mirrors WCell.RealmServer.Database.CharacterRecord (table name: "CharacterRecord")
// Only fields used by Asda2 are included.

// GetCharactersByAccount returns all character rows for one account, ordered by creation time.
func GetCharactersByAccount(accountID int) ([]*CharacterRow, error) {
	rows, err := DB.Query(
		`SELECT EntityLowId, AccountId, Name, CharNum, Created,
		        Race, ClassId, Asda2Class, Gender, Skin, face, HairStyle, HairColor, EyesColor, AvatarMask,
		        PositionX, PositionY, PositionZ, Orientation, Map,
		        Level, Xp, Health, BaseHealth, Power, BasePower, Money,
		        BaseStrength, BaseStamina, BaseSpirit, BaseIntellect, BaseAgility, BaseLuck, FreeStatPoints, BonusSkillPoints,
		        Asda2FactionId, Asda2HonorPoints, TitlePoints, Rank,
		        DiscoveredTitles, GetedTitles, PreTitleId, PostTitleId,
		        ProfessionLevel, FishingLevel, CraftingLevel, CraftingExp, LearnedRecipes,
		        GlobalChatColorDb, ChatBanned, GuildId, GuildPoints,
		        WarehousePassword, PremiumWarehouseBagsCount, PremiumAvatarWarehouseBagsCount,
		        RebornCount, Zodiac, SettingsFlags, PetBoxEnchants, MountBoxExpands
		 FROM CharacterRecord WHERE AccountId = ? ORDER BY Created ASC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*CharacterRow
	for rows.Next() {
		c := &CharacterRow{}
		var warehousePass *string
		err := rows.Scan(
			&c.EntityLowID, &c.AccountID, &c.Name, &c.CharNum, &c.Created,
			&c.Race, &c.Class, &c.Asda2Class, &c.Gender, &c.Skin, &c.Face, &c.HairStyle, &c.HairColor, &c.EyesColor, &c.AvatarMask,
			&c.PositionX, &c.PositionY, &c.PositionZ, &c.Orientation, &c.Map,
			&c.Level, &c.Xp, &c.Health, &c.BaseHealth, &c.Power, &c.BasePower, &c.Money,
			&c.BaseStrength, &c.BaseStamina, &c.BaseSpirit, &c.BaseIntellect, &c.BaseAgility, &c.BaseLuck, &c.FreeStatPoints, &c.BonusSkillPoints,
			&c.Asda2FactionID, &c.Asda2HonorPoints, &c.TitlePoints, &c.Rank,
			&c.DiscoveredTitlesRaw, &c.GetedTitlesRaw, &c.PreTitleID, &c.PostTitleID,
			&c.ProfessionLevel, &c.FishingLevel, &c.CraftingLevel, &c.CraftingExp, &c.LearnedRecipesRaw,
			&c.GlobalChatColorDB, &c.ChatBanned, &c.GuildID, &c.GuildPoints,
			&warehousePass, &c.PremiumWarehouseBagsCount, &c.PremiumAvatarWarehouseBagsCount,
			&c.RebornCount, &c.Zodiac, &c.SettingsFlags, &c.PetBoxEnchants, &c.MountBoxExpands,
		)
		if err != nil {
			return nil, err
		}
		if warehousePass != nil {
			c.WarehousePassword = *warehousePass
		}
		if err := loadCharacterCollections(c); err != nil {
			log.Printf("[DB] loading character side data for %d: %v", c.EntityLowID, err)
		}
		normalizeCharacterRowBaseStats(c)
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCharacterByID loads a single character by its primary key.
func GetCharacterByID(entityLowID int64) (*CharacterRow, error) {
	rows, err := DB.Query(
		`SELECT EntityLowId, AccountId, Name, CharNum, Created,
		        Race, ClassId, Asda2Class, Gender, Skin, face, HairStyle, HairColor, EyesColor, AvatarMask,
		        PositionX, PositionY, PositionZ, Orientation, Map,
		        Level, Xp, Health, BaseHealth, Power, BasePower, Money,
		        BaseStrength, BaseStamina, BaseSpirit, BaseIntellect, BaseAgility, BaseLuck, FreeStatPoints, BonusSkillPoints,
		        Asda2FactionId, Asda2HonorPoints, TitlePoints, Rank,
		        DiscoveredTitles, GetedTitles, PreTitleId, PostTitleId,
		        ProfessionLevel, FishingLevel, CraftingLevel, CraftingExp, LearnedRecipes,
		        GlobalChatColorDb, ChatBanned, GuildId, GuildPoints,
		        WarehousePassword, PremiumWarehouseBagsCount, PremiumAvatarWarehouseBagsCount,
		        RebornCount, Zodiac, SettingsFlags, PetBoxEnchants, MountBoxExpands
		 FROM CharacterRecord WHERE EntityLowId = ? LIMIT 1`, entityLowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	c := &CharacterRow{}
	var warehousePass *string
	err = rows.Scan(
		&c.EntityLowID, &c.AccountID, &c.Name, &c.CharNum, &c.Created,
		&c.Race, &c.Class, &c.Asda2Class, &c.Gender, &c.Skin, &c.Face, &c.HairStyle, &c.HairColor, &c.EyesColor, &c.AvatarMask,
		&c.PositionX, &c.PositionY, &c.PositionZ, &c.Orientation, &c.Map,
		&c.Level, &c.Xp, &c.Health, &c.BaseHealth, &c.Power, &c.BasePower, &c.Money,
		&c.BaseStrength, &c.BaseStamina, &c.BaseSpirit, &c.BaseIntellect, &c.BaseAgility, &c.BaseLuck, &c.FreeStatPoints, &c.BonusSkillPoints,
		&c.Asda2FactionID, &c.Asda2HonorPoints, &c.TitlePoints, &c.Rank,
		&c.DiscoveredTitlesRaw, &c.GetedTitlesRaw, &c.PreTitleID, &c.PostTitleID,
		&c.ProfessionLevel, &c.FishingLevel, &c.CraftingLevel, &c.CraftingExp, &c.LearnedRecipesRaw,
		&c.GlobalChatColorDB, &c.ChatBanned, &c.GuildID, &c.GuildPoints,
		&warehousePass, &c.PremiumWarehouseBagsCount, &c.PremiumAvatarWarehouseBagsCount,
		&c.RebornCount, &c.Zodiac, &c.SettingsFlags, &c.PetBoxEnchants, &c.MountBoxExpands,
	)
	if err == nil && warehousePass != nil {
		c.WarehousePassword = *warehousePass
	}
	if err == nil {
		if loadErr := loadCharacterCollections(c); loadErr != nil {
			log.Printf("[DB] loading character side data for %d: %v", c.EntityLowID, loadErr)
		}
		normalizeCharacterRowBaseStats(c)
	}
	return c, err
}

// GetCharacterByAccountAndSlot mirrors CharacterRecord.FindByAccountAndSlot.
// Newer WCell references use this during game-server handoff because
// EntityLowId can come from the DB id generator instead of the legacy
// accountId + charNum * 1,000,000 formula.
func GetCharacterByAccountAndSlot(accountID int, charNum int16) (*CharacterRow, error) {
	rows, err := DB.Query(
		`SELECT EntityLowId, AccountId, Name, CharNum, Created,
		        Race, ClassId, Asda2Class, Gender, Skin, face, HairStyle, HairColor, EyesColor, AvatarMask,
		        PositionX, PositionY, PositionZ, Orientation, Map,
		        Level, Xp, Health, BaseHealth, Power, BasePower, Money,
		        BaseStrength, BaseStamina, BaseSpirit, BaseIntellect, BaseAgility, BaseLuck, FreeStatPoints, BonusSkillPoints,
		        Asda2FactionId, Asda2HonorPoints, TitlePoints, Rank,
		        DiscoveredTitles, GetedTitles, PreTitleId, PostTitleId,
		        ProfessionLevel, FishingLevel, CraftingLevel, CraftingExp, LearnedRecipes,
		        GlobalChatColorDb, ChatBanned, GuildId, GuildPoints,
		        WarehousePassword, PremiumWarehouseBagsCount, PremiumAvatarWarehouseBagsCount,
		        RebornCount, Zodiac, SettingsFlags, PetBoxEnchants, MountBoxExpands
		 FROM CharacterRecord
		 WHERE AccountId = ? AND CharNum = ?
		 ORDER BY Created ASC
		 LIMIT 1`, accountID, charNum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	c := &CharacterRow{}
	var warehousePass *string
	err = rows.Scan(
		&c.EntityLowID, &c.AccountID, &c.Name, &c.CharNum, &c.Created,
		&c.Race, &c.Class, &c.Asda2Class, &c.Gender, &c.Skin, &c.Face, &c.HairStyle, &c.HairColor, &c.EyesColor, &c.AvatarMask,
		&c.PositionX, &c.PositionY, &c.PositionZ, &c.Orientation, &c.Map,
		&c.Level, &c.Xp, &c.Health, &c.BaseHealth, &c.Power, &c.BasePower, &c.Money,
		&c.BaseStrength, &c.BaseStamina, &c.BaseSpirit, &c.BaseIntellect, &c.BaseAgility, &c.BaseLuck, &c.FreeStatPoints, &c.BonusSkillPoints,
		&c.Asda2FactionID, &c.Asda2HonorPoints, &c.TitlePoints, &c.Rank,
		&c.DiscoveredTitlesRaw, &c.GetedTitlesRaw, &c.PreTitleID, &c.PostTitleID,
		&c.ProfessionLevel, &c.FishingLevel, &c.CraftingLevel, &c.CraftingExp, &c.LearnedRecipesRaw,
		&c.GlobalChatColorDB, &c.ChatBanned, &c.GuildID, &c.GuildPoints,
		&warehousePass, &c.PremiumWarehouseBagsCount, &c.PremiumAvatarWarehouseBagsCount,
		&c.RebornCount, &c.Zodiac, &c.SettingsFlags, &c.PetBoxEnchants, &c.MountBoxExpands,
	)
	if err == nil && warehousePass != nil {
		c.WarehousePassword = *warehousePass
	}
	if err == nil {
		if loadErr := loadCharacterCollections(c); loadErr != nil {
			log.Printf("[DB] loading character side data for %d: %v", c.EntityLowID, loadErr)
		}
		normalizeCharacterRowBaseStats(c)
	}
	return c, err
}

func normalizeCharacterRowBaseStats(c *CharacterRow) {
	changed, err := ApplyBaseStatsToCharacterRow(c, false)
	if err != nil {
		log.Printf("[BaseStatsDB] applying character base stats for %d: %v", c.EntityLowID, err)
		return
	}
	if changed {
		log.Printf("[BaseStatsDB] applied base stats to %q class=%d level=%d", c.Name, c.Asda2Class, c.Level)
	}
}

func loadCharacterCollections(c *CharacterRow) error {
	items, err := GetItemsByOwner(uint32(c.EntityLowID))
	if err != nil {
		return err
	}
	c.LoadedItems = items

	fastSlots, err := GetFastSlotsByOwner(uint32(c.EntityLowID))
	if err != nil {
		return err
	}
	c.LoadedFastSlots = fastSlots

	teleports, err := GetTeleportPointsByOwner(uint32(c.EntityLowID))
	if err != nil {
		return err
	}
	c.LoadedTeleports = teleports

	skills, err := GetCharacterSkillsByOwner(uint32(c.EntityLowID))
	if err != nil {
		return err
	}
	c.LearnedSkills = skills
	return nil
}

// SaveCharacter writes runtime state back to CharacterRecord.
func SaveCharacter(c *Character) error {
	_, err := DB.Exec(
		`UPDATE CharacterRecord SET
		    PositionX=?, PositionY=?, Orientation=?, Map=?,
		    ClassId=?, Asda2Class=?, ProfessionLevel=?,
		    Health=?, BaseHealth=?, Power=?, BasePower=?,
		    BaseStrength=?, BaseStamina=?, BaseSpirit=?, BaseIntellect=?, BaseAgility=?, BaseLuck=?, BonusSkillPoints=?,
		    Xp=?, Money=?,
		    Level=?, Asda2FactionId=?, Asda2HonorPoints=?,
		    TitlePoints=?, Rank=?, PreTitleId=?, PostTitleId=?,
		    FishingLevel=?, CraftingLevel=?, CraftingExp=?, LearnedRecipes=?,
		    GuildId=?, GuildPoints=?, ChatBanned=?,
		    RebornCount=?, SettingsFlags=?, AvatarMask=?,
		    WarehousePassword=?, PremiumWarehouseBagsCount=?, PremiumAvatarWarehouseBagsCount=?
		 WHERE EntityLowId=?`,
		c.X, c.Y, c.Orientation, c.MapID,
		c.Class, c.Class, c.ProfessionLevel,
		c.HP, c.MaxHP, c.MP, c.MaxMP,
		c.BaseStrength, c.BaseStamina, c.BaseSpirit, c.BaseIntellect, c.BaseAgility, c.BaseLuck, c.BonusSkillPoints,
		c.Exp, c.Gold,
		c.Level, c.FactionID, c.HonorPoints,
		c.TitlePoints, c.Rank, c.PreTitleID, c.PostTitleID,
		c.FishingLevel, c.CraftingLevel, c.CraftingExp, types.EncodeLearnedRecipeMask(c.LearnedRecipes),
		c.GuildID, c.GuildPoints, c.ChatBanned,
		c.RebornCount, c.SettingsFlags[:], c.AvatarMask,
		c.WarehousePassword, c.PremiumWarehouseBagsCount, c.PremiumAvatarWarehouseBagsCount,
		c.GUID,
	)
	return err
}

// CreateCharacter inserts a new CharacterRecord row.
// Returns the new EntityLowId (auto-assigned by the caller, see nextCharID).
func CreateCharacter(r *CharacterRow) error {
	if len(r.SettingsFlags) < len(types.DefaultSettingsFlags) {
		r.SettingsFlags = append([]byte(nil), types.DefaultSettingsFlags[:]...)
	}
	_, err := DB.Exec(
		`INSERT INTO CharacterRecord
		    (EntityLowId, AccountId, Name, CharNum, Created,
		     Race, ClassId, Asda2Class, Gender, Skin, face, HairStyle, HairColor, EyesColor,
		     PositionX, PositionY, PositionZ, Orientation, Map,
		     Level, Xp, Health, BaseHealth, Power, BasePower, Money,
		     BaseStrength, BaseStamina, BaseSpirit, BaseIntellect, BaseAgility, BaseLuck, FreeStatPoints, BonusSkillPoints,
		     Asda2FactionId, Asda2HonorPoints, TitlePoints, Rank, PreTitleId, PostTitleId,
		     ProfessionLevel, FishingLevel, CraftingLevel, CraftingExp,
		     GlobalChatColorDb, ChatBanned, GuildId,
		     WarehousePassword, PremiumWarehouseBagsCount, PremiumAvatarWarehouseBagsCount,
		     RebornCount, Zodiac, SettingsFlags, PetBoxEnchants, MountBoxExpands,
		     DiscoveredTitles, GetedTitles, LearnedRecipes, AvatarMask)
		 VALUES (?,?,?,?,NOW(),
		         ?,?,?,?,?,?,?,?,?,
		         ?,?,?,?,?,
		         ?,?,?,?,?,?,?,?,
		         ?,?,?,?,?,?,?,
		         ?,?,?,?,?,?,
		         ?,?,?,?,
		         ?,?,?,
		         ?,-1,0,
		         ?,?,?,?,?,
		         ?,?,?,?)`,
		r.EntityLowID, r.AccountID, r.Name, r.CharNum,
		r.Race, r.Class, r.Asda2Class, r.Gender, r.Skin, r.Face, r.HairStyle, r.HairColor, r.EyesColor,
		r.PositionX, r.PositionY, r.PositionZ, r.Orientation, r.Map,
		r.Level, r.Xp, r.Health, r.BaseHealth, r.Power, r.BasePower, r.Money,
		r.BaseStrength, r.BaseStamina, r.BaseSpirit, r.BaseIntellect, r.BaseAgility, r.BaseLuck, r.FreeStatPoints, r.BonusSkillPoints,
		r.Asda2FactionID, r.Asda2HonorPoints, r.TitlePoints, r.Rank, r.PreTitleID, r.PostTitleID,
		r.ProfessionLevel, r.FishingLevel, r.CraftingLevel, r.CraftingExp,
		r.GlobalChatColorDB, r.ChatBanned, r.GuildID,
		r.WarehousePassword,
		r.PremiumWarehouseBagsCount, r.PremiumAvatarWarehouseBagsCount,
		r.RebornCount, r.Zodiac, r.SettingsFlags, r.PetBoxEnchants, r.MountBoxExpands,
		r.DiscoveredTitlesRaw, r.GetedTitlesRaw, r.LearnedRecipesRaw, r.AvatarMask,
	)
	return err
}

// DeleteCharacter removes a character and its accessories.
// Mirrors CharacterRecord.DeleteCharAccessories.
func DeleteCharacter(entityLowID int64) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	tables := []string{
		"Asda2Item", "Asda2FastItemSlot", "Asda2TeleportingPointRecord",
		"SpellRecord", "AuraRecord", "ItemRecord",
		"SkillRecord", "QuestRecord",
	}
	for _, t := range tables {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE OwnerId = ?`, entityLowID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete %s: %w", t, err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM CharacterRecord WHERE EntityLowId = ?`, entityLowID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// CharacterNameExists checks for a duplicate name in CharacterRecord.
func CharacterNameExists(name string) (bool, error) {
	row := DB.QueryRow(
		`SELECT COUNT(*) FROM CharacterRecord WHERE Name = ? LIMIT 1`,
		strings.TrimSpace(name),
	)
	var count int
	err := row.Scan(&count)
	return count > 0, err
}

// GetAccountByID loads an account row by numeric ID.
func GetAccountByID(id int) (*AccountRow, error) {
	row := DB.QueryRow(
		`SELECT AccountId, Name, Password, IsActive
		   FROM Account
		  WHERE AccountId = ?
		  LIMIT 1`, id)
	a := &AccountRow{}
	if err := row.Scan(&a.AccountID, &a.Name, &a.Password, &a.IsActive); err != nil {
		return nil, err
	}
	return a, nil
}
