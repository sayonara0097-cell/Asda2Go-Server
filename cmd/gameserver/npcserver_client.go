package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"asda2/shared/npcruntime"
	"asda2/shared/types"
)

var npcServerClient *httpNpcServerClient

const npcServerUnavailableLogEvery = 30 * time.Second

var npcServerLogState struct {
	sync.Mutex
	unavailable bool
	lastLog     time.Time
	suppressed  int
}

type httpNpcServerClient struct {
	baseURL string
	client  *http.Client
}

type npcServerVisibleResponse struct {
	MapID        uint16             `json:"mapId"`
	Channel      int16              `json:"channel"`
	NpcCount     int                `json:"npcCount"`
	MonsterCount int                `json:"monsterCount"`
	Npcs         []npcServerNPC     `json:"npcs"`
	Monsters     []npcServerMonster `json:"monsters"`
}

type npcServerVisibilitySnapshot struct {
	MapID        uint16
	Channel      int16
	NpcCount     int
	MonsterCount int
	Npcs         []*Npc
	Monsters     []*Monster
}

type npcServerNPC struct {
	SessionID       int16                    `json:"sessionId"`
	WorldEntityID   int32                    `json:"worldEntityId"`
	SpawnID         uint32                   `json:"spawnId"`
	EntryID         uint16                   `json:"entryId"`
	Name            string                   `json:"name"`
	Kind            byte                     `json:"kind"`
	ClassGroup      byte                     `json:"classGroup"`
	IsTrainer       bool                     `json:"isTrainer"`
	InteractionKind types.NpcInteractionKind `json:"interactionKind"`
	MapID           uint16                   `json:"mapId"`
	LocalX          int16                    `json:"x"`
	LocalY          int16                    `json:"y"`
	Channel         int16                    `json:"channel"`
}

type npcServerMonster struct {
	SessionID      int16   `json:"sessionId"`
	WorldEntityID  int32   `json:"worldEntityId"`
	SpawnID        uint32  `json:"spawnId"`
	EntryID        uint16  `json:"entryId"`
	Name           string  `json:"name"`
	Level          byte    `json:"level"`
	MapID          uint16  `json:"mapId"`
	LocalX         int16   `json:"x"`
	LocalY         int16   `json:"y"`
	Health         int32   `json:"health"`
	MaxHealth      int32   `json:"maxHealth"`
	MoveMS         int16   `json:"moveMs"`
	WalkSpeed      float64 `json:"walkSpeed"`
	RunSpeed       float64 `json:"runSpeed"`
	RespawnSeconds int     `json:"respawnSeconds"`
	MinDamage      float64 `json:"minDamage"`
	MaxDamage      float64 `json:"maxDamage"`
	BaseAttackMS   int     `json:"baseAttackMs"`
	AggroRange     float64 `json:"aggroRange"`
	LeashRange     float64 `json:"leashRange"`
	SpawnDistance  float64 `json:"spawnDistance"`
	MovementType   int     `json:"movementType"`
	AI             string  `json:"ai"`
	Channel        int16   `json:"channel"`
}

func newHTTPNpcServerClient(baseURL string) *httpNpcServerClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	return &httpNpcServerClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 250 * time.Millisecond},
	}
}

