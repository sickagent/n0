package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"n0/services/meta-service/internal/app"
	pb "n0/proto/gen/go/lensagent/v1"
)

// HTTPServer hosts the REST API for meta-service.
type HTTPServer struct {
	srv     *http.Server
	log     *zap.Logger
	svc     *GRPCServer
	metaSvc *app.MetaService
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, log *zap.Logger, svc *GRPCServer, metaSvc *app.MetaService) *HTTPServer {
	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/v1/schema", func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetSchema(r.Context(), &pb.GetSchemaRequest{
			ConnectionId: r.URL.Query().Get("connection_id"),
			TenantId:     r.URL.Query().Get("tenant_id"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Get("/v1/workspaces", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 20
		}
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = r.URL.Query().Get("tenant_id")
		}
		resp, err := svc.ListWorkspaces(r.Context(), &pb.ListWorkspacesRequest{
			TenantId:   userID,
			Pagination: &pb.Pagination{Limit: int32(limit), Offset: int32(offset)},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/connections", func(w http.ResponseWriter, r *http.Request) {
		var req pb.CreateConnectionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := svc.CreateConnection(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Get("/v1/connections", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if limit <= 0 {
			limit = 20
		}
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = r.URL.Query().Get("tenant_id")
		}
		resp, err := svc.ListConnections(r.Context(), &pb.ListConnectionsRequest{
			TenantId:    userID,
			WorkspaceId: r.URL.Query().Get("workspace_id"),
			Pagination:  &pb.Pagination{Limit: int32(limit), Offset: int32(offset)},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Get("/v1/connections/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.GetConnection(r.Context(), &pb.GetConnectionRequest{
			ConnectionId: chi.URLParam(r, "id"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if resp.Connection == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Delete("/v1/connections/{id}", func(w http.ResponseWriter, r *http.Request) {
		resp, err := svc.DeleteConnection(r.Context(), &pb.DeleteConnectionRequest{
			ConnectionId: chi.URLParam(r, "id"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/plugins/register", func(w http.ResponseWriter, r *http.Request) {
		var req pb.RegisterPluginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := svc.RegisterPlugin(r.Context(), &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Auth endpoints (bypass gRPC, talk directly to MetaService)
	r.Post("/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := metaSvc.RegisterUser(r.Context(), req.Email, req.Password, req.Role)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"user_id": id.String()})
	})
	r.Post("/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u, err := metaSvc.LoginUser(r.Context(), req.Email, req.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user_id": u.ID.String(),
			"email":   u.Email,
			"role":    u.Role,
		})
	})
	r.Get("/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("user_id")
		if id == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		u, err := metaSvc.GetUserByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if u == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user_id": u.ID.String(),
			"email":   u.Email,
			"role":    u.Role,
		})
	})
	r.Post("/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.UserID == "" {
			req.UserID = r.URL.Query().Get("user_id")
		}
		uid, err := uuid.Parse(req.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		agent, err := metaSvc.CreateAgent(r.Context(), uid, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id": agent.ID.String(),
			"user_id":  agent.UserID.String(),
			"name":     agent.Name,
			"status":   agent.Status,
		})
	})
	r.Get("/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "missing user_id", http.StatusBadRequest)
			return
		}
		agents, err := metaSvc.ListAgents(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var out []map[string]any
		for _, a := range agents {
			out = append(out, map[string]any{
				"agent_id": a.ID.String(),
				"user_id":  a.UserID.String(),
				"name":     a.Name,
				"status":   a.Status,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"agents": out})
	})

	return &HTTPServer{
		srv:     &http.Server{Addr: addr, Handler: r},
		log:     log,
		svc:     svc,
		metaSvc: metaSvc,
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
	s.log.Info("meta-service HTTP listening", zap.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
