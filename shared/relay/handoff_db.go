package relay

import (
	"database/sql"
	"log"
	"time"

	"asda2/shared/db"
)

func SavePendingLogin(p PendingLogin) error {
	now := time.Now()
	expiresAt := now.Add(2 * time.Minute)
	_, err := db.DB.Exec(
		`INSERT INTO ServerHandoff
		    (AccountId, CharNum, Channel, ClientIP, CreatedAt, ExpiresAt)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		    Channel=VALUES(Channel),
		    ClientIP=VALUES(ClientIP),
		    CreatedAt=VALUES(CreatedAt),
		    ExpiresAt=VALUES(ExpiresAt)`,
		p.AccountID, p.CharNum, p.Channel, p.ClientIP, mysqlDateTime(now), mysqlDateTime(expiresAt),
	)
	return err
}

func ConsumePendingLoginFromDB(accountID uint32, charNum byte, clientIP string) (PendingLogin, bool, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return PendingLogin{}, false, err
	}
	defer tx.Rollback()

	var p PendingLogin
	var expiresAt time.Time
	err = tx.QueryRow(
		`SELECT AccountId, CharNum, Channel, ClientIP, CreatedAt, ExpiresAt
		   FROM ServerHandoff
		  WHERE AccountId = ? AND CharNum = ?
		  LIMIT 1
		  FOR UPDATE`,
		accountID, charNum,
	).Scan(&p.AccountID, &p.CharNum, &p.Channel, &p.ClientIP, &p.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return PendingLogin{}, false, nil
	}
	if err != nil {
		return PendingLogin{}, false, err
	}

	if _, err := tx.Exec(`DELETE FROM ServerHandoff WHERE AccountId = ? AND CharNum = ?`, accountID, charNum); err != nil {
		return PendingLogin{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PendingLogin{}, false, err
	}

	if time.Now().After(expiresAt) {
		log.Printf("[BridgeDB] expired handoff account=%d charSlot=%d", accountID, charNum)
		return PendingLogin{}, false, nil
	}
	if p.ClientIP != "" && clientIP != "" && p.ClientIP != clientIP {
		log.Printf("[BridgeDB] handoff ip mismatch account=%d charSlot=%d expected=%s got=%s", accountID, charNum, p.ClientIP, clientIP)
		return PendingLogin{}, false, nil
	}
	return p, true, nil
}

func ConsumePendingAccountHandoffFromDB(accountID uint32, clientIP string) (PendingLogin, bool, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return PendingLogin{}, false, err
	}
	defer tx.Rollback()

	var p PendingLogin
	var expiresAt time.Time
	err = tx.QueryRow(
		`SELECT AccountId, CharNum, Channel, ClientIP, CreatedAt, ExpiresAt
		   FROM ServerHandoff
		  WHERE AccountId = ?
		  ORDER BY CreatedAt DESC
		  LIMIT 1
		  FOR UPDATE`,
		accountID,
	).Scan(&p.AccountID, &p.CharNum, &p.Channel, &p.ClientIP, &p.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return PendingLogin{}, false, nil
	}
	if err != nil {
		return PendingLogin{}, false, err
	}

	if _, err := tx.Exec(`DELETE FROM ServerHandoff WHERE AccountId = ? AND CharNum = ?`, p.AccountID, p.CharNum); err != nil {
		return PendingLogin{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return PendingLogin{}, false, err
	}

	if time.Now().After(expiresAt) {
		log.Printf("[BridgeDB] expired account handoff account=%d charSlot=%d", accountID, p.CharNum)
		return PendingLogin{}, false, nil
	}
	if p.ClientIP != "" && clientIP != "" && p.ClientIP != clientIP {
		log.Printf("[BridgeDB] account handoff ip mismatch account=%d charSlot=%d expected=%s got=%s", accountID, p.CharNum, p.ClientIP, clientIP)
		return PendingLogin{}, false, nil
	}
	return p, true, nil
}

type HandoffStatus struct {
	AccountID          uint32    `json:"accountId"`
	CharNum            byte      `json:"charNum"`
	Channel            byte      `json:"channel"`
	ClientIP           string    `json:"clientIp"`
	CreatedAt          time.Time `json:"createdAt"`
	ExpiresAt          time.Time `json:"expiresAt"`
	SecondsUntilExpiry int64     `json:"secondsUntilExpiry"`
}

func LoadPendingHandoffStatuses() ([]HandoffStatus, error) {
	if _, err := db.DB.Exec(`DELETE FROM ServerHandoff WHERE ExpiresAt < NOW()`); err != nil {
		return nil, err
	}

	rows, err := db.DB.Query(
		`SELECT AccountId, CharNum, Channel, ClientIP, CreatedAt, ExpiresAt
		   FROM ServerHandoff
		  ORDER BY CreatedAt ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var out []HandoffStatus
	for rows.Next() {
		status := HandoffStatus{}
		if err := rows.Scan(
			&status.AccountID, &status.CharNum, &status.Channel,
			&status.ClientIP, &status.CreatedAt, &status.ExpiresAt,
		); err != nil {
			return nil, err
		}
		status.SecondsUntilExpiry = int64(status.ExpiresAt.Sub(now).Seconds())
		if status.SecondsUntilExpiry < 0 {
			status.SecondsUntilExpiry = 0
		}
		out = append(out, status)
	}
	return out, rows.Err()
}
