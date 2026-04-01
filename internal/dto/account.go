package dto

// POST /accounts
type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

type AccountResponse struct {
	ID        string `json:"id"`
	Currency  string `json:"currency"`
	CreatedAt string `json:"created_at"`
}

// GET /accounts/{id}/balance
type BalanceResponse struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}

// GET /accounts/{id}/entries
type EntryResponse struct {
	ID            string `json:"id"`
	TransactionID string `json:"transaction_id"`
	Direction     string `json:"direction"`
	Amount        int64  `json:"amount"`
	CreatedAt     string `json:"created_at"`
}

type EntriesResponse struct {
	Entries    []EntryResponse `json:"entries"`
	NextCursor *string         `json:"next_cursor"`
}
