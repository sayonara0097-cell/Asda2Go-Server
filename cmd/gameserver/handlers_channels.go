package main

import (
	"fmt"
	"log"

	"asda2/shared/relay"
)

var channelChangeLocationStub = []byte{45, 155, 171, 4, 0, 0, 0, 0, 36, 33, 109, 21}

func handleChangeChannel(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil {
		return
	}
	targetChannel, ok := readChangeChannelRequest(p, c.Channel)
	if !ok || !relay.ValidGameChannel(targetChannel) {
		log.Printf("[Channel] invalid change request char=%q target=%d", c.Char.Name, targetChannel)
		return
	}

	if targetChannel == c.Channel {
		resetChannelBoundCharacterState(c)
		sendGameChanelInfoResponse(c, targetChannel)
		return
	}

	endpoint := bridge.EndpointForChannel(targetChannel)
	if endpoint.Channel != targetChannel {
		log.Printf("[Channel] no endpoint for channel=%d requested by %q", targetChannel, c.Char.Name)
		sendGameChanelInfoResponse(c, c.Channel)
		return
	}

	oldChannel := c.Channel
	if err := prepareChannelTransfer(c, targetChannel); err != nil {
		log.Printf("[Channel] transfer failed char=%q %d->%d: %v", c.Char.Name, oldChannel, targetChannel, err)
		sendGameChanelInfoResponse(c, oldChannel)
		return
	}

	sendChannelChangeLocationResponse(c, endpoint)
	sendGameChanelInfoResponse(c, targetChannel)
	log.Printf("[Channel] transfer started char=%q %d->%d endpoint=%s:%d", c.Char.Name, oldChannel, targetChannel, endpoint.IP, endpoint.Port)
}

type changeChannelCandidate struct {
	offset  int
	raw     byte
	channel byte
}

func readChangeChannelRequest(p *PacketIn, currentChannel byte) (byte, bool) {
	if p == nil {
		return 0, false
	}
	candidates := make([]changeChannelCandidate, 0, 3)
	for _, offset := range []int{28, 25, 24, 0} {
		if len(p.Data) <= offset {
			continue
		}
		raw := p.Data[offset]
		if channel, ok := normalizeChangeChannelValue(raw); ok {
			candidates = append(candidates, changeChannelCandidate{
				offset:  offset,
				raw:     raw,
				channel: channel,
			})
		}
	}
	if len(candidates) == 0 {
		return 0, false
	}

	if selected, ok := selectChangeChannelCandidate(candidates, currentChannel); ok {
		log.Printf("[Channel] parsed change request offset=%d raw=%d target=%d current=%d payloadLen=%d data=% X",
			selected.offset, selected.raw, selected.channel, currentChannel, len(p.Data), p.Data)
		return selected.channel, true
	}
	return 0, false
}

func normalizeChangeChannelValue(raw byte) (byte, bool) {
	if relay.ValidGameChannel(raw) {
		return raw, true
	}
	if raw >= 1 && int(raw) <= relay.GameChannelCount {
		return raw - 1, true
	}
	return 0, false
}

func selectChangeChannelCandidate(candidates []changeChannelCandidate, currentChannel byte) (changeChannelCandidate, bool) {
	var zeroCandidate changeChannelCandidate
	hasZeroCandidate := false
	var sameChannelCandidate changeChannelCandidate
	hasSameChannelCandidate := false

	for _, candidate := range candidates {
		if candidate.channel == currentChannel {
			if !hasSameChannelCandidate {
				sameChannelCandidate = candidate
				hasSameChannelCandidate = true
			}
			continue
		}
		if candidate.raw != 0 {
			return candidate, true
		}
		if !hasZeroCandidate {
			zeroCandidate = candidate
			hasZeroCandidate = true
		}
	}
	if hasZeroCandidate {
		return zeroCandidate, true
	}
	if hasSameChannelCandidate {
		return sameChannelCandidate, true
	}
	return changeChannelCandidate{}, false
}

