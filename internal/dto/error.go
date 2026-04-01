package dto

// Standard error envelope for all error responses
// {"error": {"code": "INSUFFICIENT_FUNDS", "message": "..."}}
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes
const (
	ErrDebitCreditMismatch = "DEBIT_CREDIT_MISMATCH"
	ErrInsufficientFunds   = "INSUFFICIENT_FUNDS"
	ErrUnknownAccount      = "UNKNOWN_ACCOUNT"
	ErrCurrencyMismatch    = "CURRENCY_MISMATCH"
	ErrDuplicateKey        = "DUPLICATE_IDEMPOTENCY_KEY"
	ErrInvalidRequest      = "INVALID_REQUEST"
	ErrInternal            = "INTERNAL_ERROR"
)
