package main

// ---- Premium Items ----

func handleChangeNameRequest(c *Client, p *PacketIn) {
	pkt := NewPacket(ChangeNameResponse)
	pkt.WriteUint8(itemStatusFail)
	c.Send(pkt)
}

func handleTransformToPet(c *Client, p *PacketIn) {
	pkt := NewPacket(TransformToPet)
	pkt.WriteUint8(itemStatusFail)
	c.Send(pkt)
}
