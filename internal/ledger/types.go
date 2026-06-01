package ledger

import "time"

type Account struct {
	ID        string    `json:"id"`
	Label     *string   `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
