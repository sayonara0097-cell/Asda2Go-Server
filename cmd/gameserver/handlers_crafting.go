package main

import (
	"encoding/binary"
	"log"
	"math"
	"math/rand"

	"asda2/shared/types"
)

const recipeFlagCount = types.LearnedRecipeFlagCount

var (
	craftRecipesByID = map[int]CraftRecipeRow{}
	craftRandFloat   = rand.Float64
)

var recipeLearnedPad = []byte{
	255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0,
	255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}

func initCraftingRuntime() error {
	rows, err := LoadCraftRecipes()
	if err != nil {
		return err
	}
	craftRecipesByID = make(map[int]CraftRecipeRow, len(rows))
	for _, row := range rows {
		if row.RecipeID <= 0 {
			continue
		}
		craftRecipesByID[row.RecipeID] = row
	}
	log.Printf("[Crafting] %d recipes loaded", len(craftRecipesByID))
	return nil
}

func handleLearnRecipe(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 2 {
		sendRecipeLearned(c, false, 0, nil)
		return
	}
	ensureCraftingLevel(c.Char)
	slot := int16(binary.LittleEndian.Uint16(p.Data))
	item := findItem(c.Char, types.InventoryRegular, slot)
	if item == nil {
		sendRecipeLearned(c, false, 0, nil)
		return
	}
	if itemLockedForTrading(c.Char, item) {
		sendRecipeLearned(c, false, 0, item)
		return
	}
	templ := itemTemplateByID(item.ItemID)
	if templ.Kind != types.ItemKindRecipe && templ.Category != types.ItemCategoryRecipe {
		sendRecipeLearned(c, false, 0, item)
		return
	}
	recipeID := templ.ValueOnUse
	if recipeID <= 0 {
		sendRecipeLearned(c, false, 0, item)
		return
	}
	recipe, ok := craftRecipesByID[recipeID]
	if !ok || len(recipe.Results) == 0 || c.Char.CraftingLevel < recipe.RequiredCraftingLevel {
		sendRecipeLearned(c, false, int16(recipeID), item)
		return
	}
	if recipeID >= recipeFlagCount || c.Char.LearnedRecipes[recipeID] {
		sendRecipeLearned(c, false, int16(recipeID), item)
		return
	}
	if err := removeCharacterItem(c.Char, item, 1); err != nil {
		sendRecipeLearned(c, false, int16(recipeID), item)
		return
	}
	c.Char.LearnedRecipes[recipeID] = true
	if err := SaveCharacter(c.Char); err != nil {
		log.Printf("[Crafting] save learned recipe char=%q recipe=%d: %v", c.Char.Name, recipeID, err)
	}
	sendRecipeLearned(c, true, int16(recipeID), item)
	sendLearnedRecipes(c)
}

func handleCraft(c *Client, p *PacketIn) {
	if c.Char == nil || len(p.Data) < 2 {
		sendCrafted(c, false, 0, nil, nil)
		return
	}
	ensureCraftingLevel(c.Char)
	recipeID := int(binary.LittleEndian.Uint16(p.Data))
	crafted, materials, ok := craftItem(c.Char, recipeID)
	if !ok {
		sendCrafted(c, false, 0, nil, nil)
		return
	}
	craftTimes := craftedItemCraftTimes(crafted)
	sendCrafted(c, true, craftTimes, crafted, materials)
}

