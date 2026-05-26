package relay

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"asda2/shared/db"
)

type AccountSessionState string

const (
	AccountSessionLogin   AccountSessionState = "login"
	AccountSessionHandoff AccountSessionState = "handoff"
	AccountSessionGame    AccountSessionState = "game"

	AccountSessionLease             = 45 * time.Second
	AccountSessionHeartbeatInterval = 3 * time.Second
)

var ErrAccountSessionLost = errors.New("account session lease lost")

func initAccountSessionDB() error {
	_, err := db.DB.Exec(`
CREATE TABLE IF NOT EXISTS ServerAccountSession (
	AccountId INT UNSIGNED NOT NULL,
	OwnerToken VARCHAR(64) NOT NULL,
	SessionState VARCHAR(16) NOT NULL,
	Channel TINYINT UNSIGNED NOT NULL DEFAULT 0,
	CharNum TINYINT UNSIGNED NOT NULL DEFAULT 0,
	ClientIP VARCHAR(45) NOT NULL,
	CreatedAt DATETIME NOT NULL,
	LastHeartbeat DATETIME NOT NULL,
	ExpiresAt DATETIME NOT NULL,
	PRIMARY KEY (AccountId),
	KEY IX_ServerAccountSession_ExpiresAt (ExpiresAt),
	KEY IX_ServerAccountSession_State (SessionState)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`)
	if err != nil {
		return fmt.Errorf("create ServerAccountSession: %w", err)
	}
	if _, err := db.DB.Exec(`DELETE FROM ServerAccountSession WHERE ExpiresAt < NOW()`); err != nil {
		return fmt.Errorf("clean ServerAccountSession: %w", err)
	}
	return nil
}

func NewAccountSessionToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func ClaimAccountSession(accountID uint32, ownerToken string, state AccountSessionState, channel byte, charNum byte, clientIP string, allowHandoffTakeover bool) (bool, error) {
	return claimAccountSession(accountID, ownerToken, state, channel, charNum, clientIP, allowHandoffTakeover, false)
}

func ForceClaimAccountSession(accountID uint32, ownerToken string, state AccountSessionState, channel byte, charNum byte, clientIP string) error {
	_, err := claimAccountSession(accountID, ownerToken, state, channel, charNum, clientIP, false, true)
	return err
}

func claimAccountSession(accountID uint32, ownerToken string, state AccountSessionState, channel byte, charNum byte, clientIP string, allowHandoffTakeover bool, force bool) (bool, error) {
	if ownerToken == "" {
		return false, fmt.Errorf("account session token is required")
	}

	now := time.Now()
	expiresAt := now.Add(AccountSessionLease)
	tx, err := db.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM ServerAccountSession WHERE ExpiresAt < ?`, mysqlDateTime(now)); err != nil {
		return false, err
	}

	var existingToken string
	var existingState string
	err = tx.QueryRow(
		`SELECT OwnerToken, SessionState
		   FROM ServerAccountSession
		  WHERE AccountId = ?
		  LIMIT 1
		  FOR UPDATE`,
		accountID,
	).Scan(&existingToken, &existingState)
	switch {
	case err == sql.ErrNoRows:
		if _, err := tx.Exec(
			`INSERT INTO ServerAccountSession
			    (AccountId, OwnerToken, SessionState, Channel, CharNum, ClientIP, CreatedAt, LastHeartbeat, ExpiresAt)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, ownerToken, string(state), channel, charNum, clientIP,
			mysqlDateTime(now), mysqlDateTime(now), mysqlDateTime(expiresAt),
		); err != nil {
			return false, err
		}
	case err != nil:
		return false, err
	case force || existingToken == ownerToken || (allowHandoffTakeover && existingState == string(AccountSessionHandoff)):
		if _, err := tx.Exec(
			`UPDATE ServerAccountSession
			    SET OwnerToken = ?, SessionState = ?, Channel = ?, CharNum = ?,
			        ClientIP = ?, LastHeartbeat = ?, ExpiresAt = ?
			  WHERE AccountId = ?`,
			ownerToken, string(state), channel, charNum, clientIP,
			mysqlDateTime(now), mysqlDateTime(expiresAt), accountID,
		); err != nil {
			return false, err
		}
	default:
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func TouchAccountSession(accountID uint32, ownerToken string) error {
	if ownerToken == "" {
		return nil
	}
	now := time.Now()
	result, err := db.DB.Exec(
		`UPDATE ServerAccountSession
		    SET LastHeartbeat = ?, ExpiresAt = ?
		  WHERE AccountId = ? AND OwnerToken = ?`,
		mysqlDateTime(now), mysqlDateTime(now.Add(AccountSessionLease)), accountID, ownerToken,
	)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return ErrAccountSessionLost
	}
	return err
}

func ReleaseAccountSession(accountID uint32, ownerToken string) error {
	if ownerToken == "" {
		return nil
	}
	_, err := db.DB.Exec(
		`DELETE FROM ServerAccountSession
		  WHERE AccountId = ? AND OwnerToken = ? AND SessionState <> ?`,
		accountID, ownerToken, string(AccountSessionHandoff),
	)
	return err
}

func StartAccountSessionHeartbeat(accountID uint32, ownerToken string, onLost ...func()) func() {
	if ownerToken == "" {
		return func() {}
	}
	stop := make(chan struct{})
	var once sync.Once

	go func() {
		ticker := time.NewTicker(AccountSessionHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := TouchAccountSession(accountID, ownerToken); err != nil {
					if errors.Is(err, ErrAccountSessionLost) {
						log.Printf("[AccountSession] lease lost account=%d; closing old connection", accountID)
						for _, fn := range onLost {
							if fn != nil {
								fn()
							}
						}
						return
					}
					log.Printf("[AccountSession] heartbeat account=%d failed: %v", accountID, err)
				}
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stop)
		})
	}
}
