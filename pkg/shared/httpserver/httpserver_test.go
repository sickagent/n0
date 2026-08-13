package httpserver

import (
	"net/http"
	"testing"
)

func TestNewAppliesFiniteResourceLimits(t *testing.T) {
	srv := New(":0", http.NewServeMux())
	if srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Fatalf("expected finite timeouts, got %+v", srv)
	}
	if srv.MaxHeaderBytes <= 0 {
		t.Fatal("expected a finite maximum header size")
	}
}
