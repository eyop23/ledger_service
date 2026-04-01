package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eyop23/ledger_service/config"
	"github.com/eyop23/ledger_service/internal/db"
	"github.com/eyop23/ledger_service/internal/handler"
	"github.com/eyop23/ledger_service/internal/middleware"
	"github.com/eyop23/ledger_service/internal/service"
	"github.com/eyop23/ledger_service/utils"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Config
	cfg := config.Load()

	// 2. Logger — structured JSON, level from env
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))

	// 3. Database
	pool, err := utils.InitDB(cfg.GetDatabaseURL())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer utils.CloseDB(pool)

	// 4. Migrations — safe to run every startup, goose skips already-applied ones
	if err := runMigrations(pool); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	// 5. Queries (sqlc generated)
	queries := db.New(pool)

	// 6. Services
	accountSvc := service.NewAccountService(queries, pool)
	txnSvc := service.NewTransactionService(queries, pool)

	// 7. Handlers
	accountHandler := handler.NewAccountHandler(accountSvc)
	balanceHandler := handler.NewBalanceHandler(accountSvc)
	entriesHandler := handler.NewEntriesHandler(queries)
	transactionHandler := handler.NewTransactionHandler(txnSvc)
	auditHandler := handler.NewAuditHandler(queries)
	healthHandler := handler.NewHealthHandler(pool)

	// 8. Router + middleware stack
	// Order matters: RequestID → ActorID → Logger → Metrics → Handler
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.ActorID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Metrics)

	// 9. Routes
	r.Post("/accounts", accountHandler.CreateAccount)
	r.Get("/accounts/{id}", accountHandler.GetAccount)
	r.Get("/accounts/{id}/balance", balanceHandler.GetBalance)
	r.Get("/accounts/{id}/entries", entriesHandler.GetEntries)

	r.Post("/transactions", transactionHandler.CreateTransaction)
	r.Get("/transactions/{id}", transactionHandler.GetTransaction)

	r.Get("/audit", auditHandler.GetAuditLogs)
	r.Get("/health", healthHandler.Health)
	r.Handle("/metrics", promhttp.Handler())

	// 10. Start server in a goroutine so main goroutine can wait for shutdown signal
	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}

	go func() {
		logger.Info("server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Block until CTRL+C or SIGTERM (e.g. docker stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Give in-flight requests up to 10 seconds to finish before forcing exit
	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("server stopped")
}

// runMigrations converts the pgxpool to a *sql.DB that goose can use,
// then applies all pending migrations from the migrations/ directory.
func runMigrations(pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return goose.Up(sqlDB, "migrations")
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
