package main

import (
	"fmt"
	"sync"
)

const (
	privateShopItemSlots = 10
	privateShopBuySlots  = 6
)

const (
	privateShopOpenedError byte = iota
	privateShopOpenedOK
	privateShopOpenedNoItemInfo
	privateShopOpenedItemAlreadyPlaced
	privateShopOpenedUnexchangeable
	privateShopOpenedEquippedSowel
	privateShopOpenedTooManyItems
	privateShopOpenedGoldDenied
	privateShopOpenedNeedLevel24
)

const (
	privateShopWindowFail byte = iota
	privateShopWindowOK
	privateShopWindowAlreadyInShop
	privateShopWindowWar
	privateShopWindowDead
	privateShopWindowNeedLevel10
	privateShopWindowNoFunctionalItem
	privateShopWindowPvpZone
)

const (
	privateShopBuyError byte = iota
	privateShopBuyOK
	privateShopBuySelectedUnavailable
	privateShopBuyUserClosed
	privateShopBuyWeightExceeded
	privateShopBuyNoSlots
	privateShopBuyNotEnoughGold
	privateShopBuyAmountUnavailable
	privateShopBuyAlreadyPurchased
	privateShopBuyNeedLevel24
)

const (
	privateShopInfoNoShop byte = iota
	privateShopInfoOK
	privateShopInfoCapacityExceeded
)

const (
	privateShopCloseError byte = iota
	privateShopCloseOK
	privateShopCloseHostClosed
)

const (
	privateShopNotifyLeft byte = iota
	privateShopNotifyJoined
)

type privateShop struct {
	Owner  *Client
	Title  string
	Items  [privateShopItemSlots]*tradeItemRef
	Joined map[uint32]*Client

	Trading bool
}

type privateShopManager struct {
	mu          sync.Mutex
	ownerShops  map[uint32]*privateShop
	joinedShops map[uint32]*privateShop
}

var privateShopRuntime = &privateShopManager{
	ownerShops:  make(map[uint32]*privateShop),
	joinedShops: make(map[uint32]*privateShop),
}

