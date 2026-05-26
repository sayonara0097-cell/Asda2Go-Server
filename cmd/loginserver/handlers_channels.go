package main

import (
	"log"

	"asda2/shared/relay"
)

func handleSelectServerRequest(c *Client, p *PacketIn) {
	sendChanelInfoResponse(c, nil)
}

func sendChanelInfoResponse(c *Client, selectedChannel *byte) {
	resp := NewPacket(ChanelInfoResponse)
	writeChanelInfo(resp, selectedChannel)
	c.Send(resp)
}

func writeChanelInfo(p *PacketOut, selectedChannel *byte) {
	counts, err := relay.LoadChannelPlayerCounts(relay.ChannelHeartbeatMaxAge)
	if err != nil {
		log.Printf("[Channel] failed to load channel player counts: %v", err)
	}

	for i := 0; i < relay.GameChannelCount; i++ {
		p.WriteUint8(1)
		p.WriteInt16(1)
		for channel := 0; channel < relay.GameChannelCount; channel++ {
			value := int16(clampChannelPlayerCount(counts[channel]))
			if selectedChannel != nil && *selectedChannel != byte(channel) {
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

func clampChannelPlayerCount(count int) int {
	if count < 0 {
		return 0
	}
	if count > 32767 {
		return 32767
	}
	return count
}
