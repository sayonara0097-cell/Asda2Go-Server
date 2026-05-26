package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"asda2/shared/db"
	"asda2/shared/relay"
)

type BridgeStatus struct {
	OK          bool                     `json:"ok"`
	Time        time.Time                `json:"time"`
	Database    string                   `json:"database"`
	GameServers []relay.GameServerStatus `json:"gameServers,omitempty"`
	Channels    []relay.ChannelStatus    `json:"channels,omitempty"`
	Handoffs    []relay.HandoffStatus    `json:"handoffs,omitempty"`
	Errors      map[string]string        `json:"errors,omitempty"`
}

func serveStatusHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("/gameservers", handleGameServers)
	mux.HandleFunc("/channels", handleChannels)
	mux.HandleFunc("/handoffs", handleHandoffs)
	registerGMHTTP(mux)
	log.Printf("[RelayHTTP] Listening on http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if !allowGet(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprint(w, strings.TrimSpace(`
AsdaGo Relay Server

GET /health    database health
GET /status    database, channels, and pending handoffs
GET /gameservers  connected game server relay clients
POST /gm/login  authenticate a GM tool session
GET /gm/commands  available GM tool commands
GET /gm/players  online players visible to GM tool
GET /channels  channel registry from ServerChannel
GET /handoffs  pending login-to-game handoffs
`)+"\n")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	status := BridgeStatus{OK: true, Time: time.Now(), Database: "ok"}
	code := http.StatusOK
	if err := db.DB.Ping(); err != nil {
		status.OK = false
		status.Database = err.Error()
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	status := BridgeStatus{OK: true, Time: time.Now(), Database: "ok"}
	if err := db.DB.Ping(); err != nil {
		status.OK = false
		status.Database = err.Error()
	}
	status.GameServers = gameServers.snapshot()
	channels, err := relay.LoadChannelStatuses(relay.ChannelHeartbeatMaxAge)
	if err != nil {
		status.OK = false
		addStatusError(&status, "channels", err)
	} else {
		status.Channels = channels
	}
	handoffs, err := relay.LoadPendingHandoffStatuses()
	if err != nil {
		status.OK = false
		addStatusError(&status, "handoffs", err)
	} else {
		status.Handoffs = handoffs
	}
	code := http.StatusOK
	if !status.OK {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}

func handleGameServers(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, gameServers.snapshot())
}

type announceRequest struct {
	Message string `json:"message"`
}

type announceResponse struct {
	Sent         int                     `json:"sent"`
	Failed       int                     `json:"failed"`
	Announcement relay.WorldAnnouncement `json:"announcement"`
	Errors       []string                `json:"errors,omitempty"`
}

func handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if !allowPost(w, r) {
		return
	}
	defer r.Body.Close()

	var req announceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	announcement := relay.WorldAnnouncement{Message: message, SentAt: time.Now()}
	env, err := relay.NewEnvelope(relay.MessageWorldAnnouncement, announcement)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sent, errs := gameServers.broadcast(env)
	response := announceResponse{
		Sent:         sent,
		Failed:       len(errs),
		Announcement: announcement,
	}
	for _, err := range errs {
		response.Errors = append(response.Errors, err.Error())
	}

	log.Printf("[RelayHTTP] world announcement sent to %d game servers: %q", sent, message)
	writeJSON(w, http.StatusOK, response)
}

func handleChannels(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	channels, err := relay.LoadChannelStatuses(relay.ChannelHeartbeatMaxAge)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

func handleHandoffs(w http.ResponseWriter, r *http.Request) {
	if !allowGet(w, r) {
		return
	}
	handoffs, err := relay.LoadPendingHandoffStatuses()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, handoffs)
}

func addStatusError(status *BridgeStatus, key string, err error) {
	if status.Errors == nil {
		status.Errors = make(map[string]string)
	}
	status.Errors[key] = err.Error()
}

func allowGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func allowPost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodPost {
		return true
	}
	w.Header().Set("Allow", http.MethodPost)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		log.Printf("[RelayHTTP] failed to write JSON response: %v", err)
	}
}
