package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"asda2/shared/db"
	"asda2/shared/npcruntime"
	"asda2/shared/relay"
	"asda2/shared/types"
	"asda2/shared/worlddata"
)

const buildID = "channels-v1-20260525-1430"

func main() {
	defaultChannel := executableDefaultChannel("npcserver")
	defaultChannel = envInt("ASDA2_CHANNEL", defaultChannel)

	bind := flag.String("bind", envString("ASDA2_NPCSERVER_BIND", ""), "npc server HTTP listen address; defaults to a per-channel local port")
	channel := flag.Int("channel", defaultChannel, "game channel id for this NPC process")
	flag.Parse()

	if *channel < 0 || *channel >= relay.GameChannelCount {
		log.Fatalf("invalid channel %d; use 0-%d", *channel, relay.GameChannelCount-1)
	}
	if strings.TrimSpace(*bind) == "" {
		*bind = relay.DefaultNpcServerBind(byte(*channel))
	}

	npcTemplates, npcSpawns, monsterTemplates, monsterSpawns, source, err := loadWorldData(byte(*channel))
	if err != nil {
		log.Fatalf("[NpcServer] %v", err)
	}
	runtime := npcruntime.New(byte(*channel), npcTemplates, npcSpawns, monsterTemplates, monsterSpawns)

	mux := http.NewServeMux()
	mux.HandleFunc("/status", handleStatus(runtime, byte(*channel), source))
	mux.HandleFunc("/visible", handleVisible(runtime, byte(*channel)))
	mux.HandleFunc("/player/sync", handlePlayerSync(runtime, byte(*channel)))
	mux.HandleFunc("/player/leave", handlePlayerLeave(runtime))

	ln, err := net.Listen("tcp", *bind)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[NpcServer] build=%s channel=%d listening=http://%s source=%s npcs=%d monsters=%d",
		buildID, *channel, ln.Addr().String(), source, runtime.NpcCount(), runtime.MonsterCount())

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
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

func loadWorldData(channel byte) ([]types.NpcTemplateRow, []types.NpcSpawnRow, []types.MonsterTemplateRow, []types.MonsterSpawnRow, string, error) {
	npcTemplates, npcSpawns, npcSource, npcsOK, err := worlddata.LoadNpcs("", channel)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	monsterTemplates, monsterSpawns, monsterSource, monstersOK, err := worlddata.LoadMonsters("", channel)
	if err != nil {
		return nil, nil, nil, nil, "", err
	}
	if npcsOK && monstersOK {
		return npcTemplates, npcSpawns, monsterTemplates, monsterSpawns, npcSource + " monsters=" + monsterSource, nil
	}

	if err := db.Init(db.DefaultDB); err != nil {
		return nil, nil, nil, nil, "", fmt.Errorf("init db: %w", err)
	}
	if !npcsOK {
		if err := db.InitNpcDB(); err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("init npc db: %w", err)
		}
		npcTemplates, err = db.LoadNpcTemplates()
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("load npc templates: %w", err)
		}
		npcSpawns, err = db.LoadNpcSpawns(channel)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("load npc spawns: %w", err)
		}
		npcSource = "db"
	}
	if !monstersOK {
		if err := db.InitMonsterDB(); err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("init monster db: %w", err)
		}
		monsterTemplates, err = db.LoadMonsterTemplates()
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("load monster templates: %w", err)
		}
		monsterSpawns, err = db.LoadMonsterSpawns(channel)
		if err != nil {
			return nil, nil, nil, nil, "", fmt.Errorf("load monster spawns: %w", err)
		}
		monsterSource = "db"
	}
	return npcTemplates, npcSpawns, monsterTemplates, monsterSpawns, npcSource + " monsters=" + monsterSource, nil
}

func handleStatus(runtime *npcruntime.Runtime, channel byte, source string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"server":   "NpcServer",
			"build":    buildID,
			"channel":  channel,
			"source":   source,
			"npcs":     runtime.NpcCount(),
			"monsters": runtime.MonsterCount(),
			"players":  runtime.PlayerCount(),
		})
	}
}

func handleVisible(runtime *npcruntime.Runtime, channel byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		mapID := uint16(queryInt(query.Get("map"), 0))
		x := int16(queryInt(query.Get("x"), 0))
		y := int16(queryInt(query.Get("y"), 0))
		ch := int16(queryInt(query.Get("channel"), int(channel)))
		if ch != int16(channel) {
			http.Error(w, fmt.Sprintf("npcserver channel %d cannot serve channel %d", channel, ch), http.StatusConflict)
			return
		}
		writeJSON(w, runtime.VisibleAt(mapID, x, y, ch, queryBool(query.Get("force"))))
	}
}

func handlePlayerSync(runtime *npcruntime.Runtime, channel byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		player := readNpcPlayer(r)
		if player.Channel != int16(channel) {
			http.Error(w, fmt.Sprintf("npcserver channel %d cannot sync channel %d", channel, player.Channel), http.StatusConflict)
			return
		}
		ok := runtime.SyncPlayer(player)
		writeJSON(w, map[string]any{"ok": ok, "players": runtime.PlayerCount()})
	}
}

func handlePlayerLeave(runtime *npcruntime.Runtime) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		player := readNpcPlayer(r)
		ok := runtime.LeavePlayer(player.AccountID, player.SessionID)
		writeJSON(w, map[string]any{"ok": ok, "players": runtime.PlayerCount()})
	}
}

func readNpcPlayer(r *http.Request) npcruntime.Player {
	player := npcruntime.Player{}
	if r.Body != nil && r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&player)
	}
	query := r.URL.Query()
	if player.AccountID == 0 {
		player.AccountID = uint32(queryInt(query.Get("account"), 0))
	}
	if player.SessionID == 0 {
		player.SessionID = int16(queryInt(query.Get("session"), 0))
	}
	if player.Character == "" {
		player.Character = query.Get("name")
	}
	if player.MapID == 0 {
		player.MapID = uint16(queryInt(query.Get("map"), 0))
	}
	if player.X == 0 {
		player.X = int16(queryInt(query.Get("x"), 0))
	}
	if player.Y == 0 {
		player.Y = int16(queryInt(query.Get("y"), 0))
	}
	if player.Channel == 0 {
		player.Channel = int16(queryInt(query.Get("channel"), 0))
	}
	return player
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("[NpcServer] write json: %v", err)
	}
}

func queryInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envString(name string, fallback string) string {
	if value := strings.TrimSpace(getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenv(name string) string {
	return os.Getenv(name)
}
