package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sickagent/n0/pkg/shared/jwt"
	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	"go.uber.org/zap"
)

func TestServer_MCPRequiresJWT(t *testing.T) {
	manager := jwt.NewManager([]byte("01234567890123456789012345678901"), "n0-gateway", time.Hour)
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, &fakeQueryClient{}, &fakeCMClient{}, manager)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestServer_MCPListsToolsAndSubmitsQuery(t *testing.T) {
	query := &fakeQueryClient{submitResp: &pb.SubmitQueryResponse{JobId: "job-1", Status: "pending"}}
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, query, &fakeCMClient{}, nil)
	handler := srv.handler()

	initResponse := sendMCPRequest(t, handler, "1", "initialize", map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	})
	if initResponse.Code != http.StatusOK {
		t.Fatalf("initialize: expected 200, got %d: %s", initResponse.Code, initResponse.Body.String())
	}
	toolsResponse := sendMCPRequest(t, handler, "2", "tools/list", map[string]any{})
	if toolsResponse.Code != http.StatusOK {
		t.Fatalf("tools/list: expected 200, got %d: %s", toolsResponse.Code, toolsResponse.Body.String())
	}
	var toolsPayload struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(toolsResponse.Body.Bytes(), &toolsPayload); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if !containsMCPTool(toolsPayload.Result.Tools, "get_schema") || !containsMCPTool(toolsPayload.Result.Tools, "submit_query") {
		t.Fatalf("expected schema and query tools, got %#v", toolsPayload.Result.Tools)
	}

	callResponse := sendMCPRequest(t, handler, "3", "tools/call", map[string]any{
		"name": "submit_query",
		"arguments": map[string]any{
			"connection_id": "conn-1",
			"sql":           "SELECT 1",
		},
	})
	if callResponse.Code != http.StatusOK {
		t.Fatalf("tools/call: expected 200, got %d: %s", callResponse.Code, callResponse.Body.String())
	}
	var callPayload struct {
		Result struct {
			StructuredContent struct {
				JobID string `json:"job_id"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(callResponse.Body.Bytes(), &callPayload); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if callPayload.Result.StructuredContent.JobID != "job-1" {
		t.Fatalf("expected job-1, got %#v", callPayload.Result.StructuredContent)
	}
	if query.lastSubmit == nil || query.lastSubmit.ConnectionId != "conn-1" || query.lastSubmit.Sql != "SELECT 1" {
		t.Fatalf("unexpected submitted query: %#v", query.lastSubmit)
	}
}

func TestServer_MCPPropagatesJWTTenant(t *testing.T) {
	manager := jwt.NewManager([]byte("01234567890123456789012345678901"), "n0-gateway", time.Hour)
	token, err := manager.GenerateUserToken("tenant-42", "agent@example.com")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	query := &fakeQueryClient{submitResp: &pb.SubmitQueryResponse{JobId: "job-tenant", Status: "pending"}}
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, query, &fakeCMClient{}, manager)

	response := sendMCPRequestWithToken(t, srv.handler(), "1", "tools/call", map[string]any{
		"name": "submit_query",
		"arguments": map[string]any{
			"connection_id": "conn-1",
			"sql":           "SELECT 1",
		},
	}, token)
	if response.Code != http.StatusOK {
		t.Fatalf("tools/call: expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if query.lastSubmit == nil || query.lastSubmit.TenantId != "tenant-42" {
		t.Fatalf("expected tenant-42 in internal request, got %#v", query.lastSubmit)
	}
}

func sendMCPRequest(t *testing.T, handler http.Handler, id, method string, params map[string]any) *httptest.ResponseRecorder {
	return sendMCPRequestWithToken(t, handler, id, method, params, "")
}

func sendMCPRequestWithToken(t *testing.T, handler http.Handler, id, method string, params map[string]any, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		t.Fatalf("encode MCP request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func containsMCPTool(tools []struct {
	Name string `json:"name"`
}, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
