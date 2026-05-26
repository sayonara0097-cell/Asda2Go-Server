package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
)

// Mirrors WorldObject.BroadcastRange from WCell.RealmServer/Entities/WorldObject.cs.
const characterBroadcastRange = 50.0

var visibilityDebugEnabled bool

type characterVisibilityAction struct {
	viewer *Client
	target *Client
	remove bool
}

func (w *worldMgr) RefreshCharacterVisibility(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	if gm := w.GetMap(c.Char.MapID); gm != nil {
		gm.RefreshCharacterVisibility(c)
	}
}

func (w *worldMgr) AreaRecipients(c *Client, includeSelf bool) []*Client {
	if c == nil || c.Char == nil {
		return nil
	}
	if gm := w.GetMap(c.Char.MapID); gm != nil {
		return gm.AreaRecipients(c, includeSelf)
	}
	return nil
}

func (w *worldMgr) EnsureCharacterKnown(viewer *Client, target *Client) bool {
	if viewer == nil || viewer.Char == nil || target == nil || target.Char == nil {
		return false
	}
	if gm := w.GetMap(viewer.Char.MapID); gm != nil {
		return gm.EnsureCharacterKnown(viewer, target)
	}
	return false
}

func (m *GameMap) RefreshCharacterVisibility(c *Client) {
	sendCharacterVisibilityActions(m.refreshCharacterVisibility(c))
}

func (m *GameMap) refreshCharacterVisibility(c *Client) []characterVisibilityAction {
	if c == nil || c.Char == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.characters[c.ID]; !ok {
		return nil
	}

	actions := make([]characterVisibilityAction, 0, len(m.characters))
	for _, other := range m.characters {
		if other == nil || other.ID == c.ID || other.Char == nil {
			continue
		}
		visible := charactersCanSee(c, other)
		actions = m.setCharacterKnowledgeLocked(c, other, visible, actions)
		actions = m.setCharacterKnowledgeLocked(other, c, visible, actions)
	}
	return actions
}

func (m *GameMap) ForgetCharacter(c *Client) {
	if c == nil || c.Char == nil {
		return
	}

	var actions []characterVisibilityAction

	m.mu.Lock()
	delete(m.knownChars, c.ID)
	delete(m.knownMonsters, c.ID)
	for viewerID, known := range m.knownChars {
		if _, ok := known[c.ID]; !ok {
			continue
		}
		delete(known, c.ID)
		viewer := m.characters[viewerID]
		if viewer != nil && viewer.Char != nil {
			actions = append(actions, characterVisibilityAction{
				viewer: viewer,
				target: c,
				remove: true,
			})
		}
	}
	m.mu.Unlock()

	sendCharacterVisibilityActions(actions)
}

func (m *GameMap) AreaRecipients(c *Client, includeSelf bool) []*Client {
	if c == nil || c.Char == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.characters[c.ID]; !ok {
		return nil
	}

	out := make([]*Client, 0, len(m.characters))
	if includeSelf {
		out = append(out, c)
	}
	for _, viewer := range m.characters {
		if viewer == nil || viewer.ID == c.ID || viewer.Char == nil {
			continue
		}
		if m.characterKnownLocked(viewer.ID, c.ID) {
			out = append(out, viewer)
		}
	}
	return out
}