func (m *privateShopManager) openWindow(owner *Client) byte {
	if owner == nil || owner.Char == nil {
		return privateShopWindowFail
	}
	if tradeRuntime.hasActive(owner) {
		return privateShopWindowFail
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ownerShops[owner.Char.GUID] != nil || m.joinedShops[owner.Char.GUID] != nil {
		return privateShopWindowAlreadyInShop
	}
	m.ownerShops[owner.Char.GUID] = &privateShop{
		Owner:  owner,
		Joined: make(map[uint32]*Client),
	}
	return privateShopWindowOK
}

func (m *privateShopManager) start(owner *Client, refs [privateShopItemSlots]*tradeItemRef, title string) (byte, *privateShop) {
	if owner == nil || owner.Char == nil {
		return privateShopOpenedError, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	shop := m.ownerShops[owner.Char.GUID]
	if shop == nil {
		return privateShopOpenedError, nil
	}
	for i, ref := range refs {
		if ref == nil {
			continue
		}
		if ref.IsGold || ref.itemID() == int(goldLootItemID) {
			return privateShopOpenedGoldDenied, shop.snapshot()
		}
		if ref.Item == nil || findItem(owner.Char, ref.InventoryType, ref.Slot) == nil {
			return privateShopOpenedNoItemInfo, shop.snapshot()
		}
		if ref.Item.IsSoulBound {
			return privateShopOpenedUnexchangeable, shop.snapshot()
		}
		if ref.Price < 0 || ref.Price > maxInt32Value {
			return privateShopOpenedError, shop.snapshot()
		}
		ref.TradeSlot = byte(i)
	}
	shop.Items = clonePrivateShopRefs(refs)
	shop.Title = title
	shop.Trading = true
	return privateShopOpenedOK, shop.snapshot()
}

func (m *privateShopManager) view(viewer *Client, ownerAccID uint32) (byte, *privateShop, []*Client) {
	if viewer == nil || viewer.Char == nil {
		return privateShopInfoNoShop, nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	shop := m.shopByOwnerAccIDLocked(ownerAccID)
	if shop == nil || !shop.Trading || shop.Owner == nil || shop.Owner.Char == nil {
		return privateShopInfoNoShop, nil, nil
	}
	if viewer.Char.GUID == shop.Owner.Char.GUID {
		return privateShopInfoOK, shop.snapshot(), nil
	}
	if m.joinedShops[viewer.Char.GUID] != nil {
		return privateShopInfoCapacityExceeded, nil, nil
	}
	notify := shop.participantsLocked()
	shop.Joined[viewer.Char.GUID] = viewer
	m.joinedShops[viewer.Char.GUID] = shop
	return privateShopInfoOK, shop.snapshot(), notify
}

func (m *privateShopManager) closeFor(c *Client) (byte, *privateShop, []*Client, bool) {
	if c == nil || c.Char == nil {
		return privateShopCloseError, nil, nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if shop := m.ownerShops[c.Char.GUID]; shop != nil {
		joined := shop.joinedSnapshotLocked()
		for guid := range shop.Joined {
			delete(m.joinedShops, guid)
		}
		delete(m.ownerShops, c.Char.GUID)
		shop.Trading = false
		return privateShopCloseOK, shop.snapshot(), joined, true
	}
	shop := m.joinedShops[c.Char.GUID]
	if shop == nil {
		return privateShopCloseOK, nil, nil, false
	}
	delete(shop.Joined, c.Char.GUID)
	delete(m.joinedShops, c.Char.GUID)
	notify := shop.participantsLocked()
	return privateShopCloseOK, shop.snapshot(), notify, false
}

func (m *privateShopManager) shopForClient(c *Client) *privateShop {
	if c == nil || c.Char == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if shop := m.ownerShops[c.Char.GUID]; shop != nil {
		return shop.snapshot()
	}
	if shop := m.joinedShops[c.Char.GUID]; shop != nil {
		return shop.snapshot()
	}
	return nil
}

func (m *privateShopManager) buy(buyer *Client, requests []privateShopBuyRequest) (byte, *privateShop, []privateShopSoldRef, []*ItemRow, error) {
	if buyer == nil || buyer.Char == nil {
		return privateShopBuyError, nil, nil, nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	shop := m.joinedShops[buyer.Char.GUID]
	if shop == nil || !shop.Trading || shop.Owner == nil || shop.Owner.Char == nil {
		return privateShopBuyError, nil, nil, nil, nil
	}
	if len(requests) == 0 {
		return privateShopBuySelectedUnavailable, shop.snapshot(), nil, nil, nil
	}
	source, total, status := shop.resolveBuyRequestsLocked(requests)
	if status != privateShopBuyOK {
		return status, shop.snapshot(), nil, nil, nil
	}
	if total > buyer.Char.Gold {
		return privateShopBuyNotEnoughGold, shop.snapshot(), nil, nil, nil
	}
	if shop.Owner.Char.Gold > maxInt32Value-total {
		return privateShopBuyError, shop.snapshot(), nil, nil, nil
	}
	if status := canReceivePrivateShopItems(buyer.Char, source); status != privateShopBuyOK {
		return status, shop.snapshot(), nil, nil, nil
	}

	buyer.Char.Gold -= total
	shop.Owner.Char.Gold += total
	bought := make([]*ItemRow, 0, len(source))
	sold := make([]privateShopSoldRef, 0, len(source))
	for _, ref := range source {
		amount := ref.Amount
		if amount <= 0 {
			amount = 1
		}
		added, addStatus, err := createCharacterItemDetailed(
			buyer.Char,
			ref.Item.ItemID,
			amount,
			ref.Item.InventoryType,
			-1,
			ref.Item,
			0,
		)
		if err != nil {
			return privateShopBuyError, shop.snapshot(), sold, bought, err
		}
		if addStatus == inventoryStatusNoSpace {
			return privateShopBuyNoSlots, shop.snapshot(), sold, bought, nil
		}
		if addStatus == inventoryStatusWeightExceeds {
			return privateShopBuyWeightExceeded, shop.snapshot(), sold, bought, nil
		}
		if addStatus != inventoryStatusOK || added == nil {
			return privateShopBuyError, shop.snapshot(), sold, bought, nil
		}
		if err := removeCharacterItem(shop.Owner.Char, ref.Item, amount); err != nil {
			return privateShopBuyError, shop.snapshot(), sold, bought, err
		}
		bought = append(bought, added)
		listed := shop.Items[ref.TradeSlot]
		soldRef := privateShopSoldRef{Ref: cloneTradeRef(ref)}
		if listed != nil {
			listed.Amount -= amount
			if listed.Amount <= 0 {
				listed.Amount = -1
			}
			soldRef.Ref.Amount = amount
		}
		sold = append(sold, soldRef)
	}
	if err := SaveCharacter(buyer.Char); err != nil {
		return privateShopBuyError, shop.snapshot(), sold, bought, err
	}
	if err := SaveCharacter(shop.Owner.Char); err != nil {
		return privateShopBuyError, shop.snapshot(), sold, bought, err
	}
	return privateShopBuyOK, shop.snapshot(), sold, bought, nil
}

func (m *privateShopManager) isBusy(c *Client) bool {
	if c == nil || c.Char == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ownerShops[c.Char.GUID] != nil || m.joinedShops[c.Char.GUID] != nil
}

func (m *privateShopManager) itemLocked(chr *Character, item *ItemRow) bool {
	if chr == nil || item == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	shop := m.ownerShops[chr.GUID]
	if shop == nil {
		return false
	}
	for _, ref := range shop.Items {
		if ref != nil && ref.Item != nil && ref.Item.Guid == item.Guid {
			return true
		}
	}
	return false
}

func (m *privateShopManager) cleanupForDisconnect(c *Client) {
	status, shop, joined, ownerClosed := m.closeFor(c)
	if status != privateShopCloseOK || shop == nil {
		return
	}
	if ownerClosed {
		for _, viewer := range joined {
			sendCloseCharacterTradeShopToOwnerResponse(viewer, privateShopCloseHostClosed)
		}
		return
	}
	for _, receiver := range joined {
		sendPrivateShopChatNotification(receiver, c.Char.AccID, privateShopNotifyLeft)
	}
}

func (m *privateShopManager) shopByOwnerAccIDLocked(ownerAccID uint32) *privateShop {
	for _, shop := range m.ownerShops {
		if shop != nil && shop.Owner != nil && shop.Owner.Char != nil && shop.Owner.Char.AccID == ownerAccID {
			return shop
		}
	}
	return nil
}

func (s *privateShop) resolveBuyRequestsLocked(requests []privateShopBuyRequest) ([]*tradeItemRef, int64, byte) {
	source := make([]*tradeItemRef, 0, len(requests))
	total := int64(0)
	seen := map[byte]bool{}
	for _, req := range requests {
		if req.TradeSlot >= privateShopItemSlots || seen[req.TradeSlot] {
			return nil, 0, privateShopBuySelectedUnavailable
		}
		seen[req.TradeSlot] = true
		listed := s.Items[req.TradeSlot]
		amount := req.Amount
		if amount <= 0 {
			amount = 1
		}
		if listed == nil || listed.Amount == -1 || listed.Item == nil || listed.Item.Amount < amount || listed.Amount < amount {
			return nil, 0, privateShopBuyAmountUnavailable
		}
		price := listed.Price * int64(amount)
		if price < 0 || total > maxInt32Value-price {
			return nil, 0, privateShopBuyNotEnoughGold
		}
		total += price
		ref := cloneTradeRef(listed)
		ref.Amount = amount
		source = append(source, ref)
	}
	return source, total, privateShopBuyOK
}

func canReceivePrivateShopItems(receiver *Character, refs []*tradeItemRef) byte {
	requiredSlots := map[byte]int{}
	seenStacks := map[byte]map[int]bool{}
	addedWeight := 0
	for _, ref := range refs {
		if ref == nil || ref.Item == nil {
			continue
		}
		inv := ref.Item.InventoryType
		amount := ref.amountOrOne()
		if effectiveItemStackable(ref.Item) {
			if seenStacks[inv] == nil {
				seenStacks[inv] = map[int]bool{}
			}
			if findItemByID(receiver, inv, ref.Item.ItemID) == nil && !seenStacks[inv][ref.Item.ItemID] {
				requiredSlots[inv]++
				seenStacks[inv][ref.Item.ItemID] = true
			}
		} else {
			requiredSlots[inv] += amount
		}
		if isCarriedInventory(inv) {
			addedWeight += itemUnitWeight(ref.Item) * amount
		}
	}
	for inv, count := range requiredSlots {
		if freeInventorySlotCount(receiver, inv) < count {
			return privateShopBuyNoSlots
		}
	}
	if carriedWeight(receiver)+addedWeight > maxWeight(receiver) {
		return privateShopBuyWeightExceeded
	}
	return privateShopBuyOK
}

func (s *privateShop) participantsLocked() []*Client {
	out := make([]*Client, 0, len(s.Joined)+1)
	if s.Owner != nil {
		out = append(out, s.Owner)
	}
	for _, c := range s.Joined {
		out = append(out, c)
	}
	return out
}

func (s *privateShop) joinedSnapshotLocked() []*Client {
	out := make([]*Client, 0, len(s.Joined))
	for _, c := range s.Joined {
		out = append(out, c)
	}
	return out
}

func (s *privateShop) snapshot() *privateShop {
	if s == nil {
		return nil
	}
	out := &privateShop{
		Owner:   s.Owner,
		Title:   s.Title,
		Items:   clonePrivateShopRefs(s.Items),
		Joined:  make(map[uint32]*Client, len(s.Joined)),
		Trading: s.Trading,
	}
	for guid, c := range s.Joined {
		out.Joined[guid] = c
	}
	return out
}

type privateShopBuyRequest struct {
	TradeSlot byte
	Amount    int
}

type privateShopSoldRef struct {
	Ref *tradeItemRef
}

func clonePrivateShopRefs(in [privateShopItemSlots]*tradeItemRef) [privateShopItemSlots]*tradeItemRef {
	var out [privateShopItemSlots]*tradeItemRef
	for i, ref := range in {
		out[i] = cloneTradeRef(ref)
	}
	return out
}

func cloneTradeRef(ref *tradeItemRef) *tradeItemRef {
	if ref == nil {
		return nil
	}
	out := *ref
	return &out
}

func validatePrivateShopRef(owner *Client, ref *tradeItemRef) error {
	if owner == nil || owner.Char == nil || ref == nil {
		return fmt.Errorf("missing private shop item")
	}
	item := findItem(owner.Char, ref.InventoryType, ref.Slot)
	if item == nil || ref.Item == nil || item.Guid != ref.Item.Guid {
		return fmt.Errorf("private shop item not found")
	}
	if ref.Amount <= 0 || ref.Amount > item.Amount {
		return fmt.Errorf("invalid private shop amount")
	}
	return nil
}
