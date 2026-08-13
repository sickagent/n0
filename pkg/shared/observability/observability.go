package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"n0/pkg/shared/graceful"
	"n0/pkg/shared/httpserver"
)

// StartMetricsServer starts a background HTTP server on addr exposing /metrics.
func StartMetricsServer(addr string, log *zap.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := httpserver.New(addr, mux)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", zap.Error(err))
		}
	}()
	return srv
}

// Shutdown stops the metrics server within a bounded deadline.
func Shutdown(srv *http.Server, log *zap.Logger) {
	ctx, cancel := graceful.WithTimeout(10 * time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Warn("metrics server shutdown failed", zap.Error(err))
	}
}
