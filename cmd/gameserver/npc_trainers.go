package main

import "asda2/shared/types"

func activeTrainerNpc(c *Client, skill SkillTemplate) (*Npc, bool) {
	if c == nil || c.Char == nil {
		return nil, false
	}

	state, ok := currentNpcInteraction(c)
	if ok && state.Kind == types.NpcInteractionTrainer {
		npc := resolveNpcInteractionNpc(c, state)
		if npc != nil && npcInteractionKind(npc) == types.NpcInteractionTrainer && clientCanInteractWithNpc(c, npc) {
			return npc, true
		}
		clearNpcInteraction(c)
	}

	npc := nearestInteractableTrainer(c, skill)
	if npc == nil {
		return nil, false
	}
	rememberNpcInteraction(c, npc, uint16(npc.SessionID), types.NpcInteractionTrainer)
	debugNpcInteractionf("inferred trainer char=%q session=%d entry=%d classGroup=%d",
		c.Char.Name, npc.SessionID, npc.EntryID, npc.ClassGroup)
	return npc, true
}

func nearestInteractableTrainer(c *Client, skill SkillTemplate) *Npc {
	if c == nil || c.Char == nil {
		return nil
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return nil
	}

	playerX := float64(asda2X(c.Char.X, c.Char.MapID))
	playerY := float64(asda2Y(c.Char.Y, c.Char.MapID))
	var nearest *Npc
	nearestDistance := 0.0
	nearestScore := -1
	for _, npc := range gm.Npcs() {
		if npc == nil || npcInteractionKind(npc) != types.NpcInteractionTrainer || !clientCanInteractWithNpc(c, npc) {
			continue
		}
		score := trainerMatchScore(npc, c.Char, skill)
		distance := distance2D(float64(npc.LocalX), float64(npc.LocalY), playerX, playerY)
		if nearest == nil || score > nearestScore || score == nearestScore && distance < nearestDistance {
			nearest = npc
			nearestDistance = distance
			nearestScore = score
		}
	}
	return nearest
}

func trainerMatchScore(npc *Npc, chr *Character, skill SkillTemplate) int {
	score := 0
	if trainerMatchesCharacter(npc, chr) {
		score += 2
	}
	if trainerCanTeachSkill(npc, skill) {
		score++
	}
	return score
}
