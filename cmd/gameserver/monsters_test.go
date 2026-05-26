package main

import (
	"math"
	"sync/atomic"
	"testing"
	"time"
)

func TestAllocMonsterSessionIDUsesHighRange(t *testing.T) {
	old := atomic.LoadInt32(&nextMonsterSessionID)
	defer atomic.StoreInt32(&nextMonsterSessionID, old)

	atomic.StoreInt32(&nextMonsterSessionID, defaultMonsterSessionIDStart)
	id := allocMonsterSessionID()
	if int32(id) <= defaultMonsterSessionIDStart {
		t.Fatalf("monster session id = %d, want above %d", id, defaultMonsterSessionIDStart)
	}
	if id <= 1000 {
		t.Fatalf("monster session id = %d, should stay away from early player/NPC ids", id)
	}
}

func TestMonsterMovementUsesReferenceSpeedFields(t *testing.T) {
	monster := &Monster{WalkSpeed: 0.5, RunSpeed: 3.5}

	if got, want := monsterMoveUnitMS(monster, monsterMoveTypeWalk), int16(2000); got != want {
		t.Fatalf("walk move unit ms=%d, want %d", got, want)
	}
	if got, want := monsterMoveUnitMS(monster, monsterMoveTypeRun), int16(286); got != want {
		t.Fatalf("run move unit ms=%d, want %d", got, want)
	}
}

func TestMonsterVisibleCoordinatesIncludeDestinationWhileMoving(t *testing.T) {
	monster := &Monster{
		LocalX:        10,
		LocalY:        20,
		IsMoving:      true,
		MoveDestX:     14,
		MoveDestY:     25,
		MoveStartedAt: time.Now(),
	}

	x, y, destX, destY := monsterVisibleCoordinates(monster)
	if x != 10 || y != 20 || destX != 14 || destY != 25 {
		t.Fatalf("visible coords=(%d,%d)->(%d,%d), want (10,20)->(14,25)", x, y, destX, destY)
	}
}

func TestAdvanceMonsterMovementReportsArrival(t *testing.T) {
	gm := &GameMap{
		monsters: map[int16]*Monster{},
	}
	start := time.Now()
	monster := &Monster{
		SessionID:     20020,
		State:         MonsterStateOK,
		Health:        100,
		LocalX:        10,
		LocalY:        20,
		IsMoving:      true,
		MoveFromX:     10,
		MoveFromY:     20,
		MoveDestX:     12,
		MoveDestY:     24,
		MoveStartedAt: start,
		MoveDuration:  time.Second,
	}
	gm.monsters[monster.SessionID] = monster

	if arrived := gm.advanceMonsterMovement(monster, start.Add(500*time.Millisecond)); arrived {
		t.Fatal("advanceMonsterMovement reported arrival too early")
	}
	if arrived := gm.advanceMonsterMovement(monster, start.Add(time.Second)); !arrived {
		t.Fatal("advanceMonsterMovement did not report arrival")
	}
	if monster.IsMoving || monster.LocalX != 12 || monster.LocalY != 24 {
		t.Fatalf("monster after arrival moving=%v pos=%d,%d, want stopped at 12,24", monster.IsMoving, monster.LocalX, monster.LocalY)
	}
}

func TestMonsterChaseDestinationStopsNearTarget(t *testing.T) {
	monster := &Monster{LocalX: 10, LocalY: 10, AttackRange: 1.5}
	target := &Client{Char: &Character{MapID: 0}}
	target.Char.X = 20
	target.Char.Y = 10

	x, y := monsterChaseDestination(monster, target)
	if x >= 20 || x <= 10 || y != 10 {
		t.Fatalf("chase destination=%d,%d, want between monster and target near y=10", x, y)
	}
	if math.Abs(float64(x-20)) > monster.AttackRange || math.Abs(float64(y-10)) > monster.AttackRange {
		t.Fatalf("chase destination=%d,%d is outside axis attack range %.2f", x, y, monster.AttackRange)
	}
}

func TestMonsterCanAttackClientRequiresCloseAxes(t *testing.T) {
	monster := &Monster{LocalX: 397, LocalY: 346, AttackRange: 1.5}
	target := &Client{Char: &Character{MapID: 0, X: 395, Y: 352}}
	if monsterCanAttackClient(monster, target) {
		t.Fatal("monster can attack from far Y offset")
	}

	target.Char.X = 396
	target.Char.Y = 347
	if !monsterCanAttackClient(monster, target) {
		t.Fatal("monster cannot attack adjacent target")
	}
}

func TestUpdateMonsterAISkipsNpcServerOwnedMonsters(t *testing.T) {
	gm := newGameMap(&MapTemplate{ID: 0, Name: "test"})
	target := testVisibilityClient(1, 101, 0, 0, 10, 10)
	target.Char.HP = 100
	monster := &Monster{
		SessionID:      20020,
		EntryID:        1,
		MapID:          0,
		LocalX:         12,
		LocalY:         10,
		HomeX:          12,
		HomeY:          10,
		Health:         100,
		MaxHealth:      100,
		State:          MonsterStateOK,
		AggroRange:     50,
		AttackRange:    1.5,
		MovementType:   1,
		AI:             "passive_roam",
		WalkSpeed:      0.5,
		RunSpeed:       3.5,
		NextRoamAt:     time.Now().Add(-time.Second),
		NpcServerOwned: true,
	}

	gm.characters[target.ID] = target
	gm.monsters[monster.SessionID] = monster
	gm.updateMonsterAI([]*Client{target}, time.Now())

	if monster.TargetSession != 0 {
		t.Fatalf("npcserver monster target session=%d, want 0", monster.TargetSession)
	}
	if monster.IsMoving {
		t.Fatal("npcserver monster should not be moved by local AI")
	}
	if target.Char.HP != 100 {
		t.Fatalf("target HP=%d, want unchanged 100", target.Char.HP)
	}
}
