package main

import (
	"math"
	"time"
)

// ---- Movement runtime ----

const (
	defaultCharacterRunSpeed float32 = 0.259
	moveSpeedTimeDivisor     float32 = 100.0 // mirrors WCell: RunSpeed * elapsedMillis / 100
	zeroEndMoveTolerance     float32 = 0.25
	activeMoveDestTolerance  float32 = 0.75
	standingJumpMoveLimit    float32 = 2.0
)

func startCharacterMovement(c *Client, localFromX float32, localFromY float32, localDestX float32, localDestY float32, target int16) {
	if c == nil || c.Char == nil {
		return
	}
	if c.Char.RunSpeed <= 0 {
		c.Char.RunSpeed = defaultCharacterRunSpeed
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localFromX + offset
	c.Char.Y = localFromY + offset
	c.Char.MoveDestX = localDestX
	c.Char.MoveDestY = localDestY
	c.Char.TargetID = target
	c.Char.IsMoving = true
	c.Char.MoveLastUpdate = time.Now()
	setCharacterFacingTowardsLocal(c.Char, localFromX, localFromY, localDestX, localDestY)
}

func finishCharacterMovement(c *Client, localCurrentX float32, localCurrentY float32, localEndX float32, localEndY float32, target int16) {
	if c == nil || c.Char == nil {
		return
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localEndX + offset
	c.Char.Y = localEndY + offset
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.TargetID = target
	c.Char.IsMoving = false
	c.Char.MoveLastUpdate = time.Time{}
	setCharacterFacingTowardsLocal(c.Char, localCurrentX, localCurrentY, localEndX, localEndY)
}

func preserveCharacterMovementAfterZeroEnd(c *Client, localCurrentX float32, localCurrentY float32, target int16) {
	if c == nil || c.Char == nil {
		return
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localCurrentX + offset
	c.Char.Y = localCurrentY + offset
	c.Char.TargetID = target
	c.Char.IsMoving = true
	c.Char.MoveLastUpdate = time.Now()
}

func shouldPreserveMovementOnZeroStart(chr *Character, localFromX float32, localFromY float32, localDestX float32, localDestY float32) bool {
	if chr == nil || !chr.IsMoving {
		return false
	}
	if !validLocalPosition(localFromX, localFromY) || !validLocalPosition(localDestX, localDestY) {
		return false
	}
	if !validLocalPosition(chr.MoveDestX, chr.MoveDestY) {
		return false
	}

	startDelta := characterDistance2D(localFromX, localFromY, localDestX, localDestY)
	if startDelta > zeroEndMoveTolerance {
		return false
	}

	remaining := characterDistance2D(localFromX, localFromY, chr.MoveDestX, chr.MoveDestY)
	return remaining > activeMoveDestTolerance
}

func preserveCharacterFacingAfterZeroEnd(c *Client, localCurrentX float32, localCurrentY float32, target int16) {
	if c == nil || c.Char == nil {
		return
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localCurrentX + offset
	c.Char.Y = localCurrentY + offset
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.TargetID = target
	c.Char.IsMoving = false
	c.Char.MoveLastUpdate = time.Time{}
}

func shouldPreserveMovementOnZeroEnd(chr *Character, localCurrentX float32, localCurrentY float32, localEndX float32, localEndY float32) bool {
	if chr == nil || !chr.IsMoving {
		return false
	}
	if !validLocalPosition(localCurrentX, localCurrentY) || !validLocalPosition(localEndX, localEndY) {
		return false
	}
	if !validLocalPosition(chr.MoveDestX, chr.MoveDestY) {
		return false
	}

	endDelta := characterDistance2D(localCurrentX, localCurrentY, localEndX, localEndY)
	if endDelta > zeroEndMoveTolerance {
		return false
	}

	remaining := characterDistance2D(localEndX, localEndY, chr.MoveDestX, chr.MoveDestY)
	return remaining > activeMoveDestTolerance
}

func shouldResumeMovementOnPrematureEnd(chr *Character, localEndX float32, localEndY float32) bool {
	if chr == nil || !chr.IsMoving {
		return false
	}
	if !validLocalPosition(localEndX, localEndY) || !validLocalPosition(chr.MoveDestX, chr.MoveDestY) {
		return false
	}
	return characterDistance2D(localEndX, localEndY, chr.MoveDestX, chr.MoveDestY) > activeMoveDestTolerance
}

func isZeroLengthEndMove(localCurrentX float32, localCurrentY float32, localEndX float32, localEndY float32) bool {
	return characterDistance2D(localCurrentX, localCurrentY, localEndX, localEndY) <= zeroEndMoveTolerance
}

func isStandingJumpEndMove(localCurrentX float32, localCurrentY float32, localEndX float32, localEndY float32) bool {
	return characterDistance2D(localCurrentX, localCurrentY, localEndX, localEndY) <= standingJumpMoveLimit
}

func advanceCharacterMovement(c *Client) bool {
	if c == nil || c.Char == nil || !c.Char.IsMoving {
		return false
	}
	if c.Char.RunSpeed <= 0 {
		c.Char.RunSpeed = defaultCharacterRunSpeed
	}

	now := time.Now()
	if c.Char.MoveLastUpdate.IsZero() {
		c.Char.MoveLastUpdate = now
		return false
	}

	elapsed := now.Sub(c.Char.MoveLastUpdate)
	if elapsed <= 0 {
		return false
	}

	localX := asda2X(c.Char.X, c.Char.MapID)
	localY := asda2Y(c.Char.Y, c.Char.MapID)
	dx := c.Char.MoveDestX - localX
	dy := c.Char.MoveDestY - localY
	dist := float32(math.Hypot(float64(dx), float64(dy)))
	if dist <= 0.001 {
		finishCharacterMovement(c, localX, localY, c.Char.MoveDestX, c.Char.MoveDestY, c.Char.TargetID)
		return true
	}

	step := c.Char.RunSpeed * float32(elapsed.Milliseconds()) / moveSpeedTimeDivisor
	if step <= 0 {
		return false
	}

	if step >= dist {
		finishCharacterMovement(c, localX, localY, c.Char.MoveDestX, c.Char.MoveDestY, c.Char.TargetID)
		return true
	}

	ratio := step / dist
	localX += dx * ratio
	localY += dy * ratio
	offset := mapOffset(c.Char.MapID)
	c.Char.X = localX + offset
	c.Char.Y = localY + offset
	c.Char.MoveLastUpdate = now
	return true
}

func updateCharacterMovement(c *Client) {
	if advanceCharacterMovement(c) {
		World.RefreshCharacterVisibility(c)
		if npcServerClient != nil {
			World.RefreshNpcServerVisibility(c, false)
		} else {
			World.RefreshNpcVisibility(c)
			World.RefreshMonsterVisibility(c)
		}
		World.RefreshPortalVisibility(c)
		World.CheckPortalTriggers(c)
	}
}

func stopCharacterMovementAtCurrent(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	advanceCharacterMovement(c)
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.IsMoving = false
	c.Char.MoveLastUpdate = time.Time{}
}

func prepareCharacterRespawnAtCurrent(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	stopCharacterMovementAtCurrent(c)

	localX := asda2X(c.Char.X, c.Char.MapID)
	localY := asda2Y(c.Char.Y, c.Char.MapID)
	if !validLocalPosition(localX, localY) {
		return
	}

	offset := mapOffset(c.Char.MapID)
	c.Char.X = localX + offset
	c.Char.Y = localY + offset
	c.Char.MoveDestX = 0
	c.Char.MoveDestY = 0
	c.Char.TargetID = -1
	c.Char.IsMoving = false
	c.Char.MoveLastUpdate = time.Time{}
}

func sendInstantMovementToArea(c *Client, reason string) {
	if c == nil || c.Char == nil {
		return
	}
	recipients := World.AreaRecipients(c, true)
	p := buildStartMoveCommonPacket(c.Char, true, -1)
	debugVisibilityf("move instant source=%s reason=%s recipients=%s",
		clientDebugLabel(c), reason, clientListDebugLabel(recipients))
	for _, receiver := range recipients {
		receiver.Send(p)
	}
}

func buildStartMoveCommonPacket(chr *Character, instant bool, targetID int16) *PacketOut {
	p := NewPacket(StartMoveCommon)
	p.WriteUint8(1)
	p.WriteInt16(chr.SessionID)
	p.WriteUint32(chr.AccID)
	p.WriteInt16(2)

	fromX := asda2X(chr.X, chr.MapID)
	fromY := asda2Y(chr.Y, chr.MapID)
	toX := chr.MoveDestX
	toY := chr.MoveDestY
	speed := chr.RunSpeed
	if instant || !chr.IsMoving {
		fromX, fromY, toX, toY = characterFacingCorrectionPath(chr, fromX, fromY)
		speed = 5
	}
	if speed <= 0 {
		speed = defaultCharacterRunSpeed
	}

	p.WriteFloat32(fromX * 100)
	p.WriteFloat32(fromY * 100)
	p.WriteFloat32(toX * 100)
	p.WriteFloat32(toY * 100)
	p.WriteFloat32(speed)
	p.WriteInt16(targetID)
	return p
}

func sendStartMoveCommonToClient(receiver *Client, moving *Client, instant bool) {
	if receiver == nil || receiver.Char == nil || moving == nil || moving.Char == nil {
		return
	}
	advanceCharacterMovement(moving)
	targetID := movementAreaTargetID(moving, moving.Char.TargetID)
	receiver.Send(buildStartMoveCommonPacket(moving.Char, instant, targetID))
	debugVisibilityf("move sync viewer=%s source=%s instant=%t target=%d",
		clientDebugLabel(receiver), clientDebugLabel(moving), instant, targetID)
}

func sendMovingCharacterAfterVisible(viewer *Client, moving *Client) {
	if viewer == nil || viewer.Char == nil || moving == nil || moving.Char == nil || !moving.Char.IsMoving {
		return
	}

	gm := World.GetMap(viewer.Char.MapID)
	if gm == nil {
		sendStartMoveCommonToClient(viewer, moving, false)
		return
	}

	gm.CallDelayed(200, func() {
		if viewer.Char == nil || moving.Char == nil || !moving.Char.IsMoving {
			return
		}
		if viewer.Channel != moving.Channel || viewer.Char.MapID != moving.Char.MapID {
			return
		}
		if !gm.KnowsCharacter(viewer, moving) || !charactersCanSee(viewer, moving) {
			return
		}
		sendStartMoveCommonToClient(viewer, moving, false)
	})
}

func sendFacingDirectionAfterVisible(viewer *Client, visible *Client) {
	if viewer == nil || viewer.Char == nil || visible == nil || visible.Char == nil || visible.Char.IsMoving {
		return
	}

	gm := World.GetMap(viewer.Char.MapID)
	if gm == nil {
		sendStartMoveCommonToClient(viewer, visible, true)
		return
	}

	gm.CallDelayed(100, func() {
		if viewer.Char == nil || visible.Char == nil || visible.Char.IsMoving {
			return
		}
		if viewer.Channel != visible.Channel || viewer.Char.MapID != visible.Char.MapID {
			return
		}
		if !gm.KnowsCharacter(viewer, visible) || !charactersCanSee(viewer, visible) {
			return
		}
		sendStartMoveCommonToClient(viewer, visible, true)
	})
}

func normalizeLocalCoordinate(coordinate float32) float32 {
	if math.IsNaN(float64(coordinate)) || math.IsInf(float64(coordinate), 0) {
		return coordinate
	}
	if math.Abs(float64(coordinate)) > 1000 {
		return coordinate / 100
	}
	return coordinate
}

func validLocalCoordinate(coordinate float32) bool {
	return !math.IsNaN(float64(coordinate)) &&
		!math.IsInf(float64(coordinate), 0) &&
		coordinate >= 0 &&
		coordinate <= 512
}

func validLocalPosition(x float32, y float32) bool {
	return validLocalCoordinate(x) && validLocalCoordinate(y)
}

func characterDistance2D(fromX float32, fromY float32, toX float32, toY float32) float32 {
	return float32(math.Hypot(float64(toX-fromX), float64(toY-fromY)))
}

func setCharacterFacingTowardsLocal(chr *Character, fromX float32, fromY float32, toX float32, toY float32) {
	if chr == nil || !validLocalPosition(fromX, fromY) || !validLocalPosition(toX, toY) {
		return
	}
	dx := toX - fromX
	dy := toY - fromY
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.001 {
		return
	}
	chr.LastFacingX = dx / length
	chr.LastFacingY = dy / length
	chr.Orientation = float32(math.Atan2(float64(dy), float64(dx)))
}

func characterFacingVector(chr *Character) (float32, float32) {
	if chr == nil {
		return 1, 0
	}
	length := float32(math.Hypot(float64(chr.LastFacingX), float64(chr.LastFacingY)))
	if length >= 0.001 {
		return chr.LastFacingX / length, chr.LastFacingY / length
	}
	return float32(math.Cos(float64(chr.Orientation))), float32(math.Sin(float64(chr.Orientation)))
}

func characterFacingTarget(chr *Character, localX float32, localY float32) (float32, float32) {
	facingX, facingY := characterFacingVector(chr)
	targetX := localX + facingX*0.1
	targetY := localY + facingY*0.1
	if !validLocalPosition(targetX, targetY) {
		return localX, localY
	}
	return targetX, targetY
}

func characterFacingCorrectionPath(chr *Character, localX float32, localY float32) (float32, float32, float32, float32) {
	facingX, facingY := characterFacingVector(chr)
	fromX := localX - facingX*0.1
	fromY := localY - facingY*0.1
	if !validLocalPosition(fromX, fromY) {
		fromX = localX
		fromY = localY
	}
	return fromX, fromY, localX, localY
}
