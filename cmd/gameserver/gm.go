package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"

	"asda2/shared/relay"
)

func handleGMCommand(cmd relay.GMCommand) {
	action := strings.ToLower(strings.TrimSpace(cmd.Action))
	switch action {
	case "announce":
		sent := sendWorldAnnouncementChat(cmd.Args["message"])
		log.Printf("[GM] announcement by %s delivered to %d players", cmd.RequestedBy, sent)
	case "teleport":
		if err := gmTeleport(cmd); err != nil {
			log.Printf("[GM] teleport by %s failed: %v", cmd.RequestedBy, err)
		}
	case "add_exp":
		if err := gmAddExp(cmd); err != nil {
			log.Printf("[GM] add_exp by %s failed: %v", cmd.RequestedBy, err)
		}
	case "add_gold":
		if err := gmAddGold(cmd); err != nil {
			log.Printf("[GM] add_gold by %s failed: %v", cmd.RequestedBy, err)
		}
	case "give_item":
		if err := gmGiveItem(cmd); err != nil {
			log.Printf("[GM] give_item by %s failed: %v", cmd.RequestedBy, err)
		}
	case "set_level":
		if err := gmSetLevel(cmd); err != nil {
			log.Printf("[GM] set_level by %s failed: %v", cmd.RequestedBy, err)
		}
	case "set_profession":
		if err := gmSetProfession(cmd); err != nil {
			log.Printf("[GM] set_profession by %s failed: %v", cmd.RequestedBy, err)
		}
	case "set_speed":
		if err := gmSetSpeed(cmd); err != nil {
			log.Printf("[GM] set_speed by %s failed: %v", cmd.RequestedBy, err)
		}
	case "heal_player":
		if err := gmHealPlayer(cmd); err != nil {
			log.Printf("[GM] heal_player by %s failed: %v", cmd.RequestedBy, err)
		}
	case "damage_player":
		if err := gmDamagePlayer(cmd); err != nil {
			log.Printf("[GM] damage_player by %s failed: %v", cmd.RequestedBy, err)
		}
	case "send_packet":
		if err := gmSendPacket(cmd); err != nil {
			log.Printf("[GM] send_packet by %s failed: %v", cmd.RequestedBy, err)
		}
	case "summon_monster":
		if err := gmSummonMonster(cmd); err != nil {
			log.Printf("[GM] summon_monster by %s failed: %v", cmd.RequestedBy, err)
		}
	case "summon_monster_near_player":
		if err := gmSummonMonsterNearPlayer(cmd); err != nil {
			log.Printf("[GM] summon_monster_near_player by %s failed: %v", cmd.RequestedBy, err)
		}
	case "kill_monster":
		if err := gmKillMonster(cmd); err != nil {
			log.Printf("[GM] kill_monster by %s failed: %v", cmd.RequestedBy, err)
		}
	case "reload_monster_spawns":
		if err := gmReloadMonsterSpawns(cmd); err != nil {
			log.Printf("[GM] reload_monster_spawns by %s failed: %v", cmd.RequestedBy, err)
		}
	default:
		log.Printf("[GM] unknown command %q requested by %s args=%v", cmd.Action, cmd.RequestedBy, cmd.Args)
	}
}

func gmTeleport(cmd relay.GMCommand) error {
	targetName := strings.TrimSpace(cmd.Args["character"])
	if targetName == "" {
		return fmt.Errorf("character is required")
	}
	target := getClientByCharacterName(targetName)
	if target == nil || target.Char == nil {
		return fmt.Errorf("character %q is not online on this game server", targetName)
	}

	mapID, err := parseUint16Arg(cmd.Args, "map")
	if err != nil {
		return err
	}
	localX, err := parseInt16Arg(cmd.Args, "x")
	if err != nil {
		return err
	}
	localY, err := parseInt16Arg(cmd.Args, "y")
	if err != nil {
		return err
	}
	originalMapID := mapID
	mapID = normalizeAsda2MapID(mapID)
	if World.GetMap(mapID) == nil {
		return fmt.Errorf("map %d is not registered", originalMapID)
	}

	World.LeaveMap(target)
	offset := mapOffset(mapID)
	target.Char.MapID = mapID
	target.Char.X = offset + float32(localX)
	target.Char.Y = offset + float32(localY)
	target.Char.MoveDestX = 0
	target.Char.MoveDestY = 0
	target.Char.IsMoving = false

	sendGMTeleportResponse(target, mapID, localX, localY)
	World.EnterMap(target)
	if err := SaveCharacter(target.Char); err != nil {
		log.Printf("[GM] failed to save teleported character %q: %v", target.Char.Name, err)
	}
	log.Printf("[GM] %s teleported %q to map=%d x=%d y=%d", cmd.RequestedBy, target.Char.Name, mapID, localX, localY)
	return nil
}

func sendGMTeleportResponse(c *Client, mapID uint16, x int16, y int16) {
	p := NewPacket(TeleportedByCristal)
	p.WriteUint8(1)
	p.WriteAsdaString(gamePublicIP, 20)
	p.WriteUint16(gamePublicPort)
	p.WriteInt16(int16(mapID))
	p.WriteInt16(x)
	p.WriteInt16(y)
	p.WriteUint8(0)
	p.WriteUint8(0)
	p.WriteInt64(-1)
	p.WriteInt64(-1)
	c.Send(p)
}

func gmSendPacket(cmd relay.GMCommand) error {
	targetName := strings.TrimSpace(cmd.Args["character"])
	if targetName == "" {
		return fmt.Errorf("character is required")
	}
	target := getClientByCharacterName(targetName)
	if target == nil {
		return fmt.Errorf("character %q is not online on this game server", targetName)
	}

	opcode, err := parseUint16Arg(cmd.Args, "opcode")
	if err != nil {
		return err
	}
	payloadHex := strings.NewReplacer(" ", "", "-", "", ":", "").Replace(strings.TrimSpace(cmd.Args["payloadHex"]))
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return fmt.Errorf("payloadHex: %w", err)
	}

	p := NewPacket(Opcode(opcode))
	p.WriteBytes(payload)
	target.Send(p)
	log.Printf("[GM] %s sent packet opcode=%d payload=%dB to %q", cmd.RequestedBy, opcode, len(payload), target.Char.Name)
	return nil
}

func parseUint16Arg(args map[string]string, key string) (uint16, error) {
	value := strings.TrimSpace(args[key])
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return uint16(parsed), nil
}

func parseInt16Arg(args map[string]string, key string) (int16, error) {
	value := strings.TrimSpace(args[key])
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return int16(parsed), nil
}

func parseInt64Arg(args map[string]string, key string) (int64, error) {
	value := strings.TrimSpace(args[key])
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}
