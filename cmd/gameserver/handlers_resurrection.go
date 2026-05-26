package main

import "log"

// ---- Resurrection ----

func handleResurectOnDeathPlace(c *Client, p *PacketIn) {
	if c == nil || c.Char == nil || c.Char.HP > 0 {
		return
	}
	prepareCharacterRespawnAtCurrent(c)
	c.Char.MP = c.Char.MaxMP
	healCharacter(c, c.Char.MaxHP, "resurrect_on_death_place")
	sendUpdateStats(c)
	sendUpdateStatsOne(c)
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[DB] failed to save respawn for %q: %v", c.Char.Name, err)
		return
	}
	log.Printf("[Respawn] saved %q map=%d x=%.2f y=%.2f orientation=%.4f",
		c.Char.Name, c.Char.MapID, asda2X(c.Char.X, c.Char.MapID), asda2Y(c.Char.Y, c.Char.MapID), c.Char.Orientation)
}

func handlePreResurect(c *Client, p *PacketIn) { /* TODO: premium/pet resurrection request */ }
