package types

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"asda2/shared/crypt"
	"asda2/shared/packet"
)

var packetTraceEnabled = envPacketTraceEnabled()

func SetPacketTrace(enabled bool) {
	packetTraceEnabled = enabled
}

func envPacketTraceEnabled() bool {
	for _, name := range []string{"ASDAGO_PACKET_TRACE", "ASDA2_PACKET_TRACE", "ASDAGO_PACKET_DEBUG", "ASDA2_PACKET_DEBUG"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
		switch value {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return false
}

// PacketDispatcher routes decoded packets to server-specific handlers.
type PacketDispatcher interface {
	Dispatch(*Client, *packet.PacketIn)
}

type AreaSender func(*Client, *packet.PacketOut)

// Client holds per-connection state, mirrors IRealmClient.
type Client struct {
	ID            uint32
	Conn          net.Conn
	ServerKind    ServerKind
	ConnectedAt   time.Time
	Channel       byte
	Locale        crypt.Locale
	Char          *Character
	Account       *Account
	IsTeleporting bool
	// AccountSessionToken owns the cross-process login/game lease for this
	// connection. It is released or heartbeated by the concrete server.
	AccountSessionToken string
	StopAccountSession  func()

	mu         sync.Mutex
	dispatcher PacketDispatcher
	areaSender AreaSender
}

func NewClient(conn net.Conn, kind ServerKind, dispatcher PacketDispatcher, areaSender AreaSender) *Client {
	return &Client{
		Conn:        conn,
		ServerKind:  kind,
		ConnectedAt: time.Now(),
		Locale:      crypt.LocaleAny,
		dispatcher:  dispatcher,
		areaSender:  areaSender,
	}
}

func (c *Client) Run() {
	c.readLoop()
}

func (c *Client) Close() {
	_ = c.Conn.Close()
}

func (c *Client) readLoop() {
	header := make([]byte, 3)
	for {
		if _, err := io.ReadFull(c.Conn, header); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				log.Printf("[Client] id=%d read header failed: %v", c.ID, err)
			}
			break
		}
		if header[0] != 0xFB {
			log.Printf("[Client] id=%d bad magic=0x%02X", c.ID, header[0])
			break
		}
		length := int(binary.LittleEndian.Uint16(header[1:]))
		if length < 10 || length > 2047 {
			log.Printf("[Client] id=%d invalid packet length=%d", c.ID, length)
			break
		}
		rest := make([]byte, length-3)
		if _, err := io.ReadFull(c.Conn, rest); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				log.Printf("[Client] id=%d read body failed: %v", c.ID, err)
			}
			break
		}
		buf := append(header[:3:3], rest...)

		if packetTraceEnabled {
			log.Printf("[Packet] client=%d raw[0:%d]=% X", c.ID, min(20, len(buf)), buf[:min(20, len(buf))])
		}

		if c.Locale == crypt.LocaleAny {
			probe := []byte{buf[3], buf[4]}
			crypt.XorData(probe, 0, 2, crypt.LocaleStart)
			switch probe[1] {
			case 0:
				c.Locale = crypt.LocaleStart
			case 116:
				c.Locale = crypt.LocaleAr
			case 228:
				c.Locale = crypt.LocaleRu
			default:
				probe2 := []byte{buf[3], buf[4]}
				crypt.XorData(probe2, 0, 2, crypt.LocaleTahadi)
				switch probe2[1] {
				case 0:
					c.Locale = crypt.LocaleTahadi
				case 103:
					c.Locale = crypt.LocaleLos
				default:
					c.Locale = crypt.LocaleStart
				}
			}
			log.Printf("[Client] id=%d locale=%d", c.ID, c.Locale)
		}

		crypt.XorData(buf, 3, length-4, c.Locale)
		if packetTraceEnabled {
			log.Printf("[Packet] client=%d dec[0:%d]=% X", c.ID, min(20, len(buf)), buf[:min(20, len(buf))])
		}

		opcode := packet.Opcode(binary.LittleEndian.Uint16(buf[8:]))
		var payload []byte
		if length > 11 {
			end := length - 1
			payload = make([]byte, end-10)
			copy(payload, buf[10:end])
		}

		if c.dispatcher != nil {
			c.dispatcher.Dispatch(c, &packet.PacketIn{Opcode: opcode, Data: payload})
		}
	}
}

func (c *Client) Send(p *packet.PacketOut) {
	data := p.Finalize(c.Locale)
	c.mu.Lock()
	_, _ = c.Conn.Write(data)
	c.mu.Unlock()
}

func (c *Client) SendNoCounter(p *packet.PacketOut) {
	data := p.FinalizeNoCounter(c.Locale)
	c.mu.Lock()
	_, _ = c.Conn.Write(data)
	c.mu.Unlock()
}

func (c *Client) SendToArea(p *packet.PacketOut) {
	if c.areaSender != nil {
		c.areaSender(c, p)
	}
}