func (c *httpNpcServerClient) VisibleWorldSnapshot(viewer *Client, force bool) (*npcServerVisibilitySnapshot, error) {
	if c == nil || viewer == nil || viewer.Char == nil {
		return nil, nil
	}

	values := url.Values{}
	values.Set("map", fmt.Sprint(viewer.Char.MapID))
	values.Set("x", fmt.Sprint(int16(asda2X(viewer.Char.X, viewer.Char.MapID))))
	values.Set("y", fmt.Sprint(int16(asda2Y(viewer.Char.Y, viewer.Char.MapID))))
	values.Set("channel", fmt.Sprint(viewer.Channel))
	if force {
		values.Set("force", "true")
	}

	resp, err := c.client.Get(c.baseURL + "/visible?" + values.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	var payload npcServerVisibleResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	snapshot := &npcServerVisibilitySnapshot{
		MapID:        payload.MapID,
		Channel:      payload.Channel,
		NpcCount:     payload.NpcCount,
		MonsterCount: payload.MonsterCount,
		Npcs:         make([]*Npc, 0, len(payload.Npcs)),
		Monsters:     make([]*Monster, 0, len(payload.Monsters)),
	}
	for _, row := range payload.Npcs {
		template := types.NormalizeNpcTemplate(types.NpcTemplateRow{
			EntryID:         row.EntryID,
			Name:            row.Name,
			Kind:            row.Kind,
			ClassGroup:      row.ClassGroup,
			IsTrainer:       row.IsTrainer,
			InteractionKind: row.InteractionKind,
		})
		snapshot.Npcs = append(snapshot.Npcs, &Npc{
			SessionID:       row.SessionID,
			WorldEntityID:   row.WorldEntityID,
			SpawnID:         row.SpawnID,
			EntryID:         row.EntryID,
			Name:            template.Name,
			Kind:            template.Kind,
			ClassGroup:      template.ClassGroup,
			IsTrainer:       template.IsTrainer,
			InteractionKind: template.InteractionKind,
			MapID:           row.MapID,
			LocalX:          row.LocalX,
			LocalY:          row.LocalY,
			Channel:         row.Channel,
		})
	}

	now := time.Now()
	for _, row := range payload.Monsters {
		snapshot.Monsters = append(snapshot.Monsters, npcServerMonsterToRuntime(row, now))
	}
	return snapshot, nil
}

func (c *httpNpcServerClient) VisibleWorld(viewer *Client, force bool) ([]*Npc, []*Monster, error) {
	snapshot, err := c.VisibleWorldSnapshot(viewer, force)
	if err != nil || snapshot == nil {
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	return snapshot.Npcs, snapshot.Monsters, nil
}

func (c *httpNpcServerClient) SyncPlayer(player *Client) error {
	return c.postPlayer("/player/sync", player)
}

func (c *httpNpcServerClient) LeavePlayer(player *Client) error {
	return c.postPlayer("/player/leave", player)
}

func (c *httpNpcServerClient) postPlayer(path string, player *Client) error {
	if c == nil || player == nil || player.Char == nil {
		return nil
	}
	payload := npcruntime.Player{
		AccountID: player.Char.AccID,
		SessionID: player.Char.SessionID,
		Character: player.Char.Name,
		MapID:     player.Char.MapID,
		X:         int16(asda2X(player.Char.X, player.Char.MapID)),
		Y:         int16(asda2Y(player.Char.Y, player.Char.MapID)),
		Channel:   int16(player.Channel),
		UpdatedAt: time.Now(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.client.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s status %s", path, resp.Status)
	}
	return nil
}

func npcServerMonsterToRuntime(row npcServerMonster, now time.Time) *Monster {
	attackMS := row.BaseAttackMS
	if attackMS <= 0 {
		attackMS = int(defaultMonsterAttackInterval / time.Millisecond)
	}
	monster := &Monster{
		SessionID:      row.SessionID,
		WorldEntityID:  row.WorldEntityID,
		SpawnID:        row.SpawnID,
		EntryID:        row.EntryID,
		Level:          row.Level,
		MapID:          row.MapID,
		LocalX:         row.LocalX,
		LocalY:         row.LocalY,
		HomeX:          row.LocalX,
		HomeY:          row.LocalY,
		Health:         row.Health,
		MaxHealth:      row.MaxHealth,
		MoveMS:         row.MoveMS,
		WalkSpeed:      row.WalkSpeed,
		RunSpeed:       row.RunSpeed,
		RespawnSeconds: row.RespawnSeconds,
		MinDamage:      row.MinDamage,
		MaxDamage:      row.MaxDamage,
		AttackInterval: time.Duration(attackMS) * time.Millisecond,
		AggroRange:     row.AggroRange,
		AttackRange:    defaultMonsterAttackRange,
		LeashRange:     row.LeashRange,
		RoamRadius:     row.SpawnDistance,
		MovementType:   row.MovementType,
		AI:             strings.TrimSpace(row.AI),
		State:          MonsterStateOK,
		MoveType:       monsterMoveTypeWalk,
		NextRoamAt:     now.Add(randomRoamInterval()),
		NextSkillAt:    now.Add(defaultMonsterSkillCooldown),
		NpcServerOwned: true,
	}
	if monster.Health <= 0 {
		monster.Health = defaultMonsterHealth
	}
	if monster.MaxHealth <= 0 {
		monster.MaxHealth = monster.Health
	}
	if monster.MoveMS <= 0 {
		monster.MoveMS = defaultMonsterMoveMS
	}
	if monster.WalkSpeed <= 0 {
		monster.WalkSpeed = defaultMonsterWalkSpeed
	}
	if monster.RunSpeed <= 0 {
		monster.RunSpeed = defaultMonsterRunSpeed
	}
	if monster.MinDamage <= 0 {
		monster.MinDamage = float64(defaultMonsterAttackDamage)
	}
	if monster.MaxDamage < monster.MinDamage {
		monster.MaxDamage = monster.MinDamage
	}
	if monster.AggroRange <= 0 {
		monster.AggroRange = defaultMonsterAggroRange
	}
	if monster.LeashRange <= 0 {
		monster.LeashRange = defaultMonsterLeashRange
	}
	if monster.RoamRadius <= 0 {
		monster.RoamRadius = defaultMonsterRoamRadius
	}
	return monster
}

func logNpcServerUnavailable(err error) {
	if err == nil {
		return
	}
	npcServerLogState.Lock()
	defer npcServerLogState.Unlock()

	now := time.Now()
	if !npcServerLogState.unavailable {
		npcServerLogState.unavailable = true
		npcServerLogState.lastLog = now
		npcServerLogState.suppressed = 0
		log.Printf("[Warn] NpcServer visibility unavailable: %v", err)
		return
	}

	npcServerLogState.suppressed++
	if now.Sub(npcServerLogState.lastLog) >= npcServerUnavailableLogEvery {
		log.Printf("[Warn] NpcServer still unavailable suppressed=%d last=%v", npcServerLogState.suppressed, err)
		npcServerLogState.lastLog = now
		npcServerLogState.suppressed = 0
	}
}

func logNpcServerAvailable() {
	npcServerLogState.Lock()
	defer npcServerLogState.Unlock()
	if !npcServerLogState.unavailable {
		return
	}
	log.Printf("[NpcServer] visibility restored")
	npcServerLogState.unavailable = false
	npcServerLogState.suppressed = 0
	npcServerLogState.lastLog = time.Time{}
}

func syncNpcServerPlayer(c *Client) {
	if npcServerClient == nil || c == nil || c.Char == nil {
		return
	}
	if err := npcServerClient.SyncPlayer(c); err != nil {
		logNpcServerUnavailable(err)
		return
	}
	logNpcServerAvailable()
}

func leaveNpcServerPlayer(c *Client) {
	if npcServerClient == nil || c == nil || c.Char == nil {
		return
	}
	if err := npcServerClient.LeavePlayer(c); err != nil {
		logNpcServerUnavailable(err)
		return
	}
	logNpcServerAvailable()
}
