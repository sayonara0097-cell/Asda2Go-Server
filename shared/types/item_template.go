package types

import "sync"

const (
	InventoryShop byte = iota + 1
	InventoryRegular
	InventoryEquipment
	InventoryQuest
	InventoryWarehouse
	InventoryAvatarWarehouse
)

const (
	RegularInventorySlots         = 60
	ShopInventorySlots            = 61
	EquipmentInventorySlots       = 21
	WarehouseInventorySlots       = 270
	AvatarWarehouseInventorySlots = 270
)

const (
	DefaultShopInventorySlots    = 30
	DefaultWarehouseBagSlots     = 30
	DefaultCharacterMaxWeight    = 15150
	ReservedRegularInventorySlot = 0
)

type ItemKind byte

const (
	ItemKindUnknown ItemKind = iota
	ItemKindMaterial
	ItemKindConsumable
	ItemKindWeapon
	ItemKindArmor
	ItemKindAvatar
	ItemKindAccessory
	ItemKindSowel
	ItemKindRecipe
	ItemKindCurrency
)

type ItemQuality byte

const (
	ItemQualityWhite ItemQuality = iota
	ItemQualityYellow
	ItemQualityPurple
	ItemQualityGreen
	ItemQualityOrange
)

type ItemTemplate struct {
	ItemID             int
	Name               string
	Kind               ItemKind
	Category           int
	Quality            ItemQuality
	AuctionLevel       byte
	InventoryType      byte
	EquipmentSlot      int16
	RequiredLevel      byte
	RequiredProfession byte
	MaxDurability      byte
	Weight             uint16
	BuyPrice           int64
	SellPrice          int64
	MaxStack           int
	IsStackable        bool
	SowelSockets       byte
	ValueOnUse         int
	AttackTime         int
	AttackRange        int16
	SowelBonusType     int
	SowelBonusValue    int
}

var itemTemplates = struct {
	sync.RWMutex
	byID map[int]ItemTemplate
}{
	byID: make(map[int]ItemTemplate),
}

func SetItemTemplates(rows []ItemTemplate) {
	itemTemplates.Lock()
	itemTemplates.byID = make(map[int]ItemTemplate, len(rows))
	for _, row := range rows {
		if row.ItemID <= 0 {
			continue
		}
		normalizeItemTemplate(&row)
		itemTemplates.byID[row.ItemID] = row
	}
	itemTemplates.Unlock()
}

func ItemTemplateByID(itemID int) ItemTemplate {
	itemTemplates.RLock()
	templ, ok := itemTemplates.byID[itemID]
	itemTemplates.RUnlock()
	if ok {
		return templ
	}
	return fallbackItemTemplate(itemID)
}

func normalizeItemTemplate(t *ItemTemplate) {
	if t.InventoryType == 0 {
		t.InventoryType = InventoryRegular
	}
	switch t.Kind {
	case ItemKindWeapon, ItemKindArmor, ItemKindAvatar, ItemKindAccessory:
	default:
		if t.EquipmentSlot == 0 {
			t.EquipmentSlot = -1
		}
	}
	if t.MaxStack <= 0 {
		t.MaxStack = 1
	}
	if t.IsStackable || t.MaxStack > 1 || t.Kind == ItemKindMaterial || t.Kind == ItemKindCurrency {
		t.IsStackable = true
		if t.MaxStack == 1 {
			t.MaxStack = 999
		}
	}
	if t.SellPrice < 0 {
		t.SellPrice = 0
	}
	if t.BuyPrice < 0 {
		t.BuyPrice = 0
	}
	if t.Weight == 0 && t.Kind != ItemKindCurrency {
		t.Weight = 1
	}
}

func fallbackItemTemplate(itemID int) ItemTemplate {
	t := ItemTemplate{
		ItemID:        itemID,
		InventoryType: InventoryRegular,
		EquipmentSlot: -1,
		AuctionLevel:  auctionLevelForRequiredLevel(0),
		Weight:        1,
		SellPrice:     int64(itemID / 100),
		BuyPrice:      int64(itemID / 50),
		MaxStack:      1,
	}
	if t.SellPrice <= 0 {
		t.SellPrice = 1
	}
	if t.BuyPrice <= 0 {
		t.BuyPrice = t.SellPrice * 2
	}
	switch {
	case itemID == goldItemID:
		t.Kind = ItemKindCurrency
		t.Category = 25
		t.IsStackable = true
		t.MaxStack = 2_000_000_000
		t.Weight = 0
		t.SellPrice = 0
		t.BuyPrice = 0
	case itemID >= 30000 && itemID < 60000:
		t.Kind = ItemKindMaterial
		t.Category = 91
		t.IsStackable = true
		t.MaxStack = 999
	case itemID >= 20000 && itemID < 30000:
		t.Kind = ItemKindConsumable
		t.Category = ItemCategoryHealthPotion
		t.IsStackable = true
		t.MaxStack = 99
	default:
		t.Kind = ItemKindUnknown
	}
	normalizeItemTemplate(&t)
	return t
}

func AuctionLevelForRequiredLevel(requiredLevel byte) byte {
	return auctionLevelForRequiredLevel(requiredLevel)
}

func auctionLevelForRequiredLevel(requiredLevel byte) byte {
	switch {
	case requiredLevel < 20:
		return 0
	case requiredLevel < 40:
		return 1
	case requiredLevel < 60:
		return 2
	case requiredLevel < 80:
		return 3
	default:
		return 4
	}
}

func InventorySlotCount(inv byte) int16 {
	switch inv {
	case InventoryShop:
		return ShopInventorySlots
	case InventoryRegular:
		return RegularInventorySlots
	case InventoryEquipment:
		return EquipmentInventorySlots
	case InventoryWarehouse, InventoryAvatarWarehouse:
		return WarehouseInventorySlots
	default:
		return 0
	}
}

func IsValidInventorySlot(inv byte, slot int16) bool {
	max := InventorySlotCount(inv)
	return max > 0 && slot >= 0 && slot < max
}

func IsEquipmentInventory(inv byte) bool {
	return inv == InventoryEquipment
}
