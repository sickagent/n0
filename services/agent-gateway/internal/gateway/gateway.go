package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	"n0/pkg/shared/jwt"
	pb "n0/proto/gen/go/lensagent/v1"
)

// MetaClient defines the subset of MetaService client used by the gateway.
type MetaClient interface {
	GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error)
	ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.ListWorkspacesResponse, error)
	CreateConnection(ctx context.Context, req *pb.CreateConnectionRequest) (*pb.CreateConnectionResponse, error)
	GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error)
	ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error)
	DeleteConnection(ctx context.Context, req *pb.DeleteConnectionRequest) (*pb.DeleteConnectionResponse, error)
	RegisterPlugin(ctx context.Context, req *pb.RegisterPluginRequest) (*pb.RegisterPluginResponse, error)
}

// QueryClient defines the subset of QueryEngine client used by the gateway.
type QueryClient interface {
	SubmitQuery(ctx context.Context, req *pb.SubmitQueryRequest) (*pb.SubmitQueryResponse, error)
}

// CMClient defines the subset of ConnectionManager client used by the gateway.
type CMClient interface {
	TestConnection(ctx context.Context, req *pb.TestConnectionRequest) (*pb.TestConnectionResponse, error)
	GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error)
	ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error)
}

// Server hosts both gRPC and HTTP interfaces.
type Server struct {
	grpcAddr     string
	httpAddr     string
	log          *zap.Logger
	metaCli      MetaClient
	queryCli     QueryClient
	cmCli        CMClient
	jwtManager   *jwt.Manager
	metaHTTPBase string
}

// NewServer creates a new gateway server.
func NewServer(grpcAddr, httpAddr string, log *zap.Logger, metaCli MetaClient, queryCli QueryClient, cmCli CMClient, jwtManager *jwt.Manager) *Server {
	return &Server{
		grpcAddr:     grpcAddr,
		httpAddr:     httpAddr,
		log:          log,
		metaCli:      metaCli,
		queryCli:     queryCli,
		cmCli:        cmCli,
		jwtManager:   jwtManager,
		metaHTTPBase: "http://meta-service:8081",
	}
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

func (s *Server) jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.jwtManager == nil {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "missing authorization header", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}
		claims, err := s.jwtManager.Verify(parts[1])
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), "user_id", claims.UserID))
		r = r.WithContext(context.WithValue(r.Context(), "agent_id", claims.Subject))
		r = r.WithContext(context.WithValue(r.Context(), "token_type", claims.Type))
		next.ServeHTTP(w, r)
	})
}

func getUserID(r *http.Request) string {
	if v := r.Context().Value("user_id"); v != nil {
		return v.(string)
	}
	return ""
}

func (s *Server) handler() http.Handler {
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Public auth endpoints (proxy to meta-service HTTP)
	r.Post("/v1/auth/register", s.proxyToMetaHTTP)
	r.Post("/v1/auth/login", s.handleLogin)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(s.jwtMiddleware)
		r.Get("/v1/auth/me", s.proxyToMetaHTTP)
		r.Get("/v1/agents", s.proxyToMetaHTTP)
		r.Post("/v1/agents", s.proxyToMetaHTTP)
		r.Post("/v1/agents/{id}/token", s.handleGenerateAgentToken)

		r.Get("/v1/schema", s.handleGetSchema)
		r.Get("/v1/query", s.handleSubmitQuery)
		r.Post("/v1/test-connection", s.handleTestConnection)
		r.Post("/v1/execute-query", s.handleExecuteQuery)
		r.Post("/v1/schema", s.handleGetConnectionSchema)
		r.Post("/v1/connections", s.handleCreateConnection)
		r.Get("/v1/connections", s.handleListConnections)
		r.Get("/v1/connections/{id}", s.handleGetConnection)
		r.Delete("/v1/connections/{id}", s.handleDeleteConnection)
		r.Post("/v1/plugins/register", s.handleRegisterPlugin)
		r.Get("/v1/workspaces", s.handleListWorkspaces)
	})
	return r
}

