package main

import (
	"fmt"
	"sync"
	"time"

	"asda2/shared/types"
)

const (
	tradeSlotCount        = 5
	tradeRequestTimeout   = 30 * time.Second
	settingsGeneralTrade  = 11
	settingsShopItemTrade = 12
)

const (
	tradeTypeRegular byte = iota
	tradeTypeShopItems
)

const (
	tradeStartedDenied byte = iota
	tradeStartedOK
	tradeStartedCanceled
	tradeStartedOtherTrading
)

const (
	pushTradeStatusError byte = iota
	pushTradeStatusOK
	pushTradeStatusNoItemInfo
	pushTradeStatusItemStayed
	pushTradeStatusSoulbound
	pushTradeStatusLockedWindow
	pushTradeStatusShopItemDenied
	pushTradeStatusNeedOneGold
	pushTradeStatusPurchasedItemDenied
	pushTradeStatusUnexchangeable
	pushTradeStatusNeedLevel24
)

type tradeItemRef struct {
	Item          *ItemRow
	Amount        int
	Price         int64
	TradeSlot     byte
	InventoryType byte
	Slot          int16
	IsGold        bool
}

func (r *tradeItemRef) itemID() int {
	if r == nil {
		return 0
	}
	if r.IsGold {
		return int(goldLootItemID)
	}
	if r.Item == nil {
		return 0
	}
	return r.Item.ItemID
}

func (r *tradeItemRef) amountOrOne() int {
	if r == nil {
		return 0
	}
	if r.Amount <= 0 {
		return 1
	}
	return r.Amount
}

type tradeSession struct {
	First  *Client
	Second *Client

	TradeType byte
	CreatedAt time.Time
	Accepted  bool

	FirstItems  [tradeSlotCount]*tradeItemRef
	SecondItems [tradeSlotCount]*tradeItemRef

	FirstShown      bool
	SecondShown     bool
	FirstConfirmed  bool
	SecondConfirmed bool
	Cleaned         bool
	Transferred     bool
}

type tradeTransferResult struct {
	FirstReceives  [tradeSlotCount]*ItemRow
	SecondReceives [tradeSlotCount]*ItemRow
}

type tradeManager struct {
	mu     sync.Mutex
	byChar map[uint32]*tradeSession
}

var tradeRuntime = &tradeManager{
	byChar: make(map[uint32]*tradeSession),
}

