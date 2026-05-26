package relay

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	GameChannelCount          = 3
	DefaultGameServerBasePort = 5100
	DefaultNpcServerBasePort  = 5300
)

func ValidGameChannel(channel byte) bool {
	return int(channel) < GameChannelCount
}

func NormalizeGameChannel(channel byte) byte {
	if ValidGameChannel(channel) {
		return channel
	}
	return 0
}

func DefaultGameChannelEndpoints(publicIP string, basePort uint16) []ChannelEndpoint {
	if strings.TrimSpace(publicIP) == "" {
		publicIP = "127.0.0.1"
	}

	endpoints := make([]ChannelEndpoint, 0, GameChannelCount)
	for channel := 0; channel < GameChannelCount; channel++ {
		port := int(basePort) + channel
		if port > 65535 {
			port = int(basePort)
		}
		endpoints = append(endpoints, ChannelEndpoint{
			Channel: byte(channel),
			IP:      publicIP,
			Port:    uint16(port),
		})
	}
	return endpoints
}

func DefaultGameChannelSpec(publicIP string, basePort uint16) string {
	endpoints := DefaultGameChannelEndpoints(publicIP, basePort)
	parts := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		parts = append(parts, fmt.Sprintf("%d=%s:%d", endpoint.Channel, endpoint.IP, endpoint.Port))
	}
	return strings.Join(parts, ",")
}

func DefaultGameServerPort(channel byte) int {
	return DefaultGameServerBasePort + int(NormalizeGameChannel(channel))
}

func DefaultGameServerBind(channel byte) string {
	return fmt.Sprintf("0.0.0.0:%d", DefaultGameServerPort(channel))
}

func DefaultNpcServerBind(channel byte) string {
	return fmt.Sprintf("127.0.0.1:%d", DefaultNpcServerBasePort+int(NormalizeGameChannel(channel)))
}

func DefaultNpcServerURL(channel byte) string {
	return "http://" + DefaultNpcServerBind(channel)
}

func ChannelFromExecutableName(executablePath string, serverName string) (byte, bool) {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(executablePath), filepath.Ext(executablePath)))
	serverName = strings.ToLower(strings.TrimSpace(serverName))
	if serverName == "" {
		return 0, false
	}

	for _, marker := range []string{"-channel", "_channel", "-ch", "_ch"} {
		prefix := serverName + marker
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if channel, ok := parseExecutableChannelSuffix(strings.TrimPrefix(name, prefix)); ok {
			return channel, true
		}
	}
	return 0, false
}

func parseExecutableChannelSuffix(suffix string) (byte, bool) {
	suffix = strings.TrimLeft(suffix, "-_ ")
	if suffix == "" {
		return 0, false
	}
	end := 0
	for end < len(suffix) && suffix[end] >= '0' && suffix[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	value, err := strconv.Atoi(suffix[:end])
	if err != nil || value < 0 || value >= GameChannelCount {
		return 0, false
	}
	return byte(value), true
}
