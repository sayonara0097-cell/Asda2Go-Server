package main

import (
	"testing"
	"time"
)

func TestAdvanceCharacterMovementUpdatesPositionTowardDestination(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 20, 10, -1)
	c.Char.MoveLastUpdate = time.Now().Add(-1 * time.Second)

	if !advanceCharacterMovement(c) {
		t.Fatal("advanceCharacterMovement should report a position update")
	}
	if c.Char.X <= 10 || c.Char.X >= 20 {
		t.Fatalf("x = %.2f, want movement toward destination without overshoot", c.Char.X)
	}
	if c.Char.Y != 10 {
		t.Fatalf("y = %.2f, want unchanged y", c.Char.Y)
	}
	if c.Char.LastFacingX <= 0.99 || c.Char.LastFacingY != 0 {
		t.Fatalf("facing = %.2f,%.2f, want east", c.Char.LastFacingX, c.Char.LastFacingY)
	}
	if !c.Char.IsMoving {
		t.Fatal("character should still be moving before reaching destination")
	}
}

func TestAdvanceCharacterMovementStopsAtDestination(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 10.5, 10, -1)
	c.Char.MoveLastUpdate = time.Now().Add(-1 * time.Second)

	if !advanceCharacterMovement(c) {
		t.Fatal("advanceCharacterMovement should report a final position update")
	}
	if c.Char.X != 10.5 || c.Char.Y != 10 {
		t.Fatalf("position = %.2f,%.2f, want destination 10.50,10.00", c.Char.X, c.Char.Y)
	}
	if c.Char.IsMoving {
		t.Fatal("character should stop after reaching destination")
	}
}

func TestShouldPreserveMovementOnZeroEndWhileDestinationRemains(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 20, 10, -1)
	c.Char.LastFacingX = 0
	c.Char.LastFacingY = 1
	c.Char.Orientation = 1.5708

	if !shouldPreserveMovementOnZeroEnd(c.Char, 11, 10, 11, 10) {
		t.Fatal("zero-length end away from destination should preserve walking")
	}

	preserveCharacterMovementAfterZeroEnd(c, 11, 10, -1)
	if !c.Char.IsMoving {
		t.Fatal("character should continue moving after preserved zero-length end")
	}
	if c.Char.MoveDestX != 20 || c.Char.MoveDestY != 10 {
		t.Fatalf("destination = %.2f,%.2f, want original destination 20.00,10.00", c.Char.MoveDestX, c.Char.MoveDestY)
	}
	if c.Char.X != 11 || c.Char.Y != 10 {
		t.Fatalf("position = %.2f,%.2f, want updated current position 11.00,10.00", c.Char.X, c.Char.Y)
	}
	if c.Char.LastFacingX != 0 || c.Char.LastFacingY != 1 {
		t.Fatalf("facing = %.2f,%.2f, want unchanged jump direction", c.Char.LastFacingX, c.Char.LastFacingY)
	}
	if c.Char.Orientation != 1.5708 {
		t.Fatalf("orientation = %.4f, want unchanged orientation", c.Char.Orientation)
	}
}

func TestShouldPreserveMovementOnZeroStartWhileDestinationRemains(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 20, 10, -1)
	c.Char.LastFacingX = 0
	c.Char.LastFacingY = 1
	c.Char.Orientation = 1.5708

	if !shouldPreserveMovementOnZeroStart(c.Char, 11, 10, 11, 10) {
		t.Fatal("zero-length start away from destination should preserve walking")
	}

	preserveCharacterMovementAfterZeroEnd(c, 11, 10, -1)
	if c.Char.MoveDestX != 20 || c.Char.MoveDestY != 10 {
		t.Fatalf("destination = %.2f,%.2f, want original destination 20.00,10.00", c.Char.MoveDestX, c.Char.MoveDestY)
	}
	if c.Char.LastFacingX != 0 || c.Char.LastFacingY != 1 {
		t.Fatalf("facing = %.2f,%.2f, want unchanged jump direction", c.Char.LastFacingX, c.Char.LastFacingY)
	}
}

func TestShouldNotPreserveMovementOnArrivalEnd(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 20, 10, -1)

	if shouldPreserveMovementOnZeroEnd(c.Char, 20, 10, 20, 10) {
		t.Fatal("zero-length end at destination should finish movement")
	}
}

func TestStandingZeroEndPreservesFacing(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.LastFacingX = 0
	c.Char.LastFacingY = 1
	c.Char.Orientation = 1.5708

	if !isZeroLengthEndMove(10, 10, 10, 10) {
		t.Fatal("same-point end should be treated as zero-length")
	}

	preserveCharacterFacingAfterZeroEnd(c, 10, 10, -1)
	if c.Char.IsMoving {
		t.Fatal("standing zero-length end should keep character stopped")
	}
	if c.Char.LastFacingX != 0 || c.Char.LastFacingY != 1 {
		t.Fatalf("facing = %.2f,%.2f, want unchanged north-facing direction", c.Char.LastFacingX, c.Char.LastFacingY)
	}
	if c.Char.Orientation != 1.5708 {
		t.Fatalf("orientation = %.4f, want unchanged orientation", c.Char.Orientation)
	}
}

func TestFacingTargetUsesStoredDirection(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.LastFacingX = 0
	c.Char.LastFacingY = 1

	x, y := characterFacingTarget(c.Char, 10, 10)
	if x != 10 || y <= 10 {
		t.Fatalf("facing target = %.2f,%.2f, want slight north-facing target", x, y)
	}
}

func TestPrepareCharacterRespawnAtCurrentStopsMovementAndKeepsFacing(t *testing.T) {
	c := testVisibilityClient(1, 101, 0, 0, 10, 10)
	c.Char.RunSpeed = 0.259
	startCharacterMovement(c, 10, 10, 20, 10, -1)
	c.Char.MoveLastUpdate = time.Now().Add(-1 * time.Second)

	prepareCharacterRespawnAtCurrent(c)
	if c.Char.IsMoving {
		t.Fatal("respawn preparation should stop movement")
	}
	if c.Char.MoveDestX != 0 || c.Char.MoveDestY != 0 {
		t.Fatalf("move destination = %.2f,%.2f, want cleared", c.Char.MoveDestX, c.Char.MoveDestY)
	}
	if c.Char.X <= 10 || c.Char.X >= 20 {
		t.Fatalf("x = %.2f, want current in-flight position saved", c.Char.X)
	}
	if c.Char.LastFacingX <= 0.99 || c.Char.LastFacingY != 0 {
		t.Fatalf("facing = %.2f,%.2f, want east", c.Char.LastFacingX, c.Char.LastFacingY)
	}
}
