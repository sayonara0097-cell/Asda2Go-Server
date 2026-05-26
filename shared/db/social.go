package db

import (
	"database/sql"
	"strings"
)

type FriendRow struct {
	CharacterID     uint32
	AccountID       uint32
	CharNum         byte
	Name            string
	MapID           uint16
	Level           byte
	ProfessionLevel byte
	Class           byte
	Online          bool
}

func AddFriendship(aCharacterID uint32, bCharacterID uint32) error {
	if DB == nil || aCharacterID == 0 || bCharacterID == 0 || aCharacterID == bCharacterID {
		return nil
	}
	_, err := DB.Exec(
		`INSERT IGNORE INTO Asda2Friendship (OwnerId, FriendId, CreatedAt)
		 VALUES (?, ?, NOW()), (?, ?, NOW())`,
		aCharacterID, bCharacterID, bCharacterID, aCharacterID,
	)
	if socialTableMissing(err) {
		return nil
	}
	return err
}

func DeleteFriendship(aCharacterID uint32, bCharacterID uint32) error {
	if DB == nil || aCharacterID == 0 || bCharacterID == 0 {
		return nil
	}
	_, err := DB.Exec(
		`DELETE FROM Asda2Friendship
		  WHERE (OwnerId = ? AND FriendId = ?)
		     OR (OwnerId = ? AND FriendId = ?)`,
		aCharacterID, bCharacterID, bCharacterID, aCharacterID,
	)
	if socialTableMissing(err) {
		return nil
	}
	return err
}

func GetFriendsByOwner(ownerID uint32) ([]FriendRow, error) {
	if DB == nil || ownerID == 0 {
		return nil, nil
	}
	rows, err := DB.Query(
		`SELECT c.EntityLowId, c.AccountId, c.CharNum, c.Name, c.Map, c.Level,
		        c.ProfessionLevel, c.Asda2Class
		   FROM Asda2Friendship f
		   JOIN CharacterRecord c ON c.EntityLowId = f.FriendId
		  WHERE f.OwnerId = ?
		  ORDER BY c.Name ASC`,
		ownerID,
	)
	if err != nil {
		if socialTableMissing(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var out []FriendRow
	for rows.Next() {
		var row FriendRow
		var characterID, accountID uint32
		var charNum, level, professionLevel, class int
		var mapID int
		if err := rows.Scan(
			&characterID, &accountID, &charNum, &row.Name, &mapID, &level,
			&professionLevel, &class,
		); err != nil {
			return nil, err
		}
		row.CharacterID = characterID
		row.AccountID = accountID
		row.CharNum = byte(charNum)
		row.MapID = uint16(mapID)
		row.Level = byte(level)
		row.ProfessionLevel = byte(professionLevel)
		row.Class = byte(class)
		out = append(out, row)
	}
	return out, rows.Err()
}

func socialTableMissing(err error) bool {
	if err == nil || err == sql.ErrNoRows {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "asda2friendship") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "unknown table")
}