func prepareChannelTransfer(c *Client, targetChannel byte) error {
	if c == nil || c.Char == nil {
		return fmt.Errorf("missing character")
	}
	if !relay.ValidGameChannel(targetChannel) {
		return fmt.Errorf("unsupported channel %d", targetChannel)
	}

	advanceCharacterMovement(c)
	resetChannelBoundCharacterState(c)

	accountID := c.Char.AccID
	charNum := c.Char.CharNum
	clientIP := remoteIP(c.Conn.RemoteAddr())
	if err := SaveCharacter(c.Char); err != nil {
		return fmt.Errorf("save character: %w", err)
	}

	if err := relay.SavePendingLogin(relay.PendingLogin{
		AccountID: accountID,
		CharNum:   charNum,
		Channel:   targetChannel,
		ClientIP:  clientIP,
	}); err != nil {
		return fmt.Errorf("save handoff: %w", err)
	}

	if c.AccountSessionToken != "" {
		if err := relay.ForceClaimAccountSession(accountID, c.AccountSessionToken, relay.AccountSessionHandoff, targetChannel, charNum, clientIP); err != nil {
			return fmt.Errorf("promote account session: %w", err)
		}
		if c.StopAccountSession != nil {
			c.StopAccountSession()
			c.StopAccountSession = nil
		}
	}

	World.LeaveMap(c)
	c.Channel = targetChannel
	c.IsTeleporting = true
	return nil
}

func resetChannelBoundCharacterState(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	stopCharacterMovementAtCurrent(c)
	c.Char.IsFighting = false
	c.Char.TargetID = -1
	if gm := World.GetMap(c.Char.MapID); gm != nil {
		gm.ClearMonsterTargetsFor(c.Char.SessionID)
	}
	clearNpcInteraction(c)
}

func sendChannelChangeLocationResponse(c *Client, endpoint relay.ChannelEndpoint) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(ResurectWithChangeLocation)
	p.WriteInt16(c.Char.SessionID)
	p.WriteUint8(byte(c.Char.MapID))
	p.WriteAsdaString(endpoint.IP, 20)
	p.WriteUint16(endpoint.Port)
	p.WriteInt16(int16(asda2X(c.Char.X, c.Char.MapID)))
	p.WriteInt16(int16(asda2Y(c.Char.Y, c.Char.MapID)))
	p.WriteInt32(c.Char.HP)
	p.WriteInt16(int16(c.Char.MP))
	p.WriteBytes(channelChangeLocationStub)
	c.Send(p)
}

func sendGameChanelInfoResponse(c *Client, selectedChannel byte) {
	resp := NewPacket(ChanelInfoResponse)
	writeGameChanelInfo(resp, selectedChannel)
	c.Send(resp)
}

func writeGameChanelInfo(p *PacketOut, selectedChannel byte) {
	counts, err := relay.LoadChannelPlayerCounts(relay.ChannelHeartbeatMaxAge)
	if err != nil {
		log.Printf("[Channel] failed to load channel player counts: %v", err)
		counts[gameChannel] = countGamePlayersOnChannel(gameChannel)
	}

	for i := 0; i < relay.GameChannelCount; i++ {
		p.WriteUint8(1)
		p.WriteInt16(1)
		for channel := 0; channel < relay.GameChannelCount; channel++ {
			value := int16(clampGameChannelPlayerCount(counts[channel]))
			if selectedChannel != byte(channel) {
				value = -1
			}
			p.WriteInt16(value)
		}
		p.WriteInt16(-1)
		p.WriteInt16(-1)
		for j := 0; j < 6; j++ {
			p.WriteInt64(-1)
		}
		p.WriteInt16(-1)
	}
}

func clampGameChannelPlayerCount(count int) int {
	if count < 0 {
		return 0
	}
	if count > 32767 {
		return 32767
	}
	return count
}
