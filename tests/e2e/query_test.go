package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuery_Submit(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewHTTPClient(GatewayBaseURL)

	// Fetch workspace
	var wsRes struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	err := client.JSON("GET", "/v1/workspaces?tenant_id=default", nil, &wsRes)
	require.NoError(t, err)
	require.NotEmpty(t, wsRes.Workspaces)
	workspaceID := wsRes.Workspaces[0].ID

	// Create a postgres connection
	connName := fmt.Sprintf("e2e-query-%d", time.Now().Unix())
	var createRes struct {
		Connection Connection `json:"connection"`
	}
	err = client.JSON("POST", "/v1/connections", map[string]any{
		"workspace_id": workspaceID,
		"tenant_id":    "default",
		"name":         connName,
		"adapter_type": "postgres",
		"params": map[string]any{
			"host":     "postgres",
			"port":     "5432",
			"user":     "postgres",
			"password": "postgres",
			"database": "meta",
			"sslmode":  "disable",
		},
	}, &createRes)
	require.NoError(t, err)
	connID := createRes.Connection.ID

	// Cleanup
	t.Cleanup(func() {
		_ = client.JSON("DELETE", fmt.Sprintf("/v1/connections/%s", connID), nil, nil)
	})

	// Submit query via gateway
	var res struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	err = client.JSON("GET", fmt.Sprintf("/v1/query?tenant_id=default&connection_id=%s&sql=SELECT+1", connID), nil, &res)
	require.NoError(t, err, "submit query should succeed")
	require.NotEmpty(t, res.JobID, "job_id should be returned")
	require.NotEmpty(t, res.Status, "status should be returned")
}
