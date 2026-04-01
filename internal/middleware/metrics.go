package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ledger_requests_total",
		Help: "Total requests by endpoint and status code",
	}, []string{"endpoint", "status_code"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ledger_request_duration_seconds",
		Help:    "Request latency by endpoint",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	TransactionErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ledger_transaction_errors_total",
		Help: "Transaction post errors by error code",
	}, []string{"code"})
)

// Metrics records Prometheus request count and latency per endpoint.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		// Use route pattern (e.g. /accounts/{id}) not actual URL to keep cardinality low
		endpoint := chi.RouteContext(r.Context()).RoutePattern()
		if endpoint == "" {
			endpoint = r.URL.Path
		}

		requestCount.WithLabelValues(endpoint, strconv.Itoa(rec.status)).Inc()
		requestDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
	})
}
