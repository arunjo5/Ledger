package ledger

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusCommitted Status = "committed"
	StatusFailed    Status = "failed"
)

type Account struct {
	ID                 string    `json:"id"`
	Label              *string   `json:"label,omitempty"`
	OverdraftProtected bool      `json:"overdraft_protected"`
	CreatedAt          time.Time `json:"created_at"`
}

type Entry struct {
	ID            int64     `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Currency      string    `json:"currency"`
	Amount        int64     `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}

type Transaction struct {
	ID                    string     `json:"id"`
	Status                Status     `json:"status"`
	Description           string     `json:"description"`
	ReversesTransactionID *string    `json:"reverses_transaction_id,omitempty"`
	Entries               []Entry    `json:"entries"`
	CreatedAt             time.Time  `json:"created_at"`
	CommittedAt           *time.Time `json:"committed_at,omitempty"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}
