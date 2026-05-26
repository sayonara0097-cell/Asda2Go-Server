package relay

import (
	"fmt"
	"log"
	"time"

	"asda2/shared/db"
)

func SaveChannelHeartbeat(endpoint ChannelEndpoint, playerCount int) error {
	if !ValidGameChannel(endpoint.Channel) {
		return fmt.Errorf("unsupported channel %d", endpoint.Channel)
	}
	now := time.Now()
	_, err := db.DB.Exec(
		`INSERT INTO ServerChannel
		    (ChannelId, IP, Port, IsOnline, PlayerCount, LastHeartbeat, UpdatedAt)
		 VALUES (?, ?, ?, 1, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		    IP=VALUES(IP),
		    Port=VALUES(Port),
		    IsOnline=VALUES(IsOnline),
		    PlayerCount=VALUES(PlayerCount),
		    LastHeartbeat=VALUES(LastHeartbeat),
		    UpdatedAt=VALUES(UpdatedAt)`,
		endpoint.Channel, endpoint.IP, endpoint.Port, playerCount, mysqlDateTime(now), mysqlDateTime(now),
	)
	return err
}

func LoadOnlineChannelEndpoints(maxAge time.Duration) ([]ChannelEndpoint, error) {
	cutoff := time.Now().Add(-maxAge).Truncate(time.Second)
	if _, err := db.DB.Exec(
		`UPDATE ServerChannel SET IsOnline = 0, UpdatedAt = NOW()
		  WHERE IsOnline = 1 AND LastHeartbeat < ?`,
		mysqlDateTime(cutoff),
	); err != nil {
		return nil, err
	}

	rows, err := db.DB.Query(
		`SELECT ChannelId, IP, Port
		   FROM ServerChannel
		  WHERE IsOnline = 1 AND LastHeartbeat >= ?
		  ORDER BY ChannelId ASC`,
		mysqlDateTime(cutoff),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []ChannelEndpoint
	for rows.Next() {
		var channel int
		var port int
		endpoint := ChannelEndpoint{}
		if err := rows.Scan(&channel, &endpoint.IP, &port); err != nil {
			return nil, err
		}
		if channel < 0 || channel > 255 || port < 0 || port > 65535 || !ValidGameChannel(byte(channel)) {
			log.Printf("[BridgeDB] ignoring invalid channel endpoint channel=%d port=%d", channel, port)
			continue
		}
		endpoint.Channel = byte(channel)
		endpoint.Port = uint16(port)
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}

type ChannelStatus struct {
	Channel        byte      `json:"channel"`
	IP             string    `json:"ip"`
	Port           uint16    `json:"port"`
	Online         bool      `json:"online"`
	PlayerCount    int       `json:"playerCount"`
	LastHeartbeat  time.Time `json:"lastHeartbeat"`
	UpdatedAt      time.Time `json:"updatedAt"`
	AgeSeconds     int64     `json:"ageSeconds"`
	StaleThreshold int64     `json:"staleThresholdSeconds"`
}

func LoadChannelPlayerCounts(maxAge time.Duration) ([GameChannelCount]int, error) {
	var counts [GameChannelCount]int
	statuses, err := LoadChannelStatuses(maxAge)
	if err != nil {
		return counts, err
	}
	for _, status := range statuses {
		if !status.Online || !ValidGameChannel(status.Channel) {
			continue
		}
		counts[status.Channel] = status.PlayerCount
	}
	return counts, nil
}

func LoadChannelStatuses(maxAge time.Duration) ([]ChannelStatus, error) {
	cutoff := time.Now().Add(-maxAge).Truncate(time.Second)
	if _, err := db.DB.Exec(
		`UPDATE ServerChannel SET IsOnline = 0, UpdatedAt = NOW()
		  WHERE IsOnline = 1 AND LastHeartbeat < ?`,
		mysqlDateTime(cutoff),
	); err != nil {
		return nil, err
	}

	rows, err := db.DB.Query(
		`SELECT ChannelId, IP, Port, IsOnline, PlayerCount, LastHeartbeat, UpdatedAt
		   FROM ServerChannel
		  ORDER BY ChannelId ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var out []ChannelStatus
	for rows.Next() {
		var channel int
		var port int
		var online int
		status := ChannelStatus{}
		if err := rows.Scan(
			&channel, &status.IP, &port, &online, &status.PlayerCount,
			&status.LastHeartbeat, &status.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if channel < 0 || channel > 255 || port < 0 || port > 65535 || !ValidGameChannel(byte(channel)) {
			log.Printf("[BridgeDB] ignoring invalid channel status channel=%d port=%d", channel, port)
			continue
		}
		status.Channel = byte(channel)
		status.Port = uint16(port)
		status.Online = online != 0 && !status.LastHeartbeat.Before(cutoff)
		status.AgeSeconds = int64(now.Sub(status.LastHeartbeat).Seconds())
		status.StaleThreshold = int64(maxAge.Seconds())
		out = append(out, status)
	}
	return out, rows.Err()
}
