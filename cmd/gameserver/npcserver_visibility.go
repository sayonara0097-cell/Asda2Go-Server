package main

import (
	"log"
	"time"
)

type npcServerVisibilityStats struct {
	SentNpcs      int
	TotalNpcs     int
	SentMonsters  int
	TotalMonsters int
	Disappeared   int
}

type npcServerDeleteAction struct {
	viewer        *Client
	sessionID     int16
	worldEntityID int32
}

func (w *worldMgr) RefreshNpcServerVisibility(c *Client, forceResend bool) (int, int) {
	if c == nil || c.Char == nil {
		return 0, 0
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return 0, 0
	}
	if !forceResend {
		due, scheduledForceResend := gm.npcServerVisibilityDue(c.ID, time.Now())
		if !due {
			return 0, 0
		}
		forceResend = scheduledForceResend
	}
	stats := gm.refreshNpcServerVisibility(c, forceResend)
	return stats.SentNpcs, stats.SentMonsters
}

func (w *worldMgr) TickNpcServerVisibility(c *Client, now time.Time) {
	if c == nil || c.Char == nil {
		return
	}
	gm := w.GetMap(c.Char.MapID)
	if gm == nil {
		return
	}
	due, forceResend := gm.npcServerVisibilityDue(c.ID, now)
	if !due {
		return
	}
	gm.refreshNpcServerVisibility(c, forceResend)
}

func (m *GameMap) npcServerVisibilityDue(viewerID uint32, now time.Time) (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	forceResend := false
	if npcServerVisibilityFullResyncInterval > 0 {
		if next := m.nextNpcServerFull[viewerID]; next.IsZero() {
			m.nextNpcServerFull[viewerID] = now.Add(npcServerVisibilityFullResyncInterval)
		} else if !now.Before(next) {
			m.nextNpcServerFull[viewerID] = now.Add(npcServerVisibilityFullResyncInterval)
		}
	}

	if next := m.nextNpcResync[viewerID]; !next.IsZero() && now.Before(next) && !forceResend {
		return false, false
	}
	m.nextNpcResync[viewerID] = now.Add(npcServerVisibilityResyncInterval)
	return true, forceResend
}

