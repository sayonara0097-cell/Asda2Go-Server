package main

import (
	"flag"
	"log"
	"os"

	"asda2/shared/crypt"
	"asda2/shared/db"
	"asda2/shared/relay"
)

var bridge = relay.NewBridge()

const loginBuildID = "stats-login-v1-20260525-2005"

func main() {
	bind := flag.String("bind", envString("ASDA2_LOGIN_ADDR", "0.0.0.0:5000"), "login server listen address")
	publicIP := flag.String("public-ip", envString("ASDA2_PUBLIC_IP", "127.0.0.1"), "public IP sent to clients for game server reconnect")
	gamePort := flag.Int("game-port", envInt("ASDA2_GAME_PORT", 5100), "public game server port sent to clients")
	channel := flag.Int("channel", envInt("ASDA2_CHANNEL", 0), "default game channel id")
	channels := flag.String("channels", envString("ASDA2_CHANNELS", ""), "optional channel endpoints, for example 0=127.0.0.1:5100,1=127.0.0.1:5101,2=127.0.0.1:5102")
	flag.Parse()
	if flag.NArg() > 0 {
		*bind = flag.Arg(0)
	}
	if *channel < 0 || *channel >= relay.GameChannelCount {
		log.Fatalf("invalid channel %d; use 0-%d", *channel, relay.GameChannelCount-1)
	}
	if *gamePort < 0 || *gamePort > 65535 {
		log.Fatalf("invalid game port %d; use 0-65535", *gamePort)
	}
	logLoginBuild()

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
	if err := db.InitGuildDB(); err != nil {
		log.Fatalf("[GuildDB] %v", err)
	}
	if _, err := db.InitItemTemplateCache(); err != nil {
		log.Fatalf("[Items] %v", err)
	}

	channelSpec := *channels
	if channelSpec == "" {
		channelSpec = relay.DefaultGameChannelSpec(*publicIP, uint16(*gamePort))
	}
	bridge.ConfigureChannels(relay.ChannelEndpoint{Channel: byte(*channel), IP: *publicIP, Port: uint16(*gamePort)}, channelSpec)
	registerHandlers()
	log.Printf("[Login] Starting on %s", *bind)
	if err := serve(*bind); err != nil {
		log.Fatal(err)
	}
}

func logLoginBuild() {
	exe, err := os.Executable()
	if err != nil {
		exe = "unknown:" + err.Error()
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "unknown:" + err.Error()
	}
	log.Printf("[LoginBuild] %s exe=%s cwd=%s", loginBuildID, exe, cwd)
}
