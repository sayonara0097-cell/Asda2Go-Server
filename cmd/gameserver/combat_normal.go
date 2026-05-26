package main

import (
	"encoding/binary"
	"log"
	"math/rand"

	"asda2/shared/types"
)

const (
	normalAttackStatusFail                 byte = 0
	normalAttackStatusOK                   byte = 1
	normalAttackStatusInv90Prc             byte = 2
	normalAttackStatusDontHaveEnoughArrows byte = 7
	normalAttackStatusCannotAttackShovel   byte = 12

	equipmentSlotWeapon int16 = 9
	equipmentSlotAmmo   int16 = 10

	normalAttackMaxWeightPrc = 90

	defaultWeaponMinDamage int32 = 1
	defaultWeaponMaxDamage int32 = 3
)

type normalAttackHit struct {
	Damage int32
	Min    int32
	Max    int32
}

func resolveStartAttackTarget(c *Client, p *PacketIn) (uint16, *Monster) {
	candidates := startAttackTargetCandidates(p)
	for _, targetID := range candidates {
		if target := currentMapMonsterByClientTarget(c, targetID); target != nil {
			return targetID, target
		}
	}
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return 0, nil
}

func startAttackTargetCandidates(p *PacketIn) []uint16 {
	if p == nil {
		return nil
	}
	seen := make(map[uint16]struct{})
	out := make([]uint16, 0, 4)
	add := func(offset int) {
		if offset < 0 || offset+2 > len(p.Data) {
			return
		}
		value := binary.LittleEndian.Uint16(p.Data[offset:])
		if value == 0 {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	// WCell reads UInt16 at the packet cursor. Some captured clients include a
	// preamble before that field in our raw payload, so keep the known offset as
	// the first candidate and fall back to the direct UInt16 layout.
	add(28)
	add(0)
	for offset := 2; offset+2 <= len(p.Data); offset += 2 {
		add(offset)
	}
	return out
}

func normalAttackStartStatus(c *Client, target *Monster) byte {
	if c == nil || c.Char == nil || c.Char.HP <= 0 {
		return normalAttackStatusFail
	}

	status := normalAttackStatusFail
	if target == nil || target.State != MonsterStateOK || target.Health <= 0 {
		status = normalAttackStatusInv90Prc
	}
	if target != nil && target.State == MonsterStateOK && target.Health > 0 && clientCanSeeMonster(c, target) {
		status = normalAttackStatusOK
	}
	if characterCarriedWeightPrc(c.Char) >= normalAttackMaxWeightPrc {
		status = normalAttackStatusInv90Prc
	}
	if !normalAttackHasRequiredAmmo(c.Char) {
		status = normalAttackStatusDontHaveEnoughArrows
	}
	if normalAttackUsesShovel(c.Char) {
		status = normalAttackStatusCannotAttackShovel
	}
	return status
}

func normalAttackContinueStatus(c *Client, target *Monster) byte {
	if c == nil || c.Char == nil || c.Char.HP <= 0 {
		return normalAttackStatusFail
	}
	if target == nil || target.State != MonsterStateOK || target.Health <= 0 {
		return normalAttackStatusFail
	}
	if !clientCanSeeMonster(c, target) {
		return normalAttackStatusFail
	}
	if normalAttackUsesShovel(c.Char) {
		return normalAttackStatusCannotAttackShovel
	}
	if characterCarriedWeightPrc(c.Char) >= normalAttackMaxWeightPrc {
		return normalAttackStatusInv90Prc
	}
	if !normalAttackHasRequiredAmmo(c.Char) {
		return normalAttackStatusDontHaveEnoughArrows
	}
	return normalAttackStatusOK
}

func applyNormalMonsterAttackPulse(c *Client, target *Monster, source string) (normalAttackHit, bool, byte) {
	if c == nil || c.Char == nil || target == nil {
		return normalAttackHit{}, false, normalAttackStatusFail
	}
	status := normalAttackContinueStatus(c, target)
	if status != normalAttackStatusOK {
		return normalAttackHit{}, false, status
	}
	gm := World.GetMap(c.Char.MapID)
	if gm == nil {
		return normalAttackHit{}, false, normalAttackStatusFail
	}

	hit := rollNormalAttackHit(c.Char)
	killed := gm.DamageMonster(c, target, hit.Damage)
	consumeNormalAttackAmmo(c)
	log.Printf("[Combat] %q hit monster source=%s session=%d world=%d entry=%d damage=%d hp=%d killed=%t",
		c.Char.Name, source, target.SessionID, target.WorldEntityID, target.EntryID, hit.Damage, target.Health, killed)
	return hit, killed, normalAttackStatusOK
}

func rollNormalAttackHit(chr *Character) normalAttackHit {
	minDamage, maxDamage, _ := normalAttackDamageRange(chr)
	damage := minDamage
	if maxDamage > minDamage {
		damage += rand.Int31n(maxDamage - minDamage + 1)
	}
	return normalAttackHit{
		Damage: damage,
		Min:    minDamage,
		Max:    maxDamage,
	}
}

func equippedWeapon(chr *Character) *ItemRow {
	return findItem(chr, types.InventoryEquipment, equipmentSlotWeapon)
}

func equippedAmmo(chr *Character) *ItemRow {
	return findItem(chr, types.InventoryEquipment, equipmentSlotAmmo)
}

func itemCategory(item *ItemRow) int {
	if item == nil {
		return 0
	}
	return itemTemplateByID(item.ItemID).Category
}

func normalAttackUsesShovel(chr *Character) bool {
	return itemCategory(equippedWeapon(chr)) == types.ItemCategoryShowel
}

func normalAttackUsesAmmo(chr *Character) bool {
	switch itemCategory(equippedWeapon(chr)) {
	case types.ItemCategoryBow, types.ItemCategoryCrossbow, types.ItemCategoryBallista:
		return true
	default:
		return false
	}
}

func normalAttackHasRequiredAmmo(chr *Character) bool {
	if !normalAttackUsesAmmo(chr) {
		return true
	}
	ammo := equippedAmmo(chr)
	if ammo == nil || ammo.Amount <= 0 {
		return false
	}
	ammoCategory := itemCategory(ammo)
	switch itemCategory(equippedWeapon(chr)) {
	case types.ItemCategoryBow:
		return ammoCategory == types.ItemCategoryBowAmmo
	case types.ItemCategoryCrossbow:
		return ammoCategory == types.ItemCategoryCrossbowAmmo ||
			ammo.ItemID == 20566 || ammo.ItemID == 20567 || ammo.ItemID == 20568
	case types.ItemCategoryBallista:
		return ammo.ItemID == 20569 || ammo.ItemID == 20570 || ammo.ItemID == 20571
	default:
		return true
	}
}

func consumeNormalAttackAmmo(c *Client) {
	if c == nil || c.Char == nil || !normalAttackUsesAmmo(c.Char) {
		return
	}
	ammo := equippedAmmo(c.Char)
	if ammo == nil || ammo.Amount <= 0 {
		return
	}
	ammo.Amount--
	if ammo.Amount < 0 {
		ammo.Amount = 0
	}
	if err := SaveItem(ammo); err != nil {
		log.Printf("[Combat] save ammo guid=%d item=%d: %v", ammo.Guid, ammo.ItemID, err)
	}
	sendUpdateAmmoResponse(c, ammo)
}

func sendUpdateAmmoResponse(c *Client, ammo *ItemRow) {
	if c == nil {
		return
	}
	p := NewPacket(UpdateAmmoResponse)
	if ammo == nil {
		p.WriteUint8(0)
		p.WriteInt32(0)
	} else {
		p.WriteUint8(1)
		p.WriteInt32(int32(ammo.Amount))
	}
	c.Send(p)
}

func characterCarriedWeightPrc(chr *Character) int {
	max := maxWeight(chr)
	if max <= 0 {
		return 0
	}
	return carriedWeight(chr) * 100 / max
}
