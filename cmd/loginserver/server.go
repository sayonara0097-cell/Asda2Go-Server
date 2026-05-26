package main

import (
	"log"
	"net"
	"sync"
)

var (
	clients   = map[uint32]*Client{}
	clientsMu sync.RWMutex
	nextID    uint32 = 1
)

func serve(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("[Login] Listening on %s", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[Login] accept: %v", err)
			continue
		}
		c := NewClient(conn, ServerKindLogin, router, nil)
		addClient(c)
		go func() {
			defer removeClient(c)
			defer c.Close()
			c.Run()
		}()
	}
}

func addClient(c *Client) {
	clientsMu.Lock()
	c.ID = nextID
	nextID++
	clients[c.ID] = c
	clientsMu.Unlock()
	log.Printf("[+] login client %d connected (%s)", c.ID, c.Conn.RemoteAddr())
}

func removeClient(c *Client) {
	releaseLoginAccountSession(c)
	clientsMu.Lock()
	delete(clients, c.ID)
	clientsMu.Unlock()
	log.Printf("[-] login client %d disconnected", c.ID)
}
