package main

import (
	"encoding/binary"
	"testing"
)

func TestReadClientUiActionTargetIDUsesReferenceOffset(t *testing.T) {
	raw := make([]byte, clientUiActionTargetIDOffset+4)
	binary.LittleEndian.PutUint32(raw[clientUiActionTargetIDOffset:], 1001)

	if got := readClientUiActionTargetID(raw); got != 1001 {
		t.Fatalf("target id = %d, want 1001", got)
	}
}

func TestReadClientUiActionTargetIDFallsBackToShort(t *testing.T) {
	raw := make([]byte, clientUiActionTargetIDOffset+2)
	binary.LittleEndian.PutUint16(raw[clientUiActionTargetIDOffset:], 12)

	if got := readClientUiActionTargetID(raw); got != 12 {
		t.Fatalf("target id = %d, want 12", got)
	}
}
