package packet

import (
	"bytes"
	"encoding/binary"
	"testing"

	"asda2/shared/crypt"
)

func TestPacketOutFinalizeDoesNotMutatePacket(t *testing.T) {
	p := NewPacket(Ping)
	p.WriteUint8(1)

	original := append([]byte(nil), p.buf...)
	first := p.Finalize(crypt.LocaleStart)
	second := p.Finalize(crypt.LocaleStart)

	if !bytes.Equal(p.buf, original) {
		t.Fatal("Finalize mutated the PacketOut buffer")
	}
	if len(first) != len(second) {
		t.Fatalf("finalized packet length changed between sends: %d != %d", len(first), len(second))
	}
	if binary.LittleEndian.Uint16(first[1:]) != uint16(len(first)) {
		t.Fatalf("first length field mismatch: got %d want %d", binary.LittleEndian.Uint16(first[1:]), len(first))
	}
	if binary.LittleEndian.Uint16(second[1:]) != uint16(len(second)) {
		t.Fatalf("second length field mismatch: got %d want %d", binary.LittleEndian.Uint16(second[1:]), len(second))
	}
	if first[len(first)-1] != 0xFE || second[len(second)-1] != 0xFE {
		t.Fatal("finalized packet is missing the 0xFE terminator")
	}
}

func TestFinalizeNoCounterOmitsAsdaCounterPadding(t *testing.T) {
	p := NewPacket(NpcVisiableNow)
	p.WriteBytes(make([]byte, 108))

	original := append([]byte(nil), p.buf...)
	withCounter := p.Finalize(crypt.LocaleStart)
	withoutCounter := p.FinalizeNoCounter(crypt.LocaleStart)

	if !bytes.Equal(p.buf, original) {
		t.Fatal("FinalizeNoCounter mutated the PacketOut buffer")
	}
	if len(withCounter)-len(withoutCounter) != 6 {
		t.Fatalf("counter padding length delta = %d, want 6", len(withCounter)-len(withoutCounter))
	}
	if withoutCounter[len(withoutCounter)-1] != 0xFE {
		t.Fatal("packet without counter is missing the 0xFE terminator")
	}
	if binary.LittleEndian.Uint16(withoutCounter[1:]) != uint16(len(withoutCounter)) {
		t.Fatalf("length field mismatch: got %d want %d", binary.LittleEndian.Uint16(withoutCounter[1:]), len(withoutCounter))
	}
}

func TestAsdaStringUsesReferenceEncoding(t *testing.T) {
	p := NewPacket(Opcode(0x1234))
	p.WriteAsdaStringLocale("مسعف", 8, crypt.LocaleAr)

	got := p.buf[10:18]
	want := []byte{0xE3, 0xD3, 0xDA, 0xDD, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded bytes = % X, want % X", got, want)
	}

	in := &PacketIn{Data: got}
	if decoded := in.ReadAsdaStringLocale(8, crypt.LocaleAr); decoded != "مسعف" {
		t.Fatalf("decoded = %q, want %q", decoded, "مسعف")
	}
}

func TestAsdaCStringUsesLocaleEncoding(t *testing.T) {
	p := NewPacket(Opcode(0x1234))
	p.WriteCStringLocale("مرحبا", 16, crypt.LocaleAr)

	got := p.buf[10:16]
	want := []byte{0xE3, 0xD1, 0xCD, 0xC8, 0xC7, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded cstring bytes = % X, want % X", got, want)
	}

	in := &PacketIn{Data: got}
	if decoded := in.ReadCStringLocale(16, crypt.LocaleAr); decoded != "مرحبا" {
		t.Fatalf("decoded cstring = %q, want %q", decoded, "مرحبا")
	}
}
