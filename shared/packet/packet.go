package packet

import (
	"asda2/shared/crypt"
	"encoding/binary"
	"math"
	"sync/atomic"
)

// packetCounter increments with every outgoing packet (appended before XOR)
var packetCounter uint32

// xorKeyNum rotates 0-255, written as byte 3 of each outgoing packet
var xorKeyNum uint32

const packetMagic = uint32(7077887) // 0x006BFFFF written at bytes 4-7

// ---- Incoming ----

// PacketIn holds a decoded inbound packet, mirrors RealmPacketIn
type PacketIn struct {
	Opcode Opcode
	Data   []byte
	pos    int
}

func (p *PacketIn) ReadUint8() byte {
	b := p.Data[p.pos]
	p.pos++
	return b
}

func (p *PacketIn) ReadUint16() uint16 {
	v := binary.LittleEndian.Uint16(p.Data[p.pos:])
	p.pos += 2
	return v
}

func (p *PacketIn) ReadInt16() int16 { return int16(p.ReadUint16()) }

func (p *PacketIn) ReadUint32() uint32 {
	v := binary.LittleEndian.Uint32(p.Data[p.pos:])
	p.pos += 4
	return v
}

func (p *PacketIn) ReadInt32() int32 { return int32(p.ReadUint32()) }

func (p *PacketIn) ReadFloat32() float32 {
	return math.Float32frombits(p.ReadUint32())
}

// ReadAsdaString reads a fixed-length null-terminated string field.
// Mirrors RealmPacketIn.ReadAsdaString(int len, Locale.Start):
//
//	reads bytes until a null terminator, then seeks past the rest of the
//	field so that exactly maxLen bytes are consumed in total.
//	C# source: while((num = ReadByte()) != 0) byteList.Add(num);
//	           if(byteList.Count < len) Position += len - byteList.Count - 1;
func (p *PacketIn) ReadAsdaString(maxLen int) string {
	return p.ReadAsdaStringLocale(maxLen, crypt.LocaleStart)
}

// ReadAsdaStringLocale reads an Asda2 fixed string using the reference
// Asda2EncodingHelper tables for the requested text locale.
func (p *PacketIn) ReadAsdaStringLocale(maxLen int, locale crypt.Locale) string {
	start := p.pos
	var buf []byte
	for i := 0; i < maxLen; i++ {
		if p.pos >= len(p.Data) {
			break
		}
		b := p.Data[p.pos]
		p.pos++
		if b == 0 {
			// Seek past the remaining bytes of the fixed-width field.
			// consumed includes the null byte itself.
			consumed := p.pos - start
			if remaining := maxLen - consumed; remaining > 0 {
				p.pos += remaining
			}
			return decodeAsdaString(buf, locale)
		}
		buf = append(buf, b)
	}
	return decodeAsdaString(buf, locale)
}