func (s *Server) proxyToMetaHTTP(w http.ResponseWriter, r *http.Request) {
	url := s.metaHTTPBase + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	req, err := http.NewRequest(r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header = r.Header.Clone()
	// inject user_id into query for /v1/agents and /v1/auth/me
	if uid := getUserID(r); uid != "" {
		q := req.URL.Query()
		q.Set("user_id", uid)
		req.URL.RawQuery = q.Encode()
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// First proxy to meta-service to validate credentials
	url := s.metaHTTPBase + "/v1/auth/login"
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	req, err := http.NewRequest(r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	var user struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(respBody, &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if s.jwtManager == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBody)
		return
	}

	token, err := s.jwtManager.GenerateUserToken(user.UserID, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id": user.UserID,
		"email":   user.Email,
		"role":    user.Role,
		"token":   token,
	})
}

func (s *Server) handleGenerateAgentToken(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	userID := getUserID(r)
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if s.jwtManager == nil {
		http.Error(w, "jwt not configured", http.StatusInternalServerError)
		return
	}
	token, err := s.jwtManager.GenerateAgentToken(agentID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"agent_id": agentID, "token": token})
}

// Start launches the HTTP server. gRPC server can be added here later.
func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{Addr: s.httpAddr, Handler: s.handler()}

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			s.log.Error("http shutdown error", zap.Error(err))
		}
	}()

	s.log.Info("http gateway listening", zap.String("addr", s.httpAddr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

func (s *Server) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &pb.GetSchemaRequest{
		ConnectionId: r.URL.Query().Get("connection_id"),
	}
	resp, err := s.metaCli.GetSchema(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSubmitQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &pb.SubmitQueryRequest{
		TenantId:     getUserID(r),
		ConnectionId: r.URL.Query().Get("connection_id"),
		Sql:          r.URL.Query().Get("sql"),
	}
	resp, err := s.queryCli.SubmitQuery(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		AdapterType string                 `json:"adapter_type"`
		Params      map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	params, err := structpb.NewStruct(body.Params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.cmCli.TestConnection(ctx, &pb.TestConnectionRequest{
		AdapterType: body.AdapterType,
		Params:      params,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetConnectionSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		ConnectionID string                 `json:"connection_id"`
		AdapterType  string                 `json:"adapter_type"`
		Params       map[string]interface{} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	params, err := structpb.NewStruct(body.Params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.cmCli.GetSchema(ctx, &pb.GetConnectionSchemaRequest{
		ConnectionId: body.ConnectionID,
		AdapterType:  body.AdapterType,
		Params:       params,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleExecuteQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req pb.ExecuteQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.cmCli.ExecuteQuery(ctx, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req pb.CreateConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Inject authenticated user_id into the request workspace scope
	req.TenantId = getUserID(r)
	resp, err := s.metaCli.CreateConnection(ctx, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	resp, err := s.metaCli.ListConnections(ctx, &pb.ListConnectionsRequest{
		TenantId:    getUserID(r),
		WorkspaceId: r.URL.Query().Get("workspace_id"),
		Pagination:  &pb.Pagination{Limit: int32(limit), Offset: int32(offset)},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, err := s.metaCli.GetConnection(ctx, &pb.GetConnectionRequest{
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
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, err := s.metaCli.DeleteConnection(ctx, &pb.DeleteConnectionRequest{
		ConnectionId: chi.URLParam(r, "id"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRegisterPlugin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req pb.RegisterPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.metaCli.RegisterPlugin(ctx, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}
	resp, err := s.metaCli.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{
		TenantId:   getUserID(r),
		Pagination: &pb.Pagination{Limit: int32(limit), Offset: int32(offset)},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func init() {
	http.DefaultClient.Timeout = 30 * time.Second
}
