package relay

import (
	"fmt"
	"log"
	"time"

	"asda2/shared/db"
)

func InitBridgeDB() error {
	_, err := db.DB.Exec(`
CREATE TABLE IF NOT EXISTS ServerHandoff (
	AccountId INT UNSIGNED NOT NULL,
	CharNum TINYINT UNSIGNED NOT NULL,
	Channel TINYINT UNSIGNED NOT NULL,
	ClientIP VARCHAR(45) NOT NULL,
	CreatedAt DATETIME NOT NULL,
	ExpiresAt DATETIME NOT NULL,
	PRIMARY KEY (AccountId, CharNum),
	KEY IX_ServerHandoff_ExpiresAt (ExpiresAt)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`)
	if err != nil {
		return fmt.Errorf("create ServerHandoff: %w", err)
	}
	if _, err := db.DB.Exec(`DELETE FROM ServerHandoff WHERE ExpiresAt < NOW()`); err != nil {
		return fmt.Errorf("clean ServerHandoff: %w", err)
	}
	if err := initAccountSessionDB(); err != nil {
		return err
	}

	_, err = db.DB.Exec(`
CREATE TABLE IF NOT EXISTS ServerChannel (
	ChannelId TINYINT UNSIGNED NOT NULL,
	IP VARCHAR(45) NOT NULL,
	Port SMALLINT UNSIGNED NOT NULL,
	IsOnline TINYINT(1) NOT NULL,
	PlayerCount INT UNSIGNED NOT NULL,
	LastHeartbeat DATETIME NOT NULL,
	UpdatedAt DATETIME NOT NULL,
	PRIMARY KEY (ChannelId),
	KEY IX_ServerChannel_OnlineHeartbeat (IsOnline, LastHeartbeat)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`)
	if err != nil {
		return fmt.Errorf("create ServerChannel: %w", err)
	}
	if _, err := db.DB.Exec(
		`UPDATE ServerChannel SET IsOnline = 0, UpdatedAt = NOW()
		  WHERE IsOnline = 1 AND LastHeartbeat < ?`,
		mysqlDateTime(time.Now().Add(-ChannelHeartbeatMaxAge)),
	); err != nil {
		return fmt.Errorf("mark stale ServerChannel rows offline: %w", err)
	}

	log.Printf("[BridgeDB] ServerHandoff and ServerChannel ready")
	return nil
}

func mysqlDateTime(t time.Time) string {
	return t.Truncate(time.Second).Format("2006-01-02 15:04:05")
}
