package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"asda2/shared/db"
	"asda2/shared/relay"
)

const gmSessionTTL = 12 * time.Hour

type gmSession struct {
	Token     string    `json:"token"`
	AccountID uint32    `json:"accountId"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type gmSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]gmSession
}

var gmSessions = &gmSessionStore{sessions: make(map[string]gmSession)}

type gmLoginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type gmLoginResponse struct {
	OK        bool            `json:"ok"`
	Session   gmSession       `json:"session"`
	Commands  []gmCommandInfo `json:"commands"`
	Message   string          `json:"message,omitempty"`
	RoleGroup string          `json:"roleGroup,omitempty"`
}

type gmCommandInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Args        []string `json:"args,omitempty"`
}

type gmCommandRequest struct {
	Action         string            `json:"action"`
	TargetServerID string            `json:"targetServerId,omitempty"`
	Args           map[string]string `json:"args,omitempty"`
}

type gmCommandResponse struct {
	Accepted bool            `json:"accepted"`
	Sent     int             `json:"sent"`
	Failed   int             `json:"failed"`
	Command  relay.GMCommand `json:"command"`
	Errors   []string        `json:"errors,omitempty"`
}

var gmCommandList = []gmCommandInfo{
	{Name: "online_players", Description: "List online players reported by connected game servers"},
	{Name: "announce", Description: "Broadcast a world announcement to connected game servers", Args: []string{"message"}},
	{Name: "teleport", Description: "Teleport an online character by local map coordinates", Args: []string{"character", "map", "x", "y"}},
	{Name: "add_exp", Description: "Add EXP to an online character and run level-up checks", Args: []string{"character", "amount"}},
	{Name: "add_gold", Description: "Add gold to an online character", Args: []string{"character", "amount"}},
	{Name: "give_item", Description: "Give an item to an online character", Args: []string{"character", "itemId", "amount"}},
	{Name: "set_level", Description: "Set an online character level and reset current EXP", Args: []string{"character", "level"}},
	{Name: "set_profession", Description: "Set an online character Asda2 class and real profession level", Args: []string{"character", "class", "level"}},
	{Name: "set_speed", Description: "Set an online character movement speed multiplier", Args: []string{"character", "multiplier"}},
	{Name: "heal_player", Description: "Heal or resurrect an online character", Args: []string{"character", "amount"}},
	{Name: "damage_player", Description: "Damage an online character and trigger death if HP reaches zero", Args: []string{"character", "amount"}},
	{Name: "kill_monster", Description: "Queue a monster-kill command for the game server NPC runtime", Args: []string{"monsterSessionId or monsterId"}},
	{Name: "summon_monster", Description: "Queue a monster-summon command for the game server NPC runtime", Args: []string{"monsterId", "map", "x", "y"}},
	{Name: "summon_monster_near_player", Description: "Summon a monster next to an online character", Args: []string{"character", "monsterId", "distance"}},
	{Name: "reload_monster_spawns", Description: "Reload DB-backed monster spawns on the target game server"},
	{Name: "send_packet", Description: "Send a raw server-to-client packet to an online character", Args: []string{"character", "opcode", "payloadHex"}},
}

func registerGMHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/gm/login", handleGMLogin)
	mux.HandleFunc("/gm/commands", requireGM(handleGMCommands))
	mux.HandleFunc("/gm/announce", requireGM(handleGMAnnounce))
	mux.HandleFunc("/gm/command", requireGM(handleGMCommand))
	mux.HandleFunc("/gm/gameservers", requireGM(handleGMGameServers))
	mux.HandleFunc("/gm/items", requireGM(handleGMItems))
	mux.HandleFunc("/gm/players", requireGM(handleGMPlayers))
}

func handleGMLogin(w http.ResponseWriter, r *http.Request) {
	if !allowPost(w, r) {
		return
	}
	defer r.Body.Close()

	var req gmLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and password are required"})
		return
	}

	account, err := db.GetAccountByName(name)
	if err != nil {
		log.Printf("[GM] login DB error for %q: %v", name, err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if account == nil || account.Password != req.Password || !account.IsActive {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if !isGMRole(account.RoleGroup) {
		writeJSON(w, http.StatusForbidden, gmLoginResponse{
			OK:        false,
			Message:   "account is not a GM",
			RoleGroup: account.RoleGroup,
		})
		return
	}

	session, err := gmSessions.create(uint32(account.AccountID), account.Name, account.RoleGroup)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[GM] %q authenticated as %s", account.Name, account.RoleGroup)
	writeJSON(w, http.StatusOK, gmLoginResponse{OK: true, Session: session, Commands: gmCommandList})
}

func handleGMCommands(w http.ResponseWriter, r *http.Request, session gmSession) {
	if !allowGet(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, gmLoginResponse{OK: true, Session: session, Commands: gmCommandList})
}

func handleGMGameServers(w http.ResponseWriter, r *http.Request, _ gmSession) {
	if !allowGet(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, gameServers.snapshot())
}

func handleGMPlayers(w http.ResponseWriter, r *http.Request, _ gmSession) {
	if !allowGet(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, gameServers.playersSnapshot())
}

func handleGMItems(w http.ResponseWriter, r *http.Request, _ gmSession) {
	if !allowGet(w, r) {
		return
	}
	items, err := db.LoadItemTemplates()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func handleGMAnnounce(w http.ResponseWriter, r *http.Request, session gmSession) {
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

	log.Printf("[GM] %s broadcast announcement to %d game servers: %q", session.Name, sent, message)
	writeJSON(w, http.StatusOK, response)
}

func handleGMCommand(w http.ResponseWriter, r *http.Request, session gmSession) {
	if !allowPost(w, r) {
		return
	}
	defer r.Body.Close()

	var req gmCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action is required"})
		return
	}
	if req.Args == nil {
		req.Args = make(map[string]string)
	}

	cmd := relay.GMCommand{
		Action:         action,
		Args:           req.Args,
		TargetServerID: strings.TrimSpace(req.TargetServerID),
		RequestedBy:    session.Name,
		AccountID:      session.AccountID,
		CreatedAt:      time.Now(),
	}
	env, err := relay.NewEnvelope(relay.MessageGMCommand, cmd)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sent, errs := gameServers.sendTo(cmd.TargetServerID, env)
	response := gmCommandResponse{
		Accepted: sent > 0 && len(errs) == 0,
		Sent:     sent,
		Failed:   len(errs),
		Command:  cmd,
	}
	for _, err := range errs {
		response.Errors = append(response.Errors, err.Error())
	}
	log.Printf("[GM] %s queued %s command to %d game servers target=%q", session.Name, action, sent, cmd.TargetServerID)
	writeJSON(w, http.StatusOK, response)
}

type gmHandler func(http.ResponseWriter, *http.Request, gmSession)

func requireGM(next gmHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := gmSessions.fromRequest(r)
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or expired GM session"})
			return
		}
		next(w, r, session)
	}
}

func (s *gmSessionStore) create(accountID uint32, name string, role string) (gmSession, error) {
	token, err := newGMToken()
	if err != nil {
		return gmSession{}, err
	}
	session := gmSession{
		Token:     token,
		AccountID: accountID,
		Name:      name,
		Role:      role,
		ExpiresAt: time.Now().Add(gmSessionTTL),
	}
	s.mu.Lock()
	s.sessions[token] = session
	s.cleanupLocked(time.Now())
	s.mu.Unlock()
	return session, nil
}

func (s *gmSessionStore) fromRequest(r *http.Request) (gmSession, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return gmSession{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if token == "" {
		return gmSession{}, false
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[token]
	if !ok || now.After(session.ExpiresAt) {
		delete(s.sessions, token)
		return gmSession{}, false
	}
	return session, true
}

func (s *gmSessionStore) cleanupLocked(now time.Time) {
	for token, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, token)
		}
	}
}

func newGMToken() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func isGMRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return false
	}
	for _, allowed := range gmRoleNames() {
		if role == allowed {
			return true
		}
	}
	return false
}

func gmRoleNames() []string {
	configured := strings.TrimSpace(os.Getenv("ASDA2_GM_ROLES"))
	if configured == "" {
		return []string{"gm", "developer", "admin", "owner", "gamemaster"}
	}
	parts := strings.Split(configured, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		role := strings.ToLower(strings.TrimSpace(part))
		if role != "" {
			out = append(out, role)
		}
	}
	return out
}