func (m *tradeManager) begin(sender *Client, target *Client, tradeType byte) bool {
	if sender == nil || sender.Char == nil || target == nil || target.Char == nil {
		return false
	}
	if tradeType != tradeTypeShopItems {
		tradeType = tradeTypeRegular
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeExpiredForLocked(sender.Char.GUID, time.Now())
	m.removeExpiredForLocked(target.Char.GUID, time.Now())
	if m.byChar[sender.Char.GUID] != nil || m.byChar[target.Char.GUID] != nil {
		return false
	}
	s := &tradeSession{
		First:     sender,
		Second:    target,
		TradeType: tradeType,
		CreatedAt: time.Now(),
	}
	m.byChar[sender.Char.GUID] = s
	m.byChar[target.Char.GUID] = s
	return true
}

func (m *tradeManager) accept(c *Client) (*tradeSession, bool) {
	if c == nil || c.Char == nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil || s.Cleaned {
		return nil, false
	}
	s.Accepted = true
	return s.snapshot(), true
}

func (m *tradeManager) cancelFor(c *Client) []*Client {
	if c == nil || c.Char == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil {
		return nil
	}
	return m.cleanupLocked(s)
}

func (m *tradeManager) cancelIfActive(c *Client) []*Client {
	return m.cancelFor(c)
}

func (m *tradeManager) hasActive(c *Client) bool {
	if c == nil || c.Char == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	return s != nil && !s.Cleaned
}

func (m *tradeManager) pushItem(c *Client, inv byte, slot int16, amount int) (byte, *tradeItemRef, []*Client) {
	if c == nil || c.Char == nil {
		return pushTradeStatusError, nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil || s.Cleaned {
		return pushTradeStatusLockedWindow, nil, nil
	}
	if s.itemsShownLocked(c) {
		return pushTradeStatusLockedWindow, nil, m.cleanupLocked(s)
	}
	status, ref := s.pushItemLocked(c, inv, slot, amount)
	return status, ref, nil
}

func (m *tradeManager) popItem(c *Client, inv byte, slot int16) []*Client {
	if c == nil || c.Char == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil || s.Cleaned {
		return nil
	}
	if s.itemsShownLocked(c) {
		return m.cleanupLocked(s)
	}
	items := s.itemsForLocked(c)
	if items == nil {
		return m.cleanupLocked(s)
	}
	for i, ref := range items {
		if ref != nil && ref.InventoryType == inv && ref.Slot == slot {
			items[i] = nil
			return nil
		}
	}
	return m.cleanupLocked(s)
}

func (m *tradeManager) showItems(c *Client) (*Client, [tradeSlotCount]*tradeItemRef) {
	if c == nil || c.Char == nil {
		return nil, [tradeSlotCount]*tradeItemRef{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil || s.Cleaned {
		return nil, [tradeSlotCount]*tradeItemRef{}
	}
	if c == s.First {
		if s.FirstShown {
			return nil, [tradeSlotCount]*tradeItemRef{}
		}
		s.FirstShown = true
		return s.Second, cloneTradeRefs(s.FirstItems)
	}
	if c == s.Second {
		if s.SecondShown {
			return nil, [tradeSlotCount]*tradeItemRef{}
		}
		s.SecondShown = true
		return s.First, cloneTradeRefs(s.SecondItems)
	}
	return nil, [tradeSlotCount]*tradeItemRef{}
}

func (m *tradeManager) confirm(c *Client) (*tradeSession, tradeTransferResult, []*Client, error) {
	if c == nil || c.Char == nil {
		return nil, tradeTransferResult{}, nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[c.Char.GUID]
	if s == nil || s.Cleaned {
		return nil, tradeTransferResult{}, nil, nil
	}
	if !s.FirstShown || !s.SecondShown {
		return nil, tradeTransferResult{}, m.cleanupLocked(s), fmt.Errorf("trade items were not shown")
	}
	if c == s.First {
		if s.FirstConfirmed {
			return nil, tradeTransferResult{}, nil, nil
		}
		s.FirstConfirmed = true
	} else if c == s.Second {
		if s.SecondConfirmed {
			return nil, tradeTransferResult{}, nil, nil
		}
		s.SecondConfirmed = true
	}
	if !s.FirstConfirmed || !s.SecondConfirmed {
		return nil, tradeTransferResult{}, nil, nil
	}
	result, err := s.transferLocked()
	if err != nil {
		return nil, tradeTransferResult{}, m.cleanupLocked(s), err
	}
	snapshot := s.snapshot()
	m.cleanupLocked(s)
	return snapshot, result, nil, nil
}

func (m *tradeManager) itemLocked(chr *Character, item *ItemRow) bool {
	if chr == nil || item == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.byChar[chr.GUID]
	if s == nil || s.Cleaned {
		return false
	}
	items := s.itemsForCharacterGUIDLocked(chr.GUID)
	for _, ref := range items {
		if ref != nil && ref.Item != nil && ref.Item.Guid == item.Guid {
			return true
		}
	}
	return false
}

func (m *tradeManager) removeExpiredForLocked(guid uint32, now time.Time) {
	s := m.byChar[guid]
	if s == nil || s.Accepted || now.Sub(s.CreatedAt) <= tradeRequestTimeout {
		return
	}
	m.cleanupLocked(s)
}

func (m *tradeManager) cleanupLocked(s *tradeSession) []*Client {
	if s == nil || s.Cleaned {
		return nil
	}
	s.Cleaned = true
	if s.First != nil && s.First.Char != nil {
		delete(m.byChar, s.First.Char.GUID)
	}
	if s.Second != nil && s.Second.Char != nil {
		delete(m.byChar, s.Second.Char.GUID)
	}
	return s.participants()
}

func (s *tradeSession) pushItemLocked(c *Client, inv byte, slot int16, amount int) (byte, *tradeItemRef) {
	if amount <= 0 {
		return pushTradeStatusError, nil
	}
	items := s.itemsForLocked(c)
	if items == nil {
		return pushTradeStatusError, nil
	}
	if inv == types.InventoryRegular && slot == types.ReservedRegularInventorySlot {
		return s.pushGoldLocked(c, amount, items)
	}
	if s.TradeType == tradeTypeRegular && inv != types.InventoryRegular {
		return pushTradeStatusShopItemDenied, nil
	}
	if s.TradeType == tradeTypeShopItems && inv != types.InventoryShop {
		return pushTradeStatusShopItemDenied, nil
	}
	item := findItem(c.Char, inv, slot)
	if item == nil {
		return pushTradeStatusNoItemInfo, nil
	}
	if item.ItemID == int(goldLootItemID) {
		return s.pushGoldLocked(c, amount, items)
	}
	if item.IsSoulBound {
		return pushTradeStatusSoulbound, nil
	}
	if !effectiveItemStackable(item) || amount > item.Amount {
		amount = 1
	}
	if amount <= 0 || amount > item.Amount {
		return pushTradeStatusError, nil
	}
	for _, ref := range items {
		if ref == nil || ref.Item == nil || ref.Item.Guid != item.Guid {
			continue
		}
		if ref.Amount+amount > item.Amount {
			return pushTradeStatusError, nil
		}
		ref.Amount += amount
		return pushTradeStatusOK, ref
	}
	index := firstEmptyTradeSlot(items)
	if index < 0 {
		return pushTradeStatusError, nil
	}
	ref := &tradeItemRef{
		Item:          item,
		Amount:        amount,
		TradeSlot:     byte(index),
		InventoryType: inv,
		Slot:          slot,
	}
	items[index] = ref
	return pushTradeStatusOK, ref
}

func (s *tradeSession) pushGoldLocked(c *Client, amount int, items *[tradeSlotCount]*tradeItemRef) (byte, *tradeItemRef) {
	if amount <= 0 || int64(amount) >= c.Char.Gold {
		return pushTradeStatusNeedOneGold, nil
	}
	for _, ref := range items {
		if ref == nil || !ref.IsGold {
			continue
		}
		if int64(ref.Amount+amount) >= c.Char.Gold {
			return pushTradeStatusNeedOneGold, nil
		}
		ref.Amount += amount
		return pushTradeStatusOK, ref
	}
	index := firstEmptyTradeSlot(items)
	if index < 0 {
		return pushTradeStatusError, nil
	}
	ref := &tradeItemRef{
		Amount:        amount,
		TradeSlot:     byte(index),
		InventoryType: types.InventoryRegular,
		Slot:          types.ReservedRegularInventorySlot,
		IsGold:        true,
	}
	items[index] = ref
	return pushTradeStatusOK, ref
}

func (s *tradeSession) transferLocked() (tradeTransferResult, error) {
	var result tradeTransferResult
	if s.Transferred {
		return result, nil
	}
	if err := s.validateTransferLocked(); err != nil {
		return result, err
	}
	var err error
	result.SecondReceives, err = s.transferRefsLocked(s.First, s.Second, s.FirstItems)
	if err != nil {
		return result, err
	}
	result.FirstReceives, err = s.transferRefsLocked(s.Second, s.First, s.SecondItems)
	if err != nil {
		return result, err
	}
	if err := SaveCharacter(s.First.Char); err != nil {
		return result, err
	}
	if err := SaveCharacter(s.Second.Char); err != nil {
		return result, err
	}
	s.Transferred = true
	return result, nil
}

func (s *tradeSession) validateTransferLocked() error {
	if s.First == nil || s.First.Char == nil || s.Second == nil || s.Second.Char == nil {
		return fmt.Errorf("missing trade participant")
	}
	if err := s.validateRefsLocked(s.First, s.FirstItems); err != nil {
		return err
	}
	if err := s.validateRefsLocked(s.Second, s.SecondItems); err != nil {
		return err
	}
	if err := s.validateReceiverCapacityLocked(s.Second.Char, s.FirstItems); err != nil {
		return err
	}
	if err := s.validateReceiverCapacityLocked(s.First.Char, s.SecondItems); err != nil {
		return err
	}
	return nil
}

func (s *tradeSession) validateRefsLocked(owner *Client, refs [tradeSlotCount]*tradeItemRef) error {
	outgoingGold := int64(0)
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		amount := ref.amountOrOne()
		if ref.IsGold {
			outgoingGold += int64(amount)
			continue
		}
		item := findItem(owner.Char, ref.InventoryType, ref.Slot)
		if item == nil || ref.Item == nil || item.Guid != ref.Item.Guid {
			return fmt.Errorf("trade item no longer exists")
		}
		if item.IsSoulBound {
			return fmt.Errorf("trade item is soulbound")
		}
		if amount <= 0 || item.Amount < amount {
			return fmt.Errorf("trade item amount changed")
		}
	}
	if outgoingGold > 0 {
		if outgoingGold >= owner.Char.Gold {
			return fmt.Errorf("not enough gold for trade")
		}
		other := s.otherClient(owner)
		if other == nil || other.Char == nil || other.Char.Gold > maxInt32Value-outgoingGold {
			return fmt.Errorf("trade gold would overflow")
		}
	}
	return nil
}

func (s *tradeSession) validateReceiverCapacityLocked(receiver *Character, refs [tradeSlotCount]*tradeItemRef) error {
	requiredSlots := map[byte]int{}
	seenStacks := map[byte]map[int]bool{}
	addedWeight := 0
	for _, ref := range refs {
		if ref == nil || ref.IsGold || ref.Item == nil {
			continue
		}
		destInv := s.destinationInventory(ref)
		amount := ref.amountOrOne()
		if effectiveItemStackable(ref.Item) {
			if seenStacks[destInv] == nil {
				seenStacks[destInv] = map[int]bool{}
			}
			if findItemByID(receiver, destInv, ref.Item.ItemID) == nil && !seenStacks[destInv][ref.Item.ItemID] {
				requiredSlots[destInv]++
				seenStacks[destInv][ref.Item.ItemID] = true
			}
		} else {
			requiredSlots[destInv] += amount
		}
		if isCarriedInventory(destInv) {
			addedWeight += itemUnitWeight(ref.Item) * amount
		}
	}
	for inv, count := range requiredSlots {
		if freeInventorySlotCount(receiver, inv) < count {
			return fmt.Errorf("not enough inventory slots")
		}
	}
	if carriedWeight(receiver)+addedWeight > maxWeight(receiver) {
		return fmt.Errorf("inventory weight limit exceeded")
	}
	return nil
}

func (s *tradeSession) transferRefsLocked(from *Client, to *Client, refs [tradeSlotCount]*tradeItemRef) ([tradeSlotCount]*ItemRow, error) {
	var received [tradeSlotCount]*ItemRow
	for index, ref := range refs {
		if ref == nil {
			continue
		}
		amount := ref.amountOrOne()
		if ref.IsGold {
			from.Char.Gold -= int64(amount)
			to.Char.Gold += int64(amount)
			continue
		}
		if ref.Item == nil {
			continue
		}
		if amount > ref.Item.Amount {
			amount = ref.Item.Amount
		}
		added, status, err := createCharacterItemDetailed(
			to.Char,
			ref.Item.ItemID,
			amount,
			s.destinationInventory(ref),
			-1,
			ref.Item,
			0,
		)
		if err != nil || status != inventoryStatusOK || added == nil {
			if err != nil {
				return received, err
			}
			return received, fmt.Errorf("add traded item failed status=%d", status)
		}
		if err := removeCharacterItem(from.Char, ref.Item, amount); err != nil {
			return received, err
		}
		received[index] = added
	}
	return received, nil
}

func (s *tradeSession) destinationInventory(ref *tradeItemRef) byte {
	if s.TradeType == tradeTypeShopItems {
		return types.InventoryShop
	}
	return types.InventoryRegular
}

func (s *tradeSession) itemsForLocked(c *Client) *[tradeSlotCount]*tradeItemRef {
	if c == nil || c.Char == nil {
		return nil
	}
	if s.First != nil && s.First.Char != nil && s.First.Char.GUID == c.Char.GUID {
		return &s.FirstItems
	}
	if s.Second != nil && s.Second.Char != nil && s.Second.Char.GUID == c.Char.GUID {
		return &s.SecondItems
	}
	return nil
}

func (s *tradeSession) itemsForCharacterGUIDLocked(guid uint32) [tradeSlotCount]*tradeItemRef {
	if s.First != nil && s.First.Char != nil && s.First.Char.GUID == guid {
		return s.FirstItems
	}
	if s.Second != nil && s.Second.Char != nil && s.Second.Char.GUID == guid {
		return s.SecondItems
	}
	return [tradeSlotCount]*tradeItemRef{}
}

func (s *tradeSession) itemsShownLocked(c *Client) bool {
	if c == nil || c.Char == nil {
		return true
	}
	if s.First != nil && s.First.Char != nil && s.First.Char.GUID == c.Char.GUID {
		return s.FirstShown
	}
	if s.Second != nil && s.Second.Char != nil && s.Second.Char.GUID == c.Char.GUID {
		return s.SecondShown
	}
	return true
}

func (s *tradeSession) otherClient(c *Client) *Client {
	if c == nil || c.Char == nil {
		return nil
	}
	if s.First != nil && s.First.Char != nil && s.First.Char.GUID == c.Char.GUID {
		return s.Second
	}
	if s.Second != nil && s.Second.Char != nil && s.Second.Char.GUID == c.Char.GUID {
		return s.First
	}
	return nil
}

func (s *tradeSession) participants() []*Client {
	out := make([]*Client, 0, 2)
	if s.First != nil {
		out = append(out, s.First)
	}
	if s.Second != nil && s.Second != s.First {
		out = append(out, s.Second)
	}
	return out
}

func (s *tradeSession) snapshot() *tradeSession {
	if s == nil {
		return nil
	}
	out := *s
	out.FirstItems = cloneTradeRefs(s.FirstItems)
	out.SecondItems = cloneTradeRefs(s.SecondItems)
	return &out
}

func cloneTradeRefs(in [tradeSlotCount]*tradeItemRef) [tradeSlotCount]*tradeItemRef {
	var out [tradeSlotCount]*tradeItemRef
	for i, ref := range in {
		if ref == nil {
			continue
		}
		copyRef := *ref
		out[i] = &copyRef
	}
	return out
}

func firstEmptyTradeSlot(items *[tradeSlotCount]*tradeItemRef) int {
	if items == nil {
		return -1
	}
	for i, ref := range items {
		if ref == nil {
			return i
		}
	}
	return -1
}

func itemLockedForTrading(chr *Character, item *ItemRow) bool {
	return tradeRuntime.itemLocked(chr, item) || privateShopRuntime.itemLocked(chr, item)
}