func craftItem(chr *Character, recipeID int) (*ItemRow, []*ItemRow, bool) {
	if recipeID <= 0 || recipeID >= recipeFlagCount || !chr.LearnedRecipes[recipeID] {
		return nil, nil, false
	}
	recipe, ok := craftRecipesByID[recipeID]
	if !ok || len(recipe.Results) == 0 || chr.CraftingLevel < recipe.RequiredCraftingLevel {
		return nil, nil, false
	}
	materials, amounts, ok := reserveCraftMaterials(chr, recipe.Materials)
	if !ok {
		return nil, nil, false
	}
	result := recipe.Results[craftedResultIndex(recipe)]
	credit := removedMaterialWeight(materials, amounts)
	crafted, status, err := createCharacterItemDetailed(chr, result.ItemID, result.Amount, types.InventoryRegular, -1, nil, credit)
	if err != nil || status != inventoryStatusOK || crafted == nil {
		log.Printf("[Crafting] create result failed char=%q recipe=%d status=%d err=%v", chr.Name, recipeID, status, err)
		return nil, nil, false
	}
	usedMaterials := make([]*ItemRow, 0, len(materials))
	for i, material := range materials {
		if material == nil {
			continue
		}
		usedMaterials = append(usedMaterials, material)
		if err := removeCharacterItem(chr, material, amounts[i]); err != nil {
			log.Printf("[Crafting] remove material failed char=%q recipe=%d item=%d: %v", chr.Name, recipeID, material.ItemID, err)
			return nil, nil, false
		}
	}
	crafted.IsCrafted = true
	generateCraftedEquipmentOption(crafted)
	applyCraftingExp(chr, recipe)
	if err := SaveItem(crafted); err != nil {
		log.Printf("[Crafting] save crafted item failed char=%q recipe=%d: %v", chr.Name, recipeID, err)
	}
	if err := SaveCharacter(chr); err != nil {
		log.Printf("[Crafting] save character failed char=%q recipe=%d: %v", chr.Name, recipeID, err)
	}
	return crafted, usedMaterials, true
}

func reserveCraftMaterials(chr *Character, requirements []CraftMaterialRow) ([]*ItemRow, []int, bool) {
	materials := make([]*ItemRow, 0, len(requirements))
	amounts := make([]int, 0, len(requirements))
	for _, req := range requirements {
		if req.ItemID <= 0 || req.Amount <= 0 {
			continue
		}
		remaining := req.Amount
		for _, item := range inventoryItems(chr, types.InventoryRegular) {
			if item == nil || item.ItemID != req.ItemID || item.Amount <= 0 {
				continue
			}
			if itemLockedForTrading(chr, item) {
				continue
			}
			reserved := 0
			for i, material := range materials {
				if material == item {
					reserved = amounts[i]
					break
				}
			}
			available := item.Amount - reserved
			if available <= 0 {
				continue
			}
			use := available
			if use > remaining {
				use = remaining
			}
			found := false
			for i, material := range materials {
				if material == item {
					amounts[i] += use
					found = true
					break
				}
			}
			if !found {
				materials = append(materials, item)
				amounts = append(amounts, use)
			}
			remaining -= use
			if remaining <= 0 {
				break
			}
		}
		if remaining > 0 {
			return nil, nil, false
		}
	}
	return materials, amounts, true
}

func craftedResultIndex(recipe CraftRecipeRow) int {
	if len(recipe.Results) <= 1 {
		return 0
	}
	rarity := craftedRarity()
	if rarity <= 0 {
		rarity = 1
	}
	if rarity > len(recipe.Results) {
		rarity = len(recipe.Results)
	}
	return rarity - 1
}

func craftedRarity() int {
	for attempts := 0; attempts < 8; attempts++ {
		num := int(craftRandFloat() * 100000)
		switch {
		case num < 15000:
			continue
		case num < 50000:
			return 1
		case num < 85000:
			return 2
		case num < 98000:
			return 3
		case num < 99900:
			return 4
		default:
			return 5
		}
	}
	return 1
}

func applyCraftingExp(chr *Character, recipe CraftRecipeRow) {
	diff := int(chr.CraftingLevel) - int(recipe.RequiredCraftingLevel)
	if diff < 0 {
		return
	}
	gained := calcCraftingExp(diff, chr.CraftingLevel)
	chr.CraftingExp += gained
	for chr.CraftingExp >= 100 && chr.CraftingLevel < 255 {
		chr.CraftingLevel++
		chr.CraftingExp -= 100
	}
}

func calcCraftingExp(diff int, currentLevel byte) float32 {
	var base float64
	switch diff {
	case 0:
		base = 1
	case 1:
		base = 0.5
	case 2:
		base = 0.25
	case 3:
		base = 0.1
	case 4:
		base = 0.05
	case 5:
		base = 0.01
	case 6:
		base = 0.005
	case 7:
		base = 0.001
	default:
		base = 0
	}
	if currentLevel == 0 {
		currentLevel = 1
	}
	return float32(base / math.Pow(float64(currentLevel), 2))
}