func (m *GameMap) refreshNpcServerVisibility(c *Client, forceResend bool) npcServerVisibilityStats {
	snapshot, err := npcServerClient.VisibleWorldSnapshot(c, forceResend)
	if err != nil {
		logNpcServerUnavailable(err)
		stats := m.flushNpcServerVisibility()
		if stats.Disappeared > 0 {
			mode := visibilityMode(forceResend)
			log.Printf("[NpcServer] visibility refreshed for %q mode=%s sentNpcs=0/0 sentMonsters=0/0 disappeared=%d",
				c.Char.Name, mode, stats.Disappeared)
		}
		return stats
	}
	if snapshot == nil {
		return npcServerVisibilityStats{}
	}
	logNpcServerAvailable()

	stats := npcServerVisibilityStats{
		TotalNpcs:     visibleTotal(snapshot.NpcCount, len(snapshot.Npcs)),
		TotalMonsters: visibleTotal(snapshot.MonsterCount, len(snapshot.Monsters)),
	}
	var disappeared []npcServerDeleteAction
	var visibleNpcs []*Npc
	var visibleMonsters []*Monster

	m.mu.Lock()
	if _, ok := m.characters[c.ID]; !ok {
		m.mu.Unlock()
		return stats
	}

	knownNpcs := m.ensureKnownNpcSetLocked(c.ID)
	knownMonsters := m.ensureKnownMonsterSetLocked(c.ID)
	hasSnapshotRows := len(snapshot.Npcs) > 0 || len(snapshot.Monsters) > 0
	effectiveForceResend := forceResend || (hasSnapshotRows && len(knownNpcs) == 0 && len(knownMonsters) == 0)
	for _, npc := range snapshot.Npcs {
		if npc == nil {
			continue
		}
		m.npcs[npc.SessionID] = npc
	}
	for sessionID, npc := range m.npcs {
		if npc == nil {
			continue
		}
		_, wasKnown := knownNpcs[sessionID]
		canSee := clientCanSeeNpc(c, npc)
		if canSee && (effectiveForceResend || !wasKnown) {
			if len(visibleNpcs) >= npcServerMaxVisibleNpcsPerRefresh {
				continue
			}
			knownNpcs[sessionID] = struct{}{}
			visibleNpcs = append(visibleNpcs, npc)
			continue
		}
		if !canSee && wasKnown {
			if len(disappeared) >= npcServerMaxDeletesPerRefresh {
				continue
			}
			disappeared = append(disappeared, npcServerDeleteAction{
				viewer:        c,
				sessionID:     npc.SessionID,
				worldEntityID: npc.WorldEntityID,
			})
			delete(knownNpcs, sessionID)
		}
	}

	if npcServerMonsterVisibilityEnabled {
		for _, monster := range snapshot.Monsters {
			if monster == nil {
				continue
			}
			if existing := m.monsters[monster.SessionID]; existing != nil {
				continue
			}
			m.monsters[monster.SessionID] = monster
		}
		for sessionID, monster := range m.monsters {
			if monster == nil {
				continue
			}
			_, wasKnown := knownMonsters[sessionID]
			canSee := clientCanSeeMonster(c, monster)
			if canSee && (effectiveForceResend || !wasKnown) {
				if len(visibleMonsters) >= npcServerMaxVisibleNpcsPerRefresh {
					continue
				}
				knownMonsters[sessionID] = struct{}{}
				visibleMonsters = append(visibleMonsters, monster)
				continue
			}
			if !canSee && wasKnown {
				if len(disappeared) >= npcServerMaxDeletesPerRefresh {
					continue
				}
				disappeared = append(disappeared, npcServerDeleteAction{
					viewer:        c,
					sessionID:     monster.SessionID,
					worldEntityID: monster.WorldEntityID,
				})
				delete(knownMonsters, sessionID)
			}
		}
	} else {
		for sessionID := range knownMonsters {
			monster := m.monsters[sessionID]
			if monster == nil || !monster.NpcServerOwned {
				continue
			}
			disappeared = append(disappeared, npcServerDeleteAction{
				viewer:        c,
				sessionID:     monster.SessionID,
				worldEntityID: monster.WorldEntityID,
			})
			delete(knownMonsters, sessionID)
			delete(m.monsters, sessionID)
		}
	}
	m.mu.Unlock()

	for _, npc := range visibleNpcs {
		sendNpcVisible(c, npc)
	}
	for _, monster := range visibleMonsters {
		sendMonsterVisible(c, monster)
	}
	for _, action := range disappeared {
		sendEntityDelete(action.viewer, action.sessionID, action.worldEntityID)
	}

	stats.SentNpcs = len(visibleNpcs)
	if stats.TotalNpcs < stats.SentNpcs {
		stats.TotalNpcs = stats.SentNpcs
	}
	stats.SentMonsters = len(visibleMonsters)
	if stats.TotalMonsters < stats.SentMonsters {
		stats.TotalMonsters = stats.SentMonsters
	}
	stats.Disappeared = len(disappeared)
	mode := visibilityMode(effectiveForceResend)
	if stats.SentNpcs > 0 || stats.SentMonsters > 0 || stats.Disappeared > 0 || effectiveForceResend {
		log.Printf("[Info] NpcServer visibility refreshed for %q mode=%s sentNpcs=%d/%d sentMonsters=%d/%d disappeared=%d",
			c.Char.Name,
			mode,
			stats.SentNpcs,
			stats.TotalNpcs,
			stats.SentMonsters,
			stats.TotalMonsters,
			stats.Disappeared)
	}
	return stats
}

func visibilityMode(force bool) string {
	if force {
		return "full"
	}
	return "delta"
}

func (m *GameMap) flushNpcServerVisibility() npcServerVisibilityStats {
	var actions []npcServerDeleteAction

	m.mu.Lock()
	for viewerID, known := range m.knownNpcs {
		viewer := m.characters[viewerID]
		for sessionID := range known {
			if npc := m.npcs[sessionID]; npc != nil && viewer != nil {
				actions = append(actions, npcServerDeleteAction{
					viewer:        viewer,
					sessionID:     npc.SessionID,
					worldEntityID: npc.WorldEntityID,
				})
			}
			delete(known, sessionID)
		}
	}
	for viewerID, known := range m.knownMonsters {
		viewer := m.characters[viewerID]
		for sessionID := range known {
			if monster := m.monsters[sessionID]; monster != nil && viewer != nil {
				actions = append(actions, npcServerDeleteAction{
					viewer:        viewer,
					sessionID:     monster.SessionID,
					worldEntityID: monster.WorldEntityID,
				})
			}
			delete(known, sessionID)
		}
	}
	m.npcs = make(map[int16]*Npc)
	m.monsters = make(map[int16]*Monster)
	for _, viewer := range m.characters {
		if viewer != nil && viewer.Char != nil {
			viewer.Char.TargetID = -1
			viewer.Char.IsFighting = false
		}
	}
	m.mu.Unlock()

	for _, action := range actions {
		sendEntityDelete(action.viewer, action.sessionID, action.worldEntityID)
	}
	return npcServerVisibilityStats{Disappeared: len(actions)}
}

func visibleTotal(serverCount int, rowCount int) int {
	if serverCount > 0 {
		return serverCount
	}
	return rowCount
}
