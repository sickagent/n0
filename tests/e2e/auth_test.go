package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuth_RegisterLoginAndDefaultWorkspace(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewHTTPClient(GatewayBaseURL)

	email := fmt.Sprintf("e2e-auth-%d@example.com", time.Now().UnixNano())
	password := "secret123"

	var registerRes struct {
		UserID string `json:"user_id"`
	}
	err := client.JSON("POST", "/v1/auth/register", map[string]any{
		"email":    email,
		"password": password,
	}, &registerRes)
	require.NoError(t, err)
	require.NotEmpty(t, registerRes.UserID)

	var loginRes struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
		Token  string `json:"token"`
	}
	err = client.JSON("POST", "/v1/auth/login", map[string]any{
		"email":    email,
		"password": password,
	}, &loginRes)
	require.NoError(t, err)
	require.Equal(t, registerRes.UserID, loginRes.UserID)
	require.Equal(t, email, loginRes.Email)
	require.NotEmpty(t, loginRes.Token)

	authedClient := NewHTTPClient(GatewayBaseURL)
	authedClient.Token = loginRes.Token

	var workspacesRes struct {
		Workspaces []Workspace `json:"workspaces"`
	}
	err = authedClient.JSON("GET", "/v1/workspaces", nil, &workspacesRes)
	require.NoError(t, err)
	require.NotEmpty(t, workspacesRes.Workspaces)
	require.Equal(t, "Default Workspace", workspacesRes.Workspaces[0].Name)
}
