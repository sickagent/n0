package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sickagent/n0/services/connection-manager/internal/registry"
	"go.uber.org/zap"
)

func TestHTTPServer_Health(t *testing.T) {
	log := zap.NewNop()
	reg := registry.NewRegistry(log)
	srv := NewHTTPServer(":0", log, reg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
