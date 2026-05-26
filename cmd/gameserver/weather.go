package main

import (
	"log"
	"sync"
	"time"

	"asda2/shared/db"
	"asda2/shared/types"
)

const weatherSyncInterval = 10 * time.Second

var gameWeather = newWeatherRuntime()

type weatherRuntime struct {
	mu                      sync.RWMutex
	byMap                   map[uint16]types.WeatherState
	lastClientTimeSyncValue int
	startOnce               sync.Once
}

func newWeatherRuntime() *weatherRuntime {
	return &weatherRuntime{
		byMap:                   make(map[uint16]types.WeatherState),
		lastClientTimeSyncValue: -1,
	}
}

func initWeatherRuntime(channel byte) error {
	rows, err := db.LoadWeatherStates(channel)
	if err != nil {
		return err
	}
	gameWeather.replace(rows)
	log.Printf("[Weather] %d weather row(s) loaded for channel=%d", len(rows), channel)
	return nil
}

func startWeatherSyncLoop() {
	gameWeather.startSyncLoop()
}

func (r *weatherRuntime) replace(rows []types.WeatherState) {
	next := make(map[uint16]types.WeatherState, len(rows))
	for _, row := range rows {
		if !row.IsEnabled {
			continue
		}
		row = types.NormalizeWeatherState(row)
		next[row.MapID] = row
	}

	r.mu.Lock()
	r.byMap = next
	r.mu.Unlock()
}

func (r *weatherRuntime) stateForMap(mapID uint16) types.WeatherState {
	mapID = types.NormalizeAsda2MapID(mapID)

	r.mu.RLock()
	state, ok := r.byMap[mapID]
	r.mu.RUnlock()
	if ok {
		return state
	}
	return types.DefaultWeatherState(mapID)
}

func (r *weatherRuntime) startSyncLoop() {
	r.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(weatherSyncInterval)
			defer ticker.Stop()
			for range ticker.C {
				sendSetClientTimeToAllIfChanged(time.Now())
			}
		}()
	})
}

func (r *weatherRuntime) shouldSyncClientTime(value int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if value == r.lastClientTimeSyncValue {
		return false
	}
	r.lastClientTimeSyncValue = value
	return true
}

func weatherForClient(c *Client) types.WeatherState {
	if c == nil || c.Char == nil {
		return types.DefaultWeatherState(0)
	}
	return gameWeather.stateForMap(c.Char.MapID)
}

func sendSetClientTimeToAllIfChanged(now time.Time) {
	val1, val2 := clientTimeValues(now)
	clientTimeValue := val1<<8 | val2
	if !gameWeather.shouldSyncClientTime(clientTimeValue) {
		return
	}
	for _, c := range gameClientsSnapshot() {
		sendSetClientTimeAt(c, now)
	}
}
