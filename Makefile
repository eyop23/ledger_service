DB_URL=postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable

# ── sqlc ─────────────────────────────────────────────────────────────────────
sqlc:
	sqlc generate

# ── Database ────────────────────────────────────────────────────────────────
migrate/up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate/down:
	goose -dir migrations postgres "$(DB_URL)" down-to 0