func (p *PacketIn) ReadCString(maxLen int) string {
	var buf []byte
	for len(buf) < maxLen && p.pos < len(p.Data) {
		b := p.Data[p.pos]
		p.pos++
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return string(buf)
}

func (p *PacketIn) ReadCStringLocale(maxLen int, locale crypt.Locale) string {
	var buf []byte
	for len(buf) < maxLen && p.pos < len(p.Data) {
		b := p.Data[p.pos]
		p.pos++
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return decodeAsdaString(buf, locale)
}

func (p *PacketIn) Skip(n int) { p.pos += n }

func (p *PacketIn) Seek(pos int) { p.pos = pos }

func (p *PacketIn) Remaining() int { return len(p.Data) - p.pos }

// ---- Outgoing ----

// PacketOut builds an outbound packet, mirrors RealmPacketOut
type PacketOut struct {
	buf []byte
}

// NewPacket creates an outgoing packet with the Asda2 header.
// Layout before finalize:
//
//	[0]     0xFB
//	[1-2]   length placeholder
//	[3]     XOR key number
//	[4-7]   magic constant
//	[8-9]   opcode
//	[10+]   payload (caller writes here)
func NewPacket(op Opcode) *PacketOut {
	p := &PacketOut{buf: make([]byte, 0, 64)}
	p.writeByte(0xFB)
	p.writeUint16(0) // length filled in Finalize
	keyNum := byte(atomic.AddUint32(&xorKeyNum, 1) - 1)
	p.writeByte(keyNum)
	p.writeUint32(packetMagic)
	p.writeUint16(uint16(op))
	return p
}

func (p *PacketOut) WriteUint8(v byte)    { p.writeByte(v) }
func (p *PacketOut) WriteInt16(v int16)   { p.writeUint16(uint16(v)) }
func (p *PacketOut) WriteUint16(v uint16) { p.writeUint16(v) }
func (p *PacketOut) WriteInt32(v int32)   { p.writeUint32(uint32(v)) }
func (p *PacketOut) WriteUint32(v uint32) { p.writeUint32(v) }
func (p *PacketOut) WriteFloat32(v float32) {
	p.writeUint32(math.Float32bits(v))
}
func (p *PacketOut) WriteInt64(v int64)  { p.writeUint64(uint64(v)) }
func (p *PacketOut) WriteBytes(b []byte) { p.buf = append(p.buf, b...) }

// WriteAsdaString writes s into a fixed-length field of maxLen bytes (null-padded).
// Mirrors RealmPacketOut.WriteAsdaString / WriteFixedAsciiString with Locale.Start.
func (p *PacketOut) WriteAsdaString(s string, maxLen int) {
	p.WriteAsdaStringLocale(s, maxLen, crypt.LocaleStart)
}

// WriteAsdaStringLocale writes an Asda2 fixed string using the reference
// single-byte character tables instead of UTF-8.
func (p *PacketOut) WriteAsdaStringLocale(s string, maxLen int, locale crypt.Locale) {
	if s == "" {
		for i := 0; i < maxLen; i++ {
			p.buf = append(p.buf, 0)
		}
		return
	}
	b := encodeAsdaString(s, locale)
	if len(b) >= maxLen {
		b = b[:maxLen]
	}
	p.buf = append(p.buf, b...)
	for i := len(b); i < maxLen; i++ {
		p.buf = append(p.buf, 0)
	}
}

func (p *PacketOut) WriteCString(s string, maxLen int) {
	b := []byte(s)
	if len(b) > maxLen {
		b = b[:maxLen]
	}
	p.buf = append(p.buf, b...)
	p.buf = append(p.buf, 0)
}

func (p *PacketOut) WriteCStringLocale(s string, maxLen int, locale crypt.Locale) {
	b := encodeAsdaString(s, locale)
	if len(b) > maxLen {
		b = b[:maxLen]
	}
	p.buf = append(p.buf, b...)
	p.buf = append(p.buf, 0)
}

func (p *PacketOut) writeByte(v byte) { p.buf = append(p.buf, v) }
func (p *PacketOut) writeUint16(v uint16) {
	p.buf = append(p.buf, byte(v), byte(v>>8))
}
func (p *PacketOut) writeUint32(v uint32) {
	p.buf = append(p.buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
func (p *PacketOut) writeUint64(v uint64) {
	p.buf = append(p.buf,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
		byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

// Finalize appends counter+padding, XOR-encrypts from offset 3,
// appends 0xFE, and fills in the length field. Mirrors FinalizeAsda(true).
func (p *PacketOut) Finalize(locale crypt.Locale) []byte {
	return p.finalize(locale, true)
}

// FinalizeNoCounter XOR-encrypts and appends the packet terminator without the
// Asda2 counter padding. Mirrors RealmPacketOut.FinalizeAsda(false).
func (p *PacketOut) FinalizeNoCounter(locale crypt.Locale) []byte {
	return p.finalize(locale, false)
}

func (p *PacketOut) finalize(locale crypt.Locale, addCounter bool) []byte {
	buf := make([]byte, len(p.buf), len(p.buf)+7)
	copy(buf, p.buf)

	if addCounter {
		// Append packet counter (4B) + padding (2B) before XOR.
		cnt := atomic.AddUint32(&packetCounter, 1) - 1
		buf = appendUint32(buf, cnt)
		buf = appendUint16(buf, 0)
	}

	// XOR encrypt from offset 3 (key num byte) onward
	crypt.XorData(buf, 3, len(buf)-3, locale)

	// End-of-packet marker (after XOR, not encrypted)
	buf = append(buf, 0xFE)

	// Fill length field at bytes 1-2
	binary.LittleEndian.PutUint16(buf[1:], uint16(len(buf)))

	return buf
}

func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

func appendUint32(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
