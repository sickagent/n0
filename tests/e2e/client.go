package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	GatewayBaseURL = "http://localhost:8083"
	CMBaseURL      = "http://localhost:8086"
)

// HTTPClient is a thin wrapper around http.Client for E2E tests.
type HTTPClient struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *HTTPClient) Do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.Client.Do(req)
}

func (c *HTTPClient) JSON(method, path string, body any, out any) error {
	resp, err := c.Do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func WaitForService(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("service at %s did not become ready in time", url)
}

// Types matching API responses

type Workspace struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	CreatedAt any    `json:"created_at"`
}

type Connection struct {
	ID           string                 `json:"id"`
	WorkspaceID  string                 `json:"workspace_id"`
	TenantID     string                 `json:"tenant_id"`
	Name         string                 `json:"name"`
	AdapterType  string                 `json:"adapter_type"`
	Params       map[string]interface{} `json:"params"`
	CreatedAt    any                    `json:"created_at"`
}

type TableInfo struct {
	Name    string `json:"name"`
	Columns []struct {
		Name     string `json:"name"`
		DataType string `json:"data_type"`
		Nullable bool   `json:"nullable,omitempty"`
	} `json:"columns"`
}

type QueryResult struct {
	Columns  []string `json:"columns"`
	Rows     []Row    `json:"rows"`
	RowCount int64    `json:"row_count"`
}

type Row struct {
	Values []string `json:"values"`
}
