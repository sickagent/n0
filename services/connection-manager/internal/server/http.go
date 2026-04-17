package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/connection-manager/internal/registry"
)

// HTTPServer hosts the REST API for connection management.
type HTTPServer struct {
	srv  *http.Server
	log  *zap.Logger
	grpc *GRPCServer
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, log *zap.Logger, reg *registry.Registry) *HTTPServer {
	grpcSrv := NewGRPCServer(log, reg)
	r := chi.NewRouter()
	r.Use(corsMiddleware, middleware.Logger, middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/v1/test-connection", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AdapterType string                 `json:"adapter_type"`
			Params      map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, err := structpb.NewStruct(req.Params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := grpcSrv.TestConnection(r.Context(), &pb.TestConnectionRequest{
			AdapterType: req.AdapterType,
			Params:      params,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/execute-query", func(w http.ResponseWriter, r *http.Request) {
		var req pb.ExecuteQueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := grpcSrv.ExecuteQuery(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/schema", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ConnectionID string                 `json:"connection_id"`
			AdapterType  string                 `json:"adapter_type"`
			Params       map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, err := structpb.NewStruct(req.Params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := grpcSrv.GetSchema(r.Context(), &pb.GetConnectionSchemaRequest{
			ConnectionId: req.ConnectionID,
			AdapterType:  req.AdapterType,
			Params:       params,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	return &HTTPServer{
		srv:  &http.Server{Addr: addr, Handler: r},
		log:  log,
		grpc: grpcSrv,
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
	s.log.Info("connection-manager HTTP listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
