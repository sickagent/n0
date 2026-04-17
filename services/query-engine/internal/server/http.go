package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
)

// HTTPServer hosts the REST API for query-engine.
type HTTPServer struct {
	srv *http.Server
	log *zap.Logger
	svc *GRPCServer
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, log *zap.Logger, svc *GRPCServer) *HTTPServer {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/v1/query/submit", func(w http.ResponseWriter, r *http.Request) {
		var req pb.SubmitQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := svc.SubmitQuery(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Get("/v1/query/status", func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetJobStatus(r.Context(), &pb.GetJobStatusRequest{
			JobId: r.URL.Query().Get("job_id"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Get("/v1/query/result", func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetJobResult(r.Context(), &pb.GetJobResultRequest{
			JobId:    r.URL.Query().Get("job_id"),
			Page:     int32(parseInt(r.URL.Query().Get("page"))),
			PageSize: int32(parseInt(r.URL.Query().Get("page_size"))),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/query/suggest-chart", func(w http.ResponseWriter, r *http.Request) {
		var req pb.SuggestChartConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := svc.SuggestChartConfig(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	return &HTTPServer{
		srv: &http.Server{Addr: addr, Handler: r},
		log: log,
		svc: svc,
	}
}

// Handler returns the HTTP handler for testing.
func (s *HTTPServer) Handler() http.Handler {
	return s.srv.Handler
}

// Start launches the HTTP server.
func (s *HTTPServer) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.srv.Shutdown(context.Background())
	}()
	s.log.Info("query-engine HTTP listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
