package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"asda2/shared/crypt"
	"asda2/shared/db"
	"asda2/shared/relay"
	"asda2/shared/types"
)

var bridge = relay.NewBridge()

const gameBuildID = "npcserver-visibility-throttle-v1-20260525"

var (
	gamePublicIP                      string
	gamePublicPort                    uint16
	gameChannel                       byte
	disableNpcVisibility              bool
	npcServerMonsterVisibilityEnabled bool
)

func main() {
	defaultChannel := executableDefaultChannel("gameserver")
	defaultChannel = envInt("ASDA2_CHANNEL", defaultChannel)
	defaultBind := relay.DefaultGameServerBind(byte(defaultChannel))
	defaultPort := relay.DefaultGameServerPort(byte(defaultChannel))

	bind := flag.String("bind", envString("ASDA2_GAME_ADDR", defaultBind), "game server listen address")
	publicIP := flag.String("public-ip", envString("ASDA2_PUBLIC_IP", "127.0.0.1"), "public IP registered for this game channel")
	publicPort := flag.Int("public-port", envInt("ASDA2_GAME_PORT", defaultPort), "public game server port registered for this channel")
	channel := flag.Int("channel", defaultChannel, "game channel id for this process")
	relayAddr := flag.String("relay", envString("ASDA2_RELAY_ADDR", "127.0.0.1:5200"), "relay server address for future cross-channel features")
	npcServerAddr := flag.String("npc-server", envString("ASDA2_NPCSERVER_ADDR", ""), "NpcServer HTTP base URL; defaults to a per-channel local NpcServer")
	serverID := flag.String("server-id", envString("ASDA2_SERVER_ID", ""), "stable relay id for this game server")
	debugVisibility := flag.Bool("debug-visibility", envVisibilityDebugEnabled(), "log character visibility show/hide and movement recipients")
	packetTrace := flag.Bool("packet-trace", envPacketTraceFlagEnabled(), "log raw/decrypted packet headers for protocol diagnosis")
	disableNPCs := flag.Bool("disable-npc-visibility", envNPCVisibilityDisabled(), "temporarily skip NpcVisiableNow packets for diagnosis")
	npcServerMonsters := flag.Bool("npcserver-monsters", envNpcServerMonsterVisibilityEnabled(), "send experimental NpcServer monster visibility packets")
	flag.Parse()
	if flag.NArg() > 0 {
		*bind = flag.Arg(0)
	}
	if *channel < 0 || *channel >= relay.GameChannelCount {
		log.Fatalf("invalid channel %d; use 0-%d", *channel, relay.GameChannelCount-1)
	}
	if flagIsUnset("bind") && strings.TrimSpace(os.Getenv("ASDA2_GAME_ADDR")) == "" {
		*bind = relay.DefaultGameServerBind(byte(*channel))
	}
	if flagIsUnset("public-port") && strings.TrimSpace(os.Getenv("ASDA2_GAME_PORT")) == "" {
		*publicPort = relay.DefaultGameServerPort(byte(*channel))
	}
	if *publicPort < 0 || *publicPort > 65535 {
		log.Fatalf("invalid public port %d; use 0-65535", *publicPort)
	}
	if *serverID == "" {
		*serverID = fmt.Sprintf("game-channel-%d", *channel)
	}
	gameServerRelayID = *serverID
	gamePublicIP = *publicIP
	gamePublicPort = uint16(*publicPort)
	gameChannel = byte(*channel)
	disableNpcVisibility = *disableNPCs
	npcServerMonsterVisibilityEnabled = *npcServerMonsters
	types.SetPacketTrace(*packetTrace)
	npcServerURL := strings.TrimSpace(*npcServerAddr)
	if npcServerURL == "" {
		npcServerURL = relay.DefaultNpcServerURL(gameChannel)
	}
	npcServerClient = newHTTPNpcServerClient(npcServerURL)
	setVisibilityDebug(*debugVisibility)
	logPacketTraceState(*packetTrace)
	if disableNpcVisibility {
		log.Printf("[NPC] visibility disabled")
	}
	if npcServerClient != nil {
		log.Printf("[NpcServer] remote NPC/monster visibility enabled url=%s channel=%d", npcServerURL, gameChannel)
		log.Printf("[NpcServer] remote monster spawn packets %s", enabledDisabled(npcServerMonsterVisibilityEnabled))
	}
	logGameBuild()

	crypt.InitKeys()
	if err := db.Init(db.DefaultDB); err != nil {
		log.Fatalf("[DB] %v", err)
	}
	if err := relay.InitBridgeDB(); err != nil {
		log.Fatalf("[RelayDB] %v", err)
	}
	if err := db.InitBaseStatsDB(); err != nil {
		log.Fatalf("[BaseStatsDB] %v", err)
	}
	if err := db.InitSkillDB(); err != nil {
		log.Fatalf("[SkillDB] %v", err)
	}
	if err := db.InitTeleportDB(); err != nil {
		log.Fatalf("[TeleportDB] %v", err)
	}
	if err := db.InitPortalDB(); err != nil {
		log.Fatalf("[PortalDB] %v", err)
	}
	if err := db.InitWeatherDB(); err != nil {
		log.Fatalf("[WeatherDB] %v", err)
	}
	if err := db.InitNpcVendorDB(); err != nil {
		log.Fatalf("[NpcVendorDB] %v", err)
	}
	if err := db.InitGuildDB(); err != nil {
		log.Fatalf("[GuildDB] %v", err)
	}
	if npcServerClient == nil {
		if err := db.InitMonsterDB(); err != nil {
			log.Fatalf("[MonsterDB] %v", err)
		}
		if err := db.InitNpcDB(); err != nil {
			log.Fatalf("[NpcDB] %v", err)
		}
	}
	if err := initItemRuntime(); err != nil {
		log.Fatalf("[Items] %v", err)
	}
	if err := initCraftingRuntime(); err != nil {
		log.Fatalf("[Crafting] %v", err)
	}
	if err := initSkillRuntime(); err != nil {
		log.Fatalf("[Skill] %v", err)
	}
	if err := initNpcVendorRuntime(); err != nil {
		log.Fatalf("[NpcVendor] %v", err)
	}

	endpoint := relay.ChannelEndpoint{Channel: byte(*channel), IP: *publicIP, Port: uint16(*publicPort)}
	bridge.ConfigureChannels(endpoint, "")
	relay.StartChannelHeartbeat(endpoint, countGamePlayersOnChannel)
	stopRelay := relay.StartGameServerClientWithOutboxAndPlayers(*relayAddr, relay.GameServerRegistration{
		ServerID:  *serverID,
		Channel:   byte(*channel),
		IP:        *publicIP,
		Port:      uint16(*publicPort),
		StartedAt: time.Now(),
	}, countGamePlayersOnChannel, listGamePlayersOnChannel, relayOutbox, handleRelayMessage)
	defer stopRelay()

	initWorld()
	if err := initPortalRuntime(); err != nil {
		log.Fatalf("[Portal] %v", err)
	}
	if err := initWeatherRuntime(gameChannel); err != nil {
		log.Fatalf("[Weather] %v", err)
	}
	if npcServerClient == nil {
		if err := initMonsterRuntime(gameChannel); err != nil {
			log.Fatalf("[Monster] %v", err)
		}
		if err := initNpcRuntime(gameChannel); err != nil {
			log.Fatalf("[NPC] %v", err)
		}
	} else {
		log.Printf("[NPC] local NPC and monster runtimes skipped; using NpcServer")
	}
	registerHandlers()
	startWeatherSyncLoop()
	log.Printf("[Game] Starting channel %d on %s build=%s", *channel, *bind, gameBuildID)
	if err := serve(*bind); err != nil {
		log.Fatal(err)
	}
}

func executableDefaultChannel(serverName string) int {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}
	if channel, ok := relay.ChannelFromExecutableName(exe, serverName); ok {
		return int(channel)
	}
	return 0
}

func flagIsUnset(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return !set
}

func envPacketTraceFlagEnabled() bool {
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

func envNpcServerMonsterVisibilityEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ASDA2_NPCSERVER_MONSTERS")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func enabledDisabled(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func logPacketTraceState(enabled bool) {
	if enabled {
		log.Printf("[Packet] trace enabled")
		return
	}
	log.Printf("[Packet] trace disabled")
}

func logGameBuild() {
	exe, err := os.Executable()
	if err != nil {
		exe = "unknown:" + err.Error()
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown:" + err.Error()
	}
	log.Printf("[GameBuild] %s exe=%s cwd=%s", gameBuildID, exe, cwd)
}
