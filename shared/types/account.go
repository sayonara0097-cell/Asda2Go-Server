package types

import "time"

type Account struct {
	ID       uint32
	Name     string
	Password string
	IsOnline bool
	LastIP   []byte
	// Characters loaded from DB on SelectChanel, used for char-select screen
	Characters []*CharacterRow
}

type AccountRow struct {
	AccountID int
	Name      string
	Password  string // plain or hashed — depends on your auth setup
	IsActive  bool
	RoleGroup string
	LastLogin *time.Time
	LastIP    []byte
	Created   time.Time
}
