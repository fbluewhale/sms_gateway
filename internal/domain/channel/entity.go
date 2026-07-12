package channel

import "time"

type Channel struct {
	ID        int64
	Name      string
	WalletID  int64
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
