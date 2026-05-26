package main

import "testing"

func TestReadChangeChannelRequestPrefersActualSelectionAfterPreamble(t *testing.T) {
	data := make([]byte, 29)
	data[24] = 0
	data[25] = 2

	channel, ok := readChangeChannelRequest(&PacketIn{Data: data}, 1)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 2 {
		t.Fatalf("channel = %d, want 2", channel)
	}
}

func TestReadChangeChannelRequestUsesChannelAfterAccountID(t *testing.T) {
	data := []byte{
		0xA6, 0x4D, 0x35, 0xE9, 0xBF, 0x8B, 0x00, 0x00,
		0xAC, 0x3E, 0x15, 0x0E, 0x41, 0x9D, 0x35, 0x8D,
		0x08, 0xDD, 0x3B, 0x08, 0x32, 0xF4, 0x87, 0x56,
		0xB8, 0x00, 0x00, 0x00, 0x02,
	}

	channel, ok := readChangeChannelRequest(&PacketIn{Data: data}, 0)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 2 {
		t.Fatalf("channel = %d, want 2", channel)
	}
}

func TestReadChangeChannelRequestSupportsOneBasedThirdChannel(t *testing.T) {
	channel, ok := readChangeChannelRequest(&PacketIn{Data: []byte{3}}, 0)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 2 {
		t.Fatalf("channel = %d, want 2", channel)
	}
}

func TestReadChangeChannelRequestFallsBackToLegacyOffset(t *testing.T) {
	data := make([]byte, 25)
	data[24] = 1

	channel, ok := readChangeChannelRequest(&PacketIn{Data: data}, 2)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 1 {
		t.Fatalf("channel = %d, want 1", channel)
	}
}

func TestReadChangeChannelRequestPrefersNonZeroSelectionOverPadding(t *testing.T) {
	data := make([]byte, 29)
	data[24] = 0
	data[25] = 0
	data[0] = 2

	channel, ok := readChangeChannelRequest(&PacketIn{Data: data}, 1)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 2 {
		t.Fatalf("channel = %d, want 2", channel)
	}
}

func TestReadChangeChannelRequestAllowsChannelOneWhenOnlyZeroCandidate(t *testing.T) {
	data := make([]byte, 29)
	data[24] = 0
	data[25] = 0
	data[0] = 0

	channel, ok := readChangeChannelRequest(&PacketIn{Data: data}, 2)
	if !ok {
		t.Fatal("expected channel request to parse")
	}
	if channel != 0 {
		t.Fatalf("channel = %d, want 0", channel)
	}
}
