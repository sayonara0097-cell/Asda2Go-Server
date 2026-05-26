package main

import (
	"testing"
	"time"

	"asda2/shared/types"
)

func TestClientTimeValuesBeforeDawn(t *testing.T) {
	now := time.Date(2026, 5, 25, 5, 59, 0, 0, time.Local)
	val1, val2 := clientTimeValues(now)
	if val1 != 0 || val2 != 59 {
		t.Fatalf("clientTimeValues() = (%d, %d), want (0, 59)", val1, val2)
	}
}

func TestClientTimeValuesAfterDawn(t *testing.T) {
	now := time.Date(2026, 5, 25, 13, 35, 0, 0, time.Local)
	val1, val2 := clientTimeValues(now)
	if val1 != 2 || val2 != 15 {
		t.Fatalf("clientTimeValues() = (%d, %d), want (2, 15)", val1, val2)
	}
}

func TestSetClientTimePayloadIncludesWeather(t *testing.T) {
	now := time.Date(2026, 5, 25, 18, 0, 0, 0, time.Local)
	payload := setClientTimePayload(now, types.WeatherState{Type: types.WeatherType(3), Level: 7})
	want := []byte{3, 0, 0, 3, 8, 0}
	if string(payload) != string(want) {
		t.Fatalf("setClientTimePayload() = %v, want %v", payload, want)
	}
}

func TestSetClientTimePayloadEncodesMaxWeatherLevel(t *testing.T) {
	now := time.Date(2026, 5, 25, 0, 0, 0, 0, time.Local)
	payload := setClientTimePayload(now, types.WeatherState{Level: 255})
	want := []byte{0, 0, 0, 0, 0, 1}
	if string(payload) != string(want) {
		t.Fatalf("setClientTimePayload() = %v, want %v", payload, want)
	}
}
