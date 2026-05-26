package db

import (
	"fmt"
	"log"

	"asda2/shared/types"
)

type WeatherRow = types.WeatherState

// InitWeatherDB creates the small Asda2 map-weather table used by the game
// server. A missing row means clear weather for that map.
func InitWeatherDB() error {
	if _, err := DB.Exec(`
CREATE TABLE IF NOT EXISTS Asda2Weather (
	MapId SMALLINT UNSIGNED NOT NULL,
	Channel SMALLINT NOT NULL DEFAULT -1,
	WeatherType TINYINT UNSIGNED NOT NULL DEFAULT 0,
	Level TINYINT UNSIGNED NOT NULL DEFAULT 0,
	IsEnabled TINYINT(1) NOT NULL DEFAULT 1,
	UpdatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (MapId, Channel),
	KEY IX_Asda2Weather_Channel (Channel, IsEnabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`); err != nil {
		return fmt.Errorf("create Asda2Weather: %w", err)
	}
	log.Printf("[WeatherDB] Asda2Weather ready")
	return nil
}

func LoadWeatherStates(channel byte) ([]WeatherRow, error) {
	rows, err := DB.Query(`
SELECT MapId, Channel, WeatherType, Level, IsEnabled
  FROM Asda2Weather
 WHERE IsEnabled = 1
   AND (Channel = -1 OR Channel = ?)
 ORDER BY MapId ASC, Channel ASC`, int(channel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WeatherRow
	for rows.Next() {
		var row WeatherRow
		var mapID, channelID, weatherType, level, enabled int
		if err := rows.Scan(&mapID, &channelID, &weatherType, &level, &enabled); err != nil {
			return nil, err
		}
		row.MapID = uint16FromDB(mapID)
		row.Channel = int16FromDB(channelID)
		row.Type = types.WeatherType(byteFromDB(weatherType))
		row.Level = byteFromDB(level)
		row.IsEnabled = enabled != 0
		out = append(out, types.NormalizeWeatherState(row))
	}
	return out, rows.Err()
}
