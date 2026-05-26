package main

import (
	"time"

	"asda2/shared/types"
)

// sendSetClientTime mirrors GlobalHandler.SendSetClientTimeResponse in the
// weather-focused reference build. Packet payload is 6 bytes:
// time part 1, time part 2, reserved, weather type, intensity low, intensity high.
func sendSetClientTime(c *Client) {
	sendSetClientTimeAt(c, time.Now())
}

func sendSetClientTimeAt(c *Client, now time.Time) {
	if c == nil {
		return
	}
	p := NewPacket(SetClientTime)
	p.WriteBytes(setClientTimePayload(now, weatherForClient(c)))
	c.Send(p)
}

func setClientTimePayload(now time.Time, weather types.WeatherState) []byte {
	val1, val2 := clientTimeValues(now)
	intensity := uint16(weather.Level) + 1
	return []byte{
		byte(val1),
		byte(val2),
		0,
		byte(weather.Type),
		byte(intensity),
		byte(intensity >> 8),
	}
}

func clientTimeValues(now time.Time) (int, int) {
	hour := now.Hour()
	minute := now.Minute()
	if hour < 6 {
		return 0, hour*10 + minute/6
	}
	val1 := hour / 6
	val2 := (hour-val1*6)*10 + minute/6
	return val1, val2
}
