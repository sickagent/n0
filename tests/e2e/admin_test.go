package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdmin_Workspaces(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewHTTPClient(GatewayBaseURL)

	var res struct {
		Workspaces []Workspace `json:"workspaces"`
		Meta       any         `json:"meta"`
	}
	err := client.JSON("GET", "/v1/workspaces?tenant_id=default", nil, &res)
	require.NoError(t, err, "list workspaces should succeed")
	require.NotEmpty(t, res.Workspaces, "should have at least one workspace")

	foundDefault := false
	for _, ws := range res.Workspaces {
		require.NotEmpty(t, ws.ID, "workspace id should not be empty")
		require.NotEmpty(t, ws.Name, "workspace name should not be empty")
		if ws.Name == "Default Workspace" {
			foundDefault = true
		}
	}
	require.True(t, foundDefault, "Default Workspace should exist")
}

func TestAdmin_Connections_CRUD(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewHTTPClient(GatewayBaseURL)

	// Fetch a valid workspace
	var wsRes struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	err := client.JSON("GET", "/v1/workspaces?tenant_id=default", nil, &wsRes)
	require.NoError(t, err)
	require.NotEmpty(t, wsRes.Workspaces, "need at least one workspace")
	workspaceID := wsRes.Workspaces[0].ID

	connName := fmt.Sprintf("e2e-postgres-%d", time.Now().Unix())
	payload := map[string]any{
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
	}

	// CREATE
	var createRes struct {
		Connection Connection `json:"connection"`
	}
	err = client.JSON("POST", "/v1/connections", payload, &createRes)
	require.NoError(t, err, "create connection should succeed")
	require.NotEmpty(t, createRes.Connection.ID, "created connection should have an id")
	require.Equal(t, connName, createRes.Connection.Name)
	require.Equal(t, "postgres", createRes.Connection.AdapterType)
	connID := createRes.Connection.ID

	// LIST
	var listRes struct {
		Connections []Connection `json:"connections"`
		Meta        any          `json:"meta"`
	}
	err = client.JSON("GET", fmt.Sprintf("/v1/connections?tenant_id=default&workspace_id=%s", workspaceID), nil, &listRes)
	require.NoError(t, err, "list connections should succeed")
	found := false
	for _, c := range listRes.Connections {
		if c.ID == connID {
			found = true
			break
		}
	}
	require.True(t, found, "created connection should appear in list")

	// GET
	var getRes struct {
		Connection Connection `json:"connection"`
	}
	err = client.JSON("GET", fmt.Sprintf("/v1/connections/%s", connID), nil, &getRes)
	require.NoError(t, err, "get connection should succeed")
	require.Equal(t, connID, getRes.Connection.ID)
	require.Equal(t, connName, getRes.Connection.Name)

	// DELETE
	var delRes struct {
		Deleted bool `json:"deleted"`
	}
	err = client.JSON("DELETE", fmt.Sprintf("/v1/connections/%s", connID), nil, &delRes)
	require.NoError(t, err, "delete connection should succeed")

	// VERIFY DELETE
	err = client.JSON("GET", fmt.Sprintf("/v1/connections/%s", connID), nil, &getRes)
	require.Error(t, err, "getting deleted connection should fail")
}

func TestAdmin_Plugins_Register(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewHTTPClient(GatewayBaseURL)

	payload := map[string]any{
		"plugin_type": "adapter",
		"name":        fmt.Sprintf("e2e-test-adapter-%d", time.Now().Unix()),
		"version":     "1.0.0",
		"author":      "e2e",
		"endpoint":    "localhost:50051",
		"protocol":    "grpc",
		"capabilities": []map[string]any{
			{
				"capability_name": "test.query",
				"capability_schema": map[string]any{
					"type": "object",
				},
			},
		},
	}

	var res struct {
		PluginID string `json:"plugin_id"`
	}
	err := client.JSON("POST", "/v1/plugins/register", payload, &res)
	require.NoError(t, err, "register plugin should succeed")
	require.NotEmpty(t, res.PluginID, "plugin_id should be returned")
}
