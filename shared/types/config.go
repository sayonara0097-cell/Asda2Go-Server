package types

import (
	"net"
	"os"
	"strconv"
)

type ServerKind string

const (
	ServerKindLogin ServerKind = "login"
	ServerKindGame  ServerKind = "game"
	ServerKindRelay ServerKind = "relay"
)

func EnvString(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func EnvInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func RemoteIP(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.IP == nil {
		return ""
	}
	return tcp.IP.String()
}