func removedMaterialWeight(materials []*ItemRow, amounts []int) int {
	total := 0
	for i, material := range materials {
		if material == nil || i >= len(amounts) {
			continue
		}
		total += itemUnitWeight(material) * amounts[i]
	}
	return total
}

func craftedItemCraftTimes(item *ItemRow) byte {
	if item == nil {
		return 0
	}
	templ := itemTemplateByID(item.ItemID)
	if !isEquipmentTemplate(templ) {
		return 1
	}
	return byte(templ.Quality) + 1
}

func ensureCraftingLevel(chr *Character) {
	if chr != nil && chr.CraftingLevel == 0 {
		chr.CraftingLevel = 1
	}
}

func syncLearnedRecipesFromInventory(chr *Character) bool {
	if chr == nil {
		return false
	}
	changed := false
	for _, item := range inventoryItems(chr, types.InventoryRegular) {
		if item == nil {
			continue
		}
		templ := itemTemplateByID(item.ItemID)
		if templ.Category != types.ItemCategoryRecipe && templ.Kind != types.ItemKindRecipe {
			continue
		}
		recipeID := templ.ValueOnUse
		if recipeID <= 0 || recipeID >= recipeFlagCount || chr.LearnedRecipes[recipeID] {
			continue
		}
		chr.LearnedRecipes[recipeID] = true
		changed = true
	}
	return changed
}

func sendLearnedRecipes(c *Client) {
	if c == nil || c.Char == nil {
		return
	}
	ensureCraftingLevel(c.Char)
	if syncLearnedRecipesFromInventory(c.Char) {
		if err := SaveCharacter(c.Char); err != nil {
			log.Printf("[Crafting] save synced recipes char=%q: %v", c.Char.Name, err)
		}
	}
	p := NewPacket(LeanedRecipes)
	p.WriteInt16(int16(types.LearnedRecipeCount(c.Char.LearnedRecipes)))
	p.WriteInt16(1000)
	for i := 0; i < recipeFlagCount; i++ {
		if c.Char.LearnedRecipes[i] {
			p.WriteUint8(1)
		} else {
			p.WriteUint8(0)
		}
	}
	p.WriteInt32(0)
	p.WriteUint8(c.Char.CraftingLevel)
	p.WriteInt32(0)
	p.WriteInt16(int16(c.Char.CraftingExp))
	p.WriteInt16(0)
	c.Send(p)
}

func sendRecipeLearned(c *Client, success bool, recipeID int16, item *ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(RecipeLeadned)
	if success {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteInt32(int32(itemWeight(c.Char)))
	if item == nil {
		p.WriteInt32(0)
		p.WriteUint8(types.InventoryRegular)
		p.WriteInt16(0)
		p.WriteInt16(0)
		p.WriteInt32(0)
		p.WriteUint8(0)
		p.WriteInt16(0)
	} else {
		p.WriteInt32(int32(item.ItemID))
		p.WriteUint8(types.InventoryRegular)
		p.WriteInt16(item.Slot)
		if item.Amount <= 0 {
			p.WriteInt16(-1)
		} else {
			p.WriteInt16(0)
		}
		p.WriteInt32(int32(item.Amount))
		p.WriteUint8(0)
		p.WriteInt16(int16(item.Amount))
	}
	p.WriteBytes(recipeLearnedPad)
	p.WriteInt16(recipeID)
	c.Send(p)
}

func sendCrafted(c *Client, success bool, craftTimes byte, crafted *ItemRow, materials []*ItemRow) {
	if c == nil || c.Char == nil {
		return
	}
	p := NewPacket(Crafted)
	if success {
		p.WriteUint8(1)
	} else {
		p.WriteUint8(0)
	}
	p.WriteInt16(itemWeight(c.Char))
	p.WriteUint8(c.Char.CraftingLevel)
	p.WriteInt32(0)
	p.WriteInt16(int16(c.Char.CraftingExp))
	p.WriteInt16(int16(craftTimes))
	writeItemInfoToPacket(p, crafted, c.Char, false)
	for i := 0; i < 7; i++ {
		var item *ItemRow
		if i < len(materials) {
			item = materials[i]
		}
		writeItemInfoToPacket(p, item, c.Char, false)
	}
	c.Send(p)
}
