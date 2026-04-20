package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestServer_GetJobStatus(t *testing.T) {
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, &fakeQueryClient{}, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/query/status?job_id=job-42", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.JobID != "job-42" {
		t.Fatalf("expected job-42, got %s", resp.JobID)
	}
	if resp.Status != "pending" {
		t.Fatalf("expected pending, got %s", resp.Status)
	}
}

func TestServer_GetJobResult(t *testing.T) {
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, &fakeQueryClient{}, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/query/result?job_id=job-42&page=1&page_size=10", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.JobID != "job-42" {
		t.Fatalf("expected job-42, got %s", resp.JobID)
	}
}