func (m *GameMap) EnsureCharacterKnown(viewer *Client, target *Client) bool {
	if viewer == nil || viewer.Char == nil || target == nil || target.Char == nil || viewer.ID == target.ID {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.characters[viewer.ID] == nil || m.characters[target.ID] == nil {
		return false
	}
	if !charactersCanSee(viewer, target) {
		return false
	}
	m.ensureKnownSetLocked(viewer.ID)[target.ID] = struct{}{}
	debugVisibilityf("repair viewer=%s target=%s map=%d channel=%d dist=%.2f",
		clientDebugLabel(viewer), clientDebugLabel(target), viewer.Char.MapID, viewer.Channel, characterDistance(viewer.Char, target.Char))
	return false
}

func (m *GameMap) setCharacterKnowledgeLocked(viewer *Client, target *Client, visible bool, actions []characterVisibilityAction) []characterVisibilityAction {
	if viewer == nil || viewer.Char == nil || target == nil || target.Char == nil || viewer.ID == target.ID {
		return actions
	}

	known := m.ensureKnownSetLocked(viewer.ID)
	_, wasKnown := known[target.ID]
	if visible {
		if !wasKnown {
			known[target.ID] = struct{}{}
			actions = append(actions, characterVisibilityAction{viewer: viewer, target: target})
		}
		return actions
	}

	if wasKnown {
		delete(known, target.ID)
		actions = append(actions, characterVisibilityAction{viewer: viewer, target: target, remove: true})
	}
	return actions
}

func (m *GameMap) ensureKnownSetLocked(viewerID uint32) map[uint32]struct{} {
	known := m.knownChars[viewerID]
	if known == nil {
		known = make(map[uint32]struct{})
		m.knownChars[viewerID] = known
	}
	return known
}

func (m *GameMap) characterKnownLocked(viewerID uint32, targetID uint32) bool {
	known := m.knownChars[viewerID]
	if known == nil {
		return false
	}
	_, ok := known[targetID]
	return ok
}

func (m *GameMap) KnowsCharacter(viewer *Client, target *Client) bool {
	if viewer == nil || target == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.characterKnownLocked(viewer.ID, target.ID)
}

func charactersCanSee(a *Client, b *Client) bool {
	if a == nil || a.Char == nil || b == nil || b.Char == nil {
		return false
	}
	if a.Char.MapID != b.Char.MapID || a.Channel != b.Channel {
		return false
	}
	return characterDistance(a.Char, b.Char) < characterBroadcastRange
}

func characterDistance(a *Character, b *Character) float64 {
	if a == nil || b == nil {
		return math.MaxFloat64
	}
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
}

func sendCharacterVisibilityActions(actions []characterVisibilityAction) {
	for _, action := range actions {
		if action.viewer == nil || action.viewer.Char == nil || action.target == nil || action.target.Char == nil {
			continue
		}
		if action.remove {
			debugVisibilityf("hide viewer=%s target=%s map=%d channel=%d dist=%.2f",
				clientDebugLabel(action.viewer), clientDebugLabel(action.target),
				action.target.Char.MapID, action.viewer.Channel, characterDistance(action.viewer.Char, action.target.Char))
			sendCharacterDelete(action.viewer, action.target.Char)
			continue
		}
		debugVisibilityf("show viewer=%s target=%s map=%d channel=%d dist=%.2f",
			clientDebugLabel(action.viewer), clientDebugLabel(action.target),
			action.target.Char.MapID, action.viewer.Channel, characterDistance(action.viewer.Char, action.target.Char))
		sendCharacterVisibleNow(action.viewer, action.target.Char)
	}
}

func sendCharacterDelete(receiver *Client, chr *Character) {
	if receiver == nil || chr == nil {
		return
	}
	sendEntityDelete(receiver, chr.SessionID, int32(chr.AccID))
}

func sendEntityDelete(receiver *Client, sessionID int16, worldEntityID int32) {
	if receiver == nil {
		return
	}
	p := NewPacket(CharacterDelete)
	p.WriteInt16(sessionID)
	p.WriteInt32(worldEntityID)
	receiver.Send(p)
}

func handleIDontKnowAboutCharacter(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil || p == nil {
		return
	}

	for _, sessionID := range dontKnowCharacterSessionCandidates(p) {
		target := getClientBySessionID(sessionID)
		if target == nil || target.Char == nil || target.ID == c.ID {
			continue
		}
		if World.EnsureCharacterKnown(c, target) {
			sendCharacterVisibleNow(c, target.Char)
			return
		}
	}

	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	for _, sessionID := range dontKnowCharacterSessionCandidates(p) {
		if sessionID <= 0 {
			continue
		}
		monster, ok := gm.FindMonsterByClientTargetID(uint16(sessionID))
		if !ok {
			continue
		}
		if gm.EnsureMonsterKnown(c, monster) {
			sendMonsterVisible(c, monster)
			return
		}
	}
}

func dontKnowCharacterSessionCandidates(p *PacketIn) []int16 {
	if p == nil {
		return nil
	}

	offsets := []int{0, 24, 28}
	candidates := make([]int16, 0, len(offsets))
	seen := make(map[int16]struct{}, len(offsets))
	for _, offset := range offsets {
		if len(p.Data) < offset+2 {
			continue
		}
		sessionID := int16(binary.LittleEndian.Uint16(p.Data[offset:]))
		if sessionID <= 0 {
			continue
		}
		if _, ok := seen[sessionID]; ok {
			continue
		}
		seen[sessionID] = struct{}{}
		candidates = append(candidates, sessionID)
	}
	return candidates
}

func envVisibilityDebugEnabled() bool {
	for _, name := range []string{"ASDAGO_DEBUG_VISIBILITY", "ASDA2_DEBUG_VISIBILITY"} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
		switch value {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return true
}

func debugVisibilityf(format string, args ...any) {
	if !visibilityDebugEnabled {
		return
	}
	log.Printf("[Visibility] "+format, args...)
}

func setVisibilityDebug(enabled bool) {
	visibilityDebugEnabled = enabled
	if enabled {
		log.Printf("[Visibility] debug enabled")
		return
	}
	log.Printf("[Visibility] debug disabled")
}

func clientDebugLabel(c *Client) string {
	if c == nil || c.Char == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s#%d", c.Char.Name, c.Char.SessionID)
}

func characterDebugLabel(chr *Character) string {
	if chr == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s#%d", chr.Name, chr.SessionID)
}

func clientListDebugLabel(clients []*Client) string {
	if len(clients) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(clients))
	for _, c := range clients {
		parts = append(parts, clientDebugLabel(c))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
