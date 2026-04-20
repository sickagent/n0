package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	GatewayBaseURL = "http://localhost:8083"
	CMBaseURL      = "http://localhost:8086"
	adminUserID    = "00000000-0000-0000-0000-000000000001"
	adminEmail     = "admin@n0.local"
)

// HTTPClient is a thin wrapper around http.Client for E2E tests.
type HTTPClient struct {
	BaseURL string
	Client  *http.Client
	Token   string
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func NewGatewayClient(t *testing.T) *HTTPClient {
	t.Helper()
	client := NewHTTPClient(GatewayBaseURL)
	client.Token = mustAdminToken(t)
	return client
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
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
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

func mustAdminToken(t *testing.T) string {
	t.Helper()

	secretB64 := os.Getenv("JWT_SECRET")
	if secretB64 == "" {
		envPath := filepath.Clean(filepath.Join("..", "..", ".env"))
		data, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read .env: %v", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "JWT_SECRET=") {
				secretB64 = strings.TrimSpace(strings.TrimPrefix(line, "JWT_SECRET="))
				break
			}
		}
	}
	if secretB64 == "" {
		t.Fatal("JWT_SECRET is not configured")
	}

	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		t.Fatalf("decode JWT_SECRET: %v", err)
	}

	now := time.Now().UTC()
	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	payload := map[string]any{
		"iss":     "n0-gateway",
		"sub":     adminUserID,
		"aud":     []string{"n0-platform"},
		"exp":     now.Add(24 * time.Hour).Unix(),
		"iat":     now.Unix(),
		"jti":     fmt.Sprintf("e2e-%d", now.UnixNano()),
		"user_id": adminUserID,
		"email":   adminEmail,
		"type":    "user",
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	body := base64.RawURLEncoding.EncodeToString(payloadJSON)
	unsigned := header + "." + body

	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature
}

// Types matching API responses

type Workspace struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	CreatedAt any    `json:"created_at"`
}

type Connection struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	TenantID    string                 `json:"tenant_id"`
	Name        string                 `json:"name"`
	AdapterType string                 `json:"adapter_type"`
	Params      map[string]interface{} `json:"params"`
	CreatedAt   any                    `json:"created_at"`
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
