package main

import (
	"log"

	"asda2/shared/types"
)

func handleTrainerNpcInteraction(c *Client, npc *Npc, state npcInteractionState) {
	if c == nil || c.Char == nil || npc == nil {
		return
	}
	if !trainerMatchesCharacter(npc, c.Char) {
		log.Printf("[NPC] %q clicked trainer session=%d entry=%d classGroup=%d result=wrong-class",
			c.Char.Name, npc.SessionID, npc.EntryID, npc.ClassGroup)
		return
	}

	sendSkillsInfo(c)
	log.Printf("[NPC] %q opened trainer session=%d entry=%d classGroup=%d expires=%s",
		c.Char.Name, npc.SessionID, npc.EntryID, state.ClassGroup, state.ExpiresAt.Format("15:04:05"))
}

func trainerLearnStatus(c *Client, skillID int16) skillLearnStatus {
	if c == nil || c.Char == nil {
		return skillLearnFail
	}
	skill, ok := skillTemplates[skillID]
	if !ok {
		debugNpcInteractionf("rejected trainer learn char=%q skill=%d reason=unknown-skill", c.Char.Name, skillID)
		return skillLearnFail
	}

	npc, ok := activeTrainerNpc(c, skill)
	if !ok {
		debugNpcInteractionf("rejected trainer learn char=%q skill=%d reason=no-active-trainer", c.Char.Name, skillID)
		return skillLearnFail
	}
	if !trainerMatchesCharacter(npc, c.Char) || !trainerCanTeachSkill(npc, skill) {
		debugNpcInteractionf("rejected trainer learn char=%q skill=%d session=%d entry=%d reason=wrong-class",
			c.Char.Name, skillID, npc.SessionID, npc.EntryID)
		return skillLearnBadProfession
	}
	return skillLearnOK
}

func trainerMatchesCharacter(npc *Npc, chr *Character) bool {
	if npc == nil || chr == nil || npcInteractionKind(npc) != types.NpcInteractionTrainer {
		return false
	}
	if npc.ClassGroup == types.NpcClassGroupAll {
		return true
	}
	return npc.ClassGroup == skillClassGroupForCharacter(chr)
}

func trainerCanTeachSkill(npc *Npc, skill SkillTemplate) bool {
	if npc == nil {
		return false
	}
	if npc.ClassGroup == types.NpcClassGroupAll || skill.ClassGroup == skillClassAll {
		return true
	}
	return npc.ClassGroup == skill.ClassGroup
}
