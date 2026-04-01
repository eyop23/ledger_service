package dto

// POST /transactions
type EntryRequest struct {
	AccountID string `json:"account_id"`
	Direction string `json:"direction"` // "DEBIT" | "CREDIT"
	Amount    int64  `json:"amount"`
}

type CreateTransactionRequest struct {
	IdempotencyKey string         `json:"idempotency_key"`
	Amount         int64          `json:"amount"`
	Currency       string         `json:"currency"`
	Entries        []EntryRequest `json:"entries"`
}

// GET /transactions/{id} and POST /transactions response
type TransactionEntryResponse struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Direction string `json:"direction"`
	Amount    int64  `json:"amount"`
	CreatedAt string `json:"created_at"`
}

type TransactionResponse struct {
	ID             string                     `json:"id"`
	IdempotencyKey string                     `json:"idempotency_key"`
	Amount         int64                      `json:"amount"`
	Currency       string                     `json:"currency"`
	Status         string                     `json:"status"`
	CreatedAt      string                     `json:"created_at"`
	Entries        []TransactionEntryResponse `json:"entries"`
}
