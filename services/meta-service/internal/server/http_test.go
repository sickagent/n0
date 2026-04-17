package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"n0/services/meta-service/internal/app"
)

func TestHTTPServer_Health(t *testing.T) {
	log := zap.NewNop()
	svc := app.NewMetaService(nil, nil, nil)
	grpcHandler := NewGRPCServer(svc)
	srv := NewHTTPServer(":0", log, grpcHandler, svc)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
