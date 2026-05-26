package main

// ---- War Shop ----

func handleByuItemFromWarshop(c *Client, p *PacketIn) {
	handleBuyItemWithVendor(c, p, false)
	pkt := NewPacket(ItemFromWarshopBuyed)
	pkt.WriteUint8(itemStatusOK)
	pkt.WriteInt32(clampInt32(c.Char.Gold))
	c.Send(pkt)
}
