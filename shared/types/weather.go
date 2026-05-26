package types

// WeatherType is the raw Asda2 client weather byte. The reference server keeps
// it numeric, so only the clear/default value is named here.
type WeatherType byte

const (
	WeatherTypeClear WeatherType = 0
)

// WeatherState is the Asda2-only map weather state consumed by SetClientTime.
type WeatherState struct {
	MapID     uint16
	Channel   int16
	Type      WeatherType
	Level     byte
	IsEnabled bool
}

func DefaultWeatherState(mapID uint16) WeatherState {
	return WeatherState{
		MapID:     NormalizeAsda2MapID(mapID),
		Channel:   -1,
		Type:      WeatherTypeClear,
		Level:     0,
		IsEnabled: true,
	}
}

func NormalizeWeatherState(state WeatherState) WeatherState {
	state.MapID = NormalizeAsda2MapID(state.MapID)
	if state.Channel < -1 {
		state.Channel = -1
	}
	return state
}
