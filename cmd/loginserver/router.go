package main

import "log"

type HandlerFunc func(c *Client, p *PacketIn)

var router = &packetRouter{handlers: make(map[Opcode]HandlerFunc)}

type packetRouter struct {
	handlers map[Opcode]HandlerFunc
}

func (r *packetRouter) register(op Opcode, fn HandlerFunc) {
	r.handlers[op] = fn
}

func (r *packetRouter) Dispatch(c *Client, p *PacketIn) {
	fn, ok := r.handlers[p.Opcode]
	if !ok {
		log.Printf("unhandled opcode 0x%04X (%d)", uint16(p.Opcode), p.Opcode)
		return
	}
	fn(c, p)
}
