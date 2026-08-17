package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
)

type mcpConnectionInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"connection identifier"`
}

type mcpSubmitQueryInput struct {
	ConnectionID string `json:"connection_id" jsonschema:"connection identifier"`
	SQL          string `json:"sql" jsonschema:"a single read-only SELECT statement"`
}

type mcpJobInput struct {
	JobID string `json:"job_id" jsonschema:"query job identifier"`
}

type mcpResultInput struct {
	JobID    string `json:"job_id" jsonschema:"query job identifier"`
	Page     int    `json:"page,omitempty" jsonschema:"zero-based result page"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"number of rows to return"`
}

type mcpConnectionsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty" jsonschema:"optional workspace filter"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of connections"`
	Offset      int    `json:"offset,omitempty" jsonschema:"pagination offset"`
}

type mcpWorkspacesInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"maximum number of workspaces"`
	Offset int `json:"offset,omitempty" jsonschema:"pagination offset"`
}

type mcpSchemaOutput struct {
	Snapshot map[string]any `json:"snapshot"`
}

type mcpSubmitQueryOutput struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type mcpJobStatusOutput struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type mcpQueryResultOutput struct {
	JobID         string           `json:"job_id"`
	Rows          []map[string]any `json:"rows"`
	NextPageToken string           `json:"next_page_token,omitempty"`
	Truncated     bool             `json:"truncated"`
}

type mcpConnectionOutput struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	AdapterType string `json:"adapter_type"`
}

type mcpConnectionsOutput struct {
	Connections []mcpConnectionOutput `json:"connections"`
}

type mcpWorkspaceOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mcpWorkspacesOutput struct {
	Workspaces []mcpWorkspaceOutput `json:"workspaces"`
}

func (s *Server) newMCPHandler() http.Handler {
	server := mcp.NewServer(&mcp.Implementation{
		Name:        "n0-agent-gateway",
		Title:       "n0 Agent Gateway",
		Description: "Tenant-scoped read-only data access for AI agents",
		Version:     "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Use get_schema before submit_query. Queries must be read-only SELECT statements. Poll get_query_status and then fetch pages with get_query_result.",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_schema",
		Description: "Return the tables and columns visible for a connection in the authenticated tenant.",
	}, s.mcpGetSchema)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "submit_query",
		Description: "Submit a tenant-scoped read-only SQL query and return its asynchronous job id.",
	}, s.mcpSubmitQuery)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_query_status",
		Description: "Read the status of a query job belonging to the authenticated tenant.",
	}, s.mcpGetJobStatus)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_query_result",
		Description: "Read one paginated result page for a query job belonging to the authenticated tenant.",
	}, s.mcpGetJobResult)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_connections",
		Description: "List connections visible in the authenticated tenant without returning credentials.",
	}, s.mcpListConnections)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List workspaces visible in the authenticated tenant.",
	}, s.mcpListWorkspaces)

	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		MaxRequestBodyBytes:          1 << 20,
		PropagateRequestCancellation: true,
	})
}

func (s *Server) mcpGetSchema(ctx context.Context, _ *mcp.CallToolRequest, input mcpConnectionInput) (*mcp.CallToolResult, mcpSchemaOutput, error) {
	connectionID, err := requiredMCPValue("connection_id", input.ConnectionID)
	if err != nil {
		return nil, mcpSchemaOutput{}, err
	}
	resp, err := s.metaCli.GetSchema(ctx, &pb.GetSchemaRequest{ConnectionId: connectionID, TenantId: mcpTenantID(ctx)})
	if err != nil {
		return nil, mcpSchemaOutput{}, err
	}
	if resp == nil || resp.Snapshot == nil {
		return nil, mcpSchemaOutput{}, fmt.Errorf("schema not found for connection %q", connectionID)
	}
	return nil, mcpSchemaOutput{Snapshot: protoJSONMap(resp.Snapshot)}, nil
}

func (s *Server) mcpSubmitQuery(ctx context.Context, _ *mcp.CallToolRequest, input mcpSubmitQueryInput) (*mcp.CallToolResult, mcpSubmitQueryOutput, error) {
	connectionID, err := requiredMCPValue("connection_id", input.ConnectionID)
	if err != nil {
		return nil, mcpSubmitQueryOutput{}, err
	}
	sql, err := requiredMCPValue("sql", input.SQL)
	if err != nil {
		return nil, mcpSubmitQueryOutput{}, err
	}
	resp, err := s.queryCli.SubmitQuery(ctx, &pb.SubmitQueryRequest{TenantId: mcpTenantID(ctx), ConnectionId: connectionID, Sql: sql})
	if err != nil {
		return nil, mcpSubmitQueryOutput{}, err
	}
	if resp == nil {
		return nil, mcpSubmitQueryOutput{}, fmt.Errorf("query service returned an empty response")
	}
	return nil, mcpSubmitQueryOutput{JobID: resp.JobId, Status: resp.Status}, nil
}

func (s *Server) mcpGetJobStatus(ctx context.Context, _ *mcp.CallToolRequest, input mcpJobInput) (*mcp.CallToolResult, mcpJobStatusOutput, error) {
	jobID, err := requiredMCPValue("job_id", input.JobID)
	if err != nil {
		return nil, mcpJobStatusOutput{}, err
	}
	resp, err := s.queryCli.GetJobStatus(ctx, &pb.GetJobStatusRequest{JobId: jobID, TenantId: mcpTenantID(ctx)})
	if err != nil {
		return nil, mcpJobStatusOutput{}, err
	}
	if resp == nil {
		return nil, mcpJobStatusOutput{}, fmt.Errorf("query service returned an empty response")
	}
	return nil, mcpJobStatusOutput{JobID: resp.JobId, Status: resp.Status, Error: resp.ErrorMessage}, nil
}

func (s *Server) mcpGetJobResult(ctx context.Context, _ *mcp.CallToolRequest, input mcpResultInput) (*mcp.CallToolResult, mcpQueryResultOutput, error) {
	jobID, err := requiredMCPValue("job_id", input.JobID)
	if err != nil {
		return nil, mcpQueryResultOutput{}, err
	}
	resp, err := s.queryCli.GetJobResult(ctx, &pb.GetJobResultRequest{JobId: jobID, TenantId: mcpTenantID(ctx), Page: int32(input.Page), PageSize: int32(input.PageSize)})
	if err != nil {
		return nil, mcpQueryResultOutput{}, err
	}
	if resp == nil {
		return nil, mcpQueryResultOutput{}, fmt.Errorf("query service returned an empty response")
	}
	rows := make([]map[string]any, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		if row != nil {
			rows = append(rows, row.AsMap())
		}
	}
	return nil, mcpQueryResultOutput{JobID: resp.JobId, Rows: rows, NextPageToken: resp.NextPageToken, Truncated: resp.Truncated}, nil
}

func (s *Server) mcpListConnections(ctx context.Context, _ *mcp.CallToolRequest, input mcpConnectionsInput) (*mcp.CallToolResult, mcpConnectionsOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	resp, err := s.metaCli.ListConnections(ctx, &pb.ListConnectionsRequest{TenantId: mcpTenantID(ctx), WorkspaceId: input.WorkspaceID, Pagination: &pb.Pagination{Limit: int32(limit), Offset: int32(max(input.Offset, 0))}})
	if err != nil {
		return nil, mcpConnectionsOutput{}, err
	}
	out := mcpConnectionsOutput{Connections: make([]mcpConnectionOutput, 0)}
	if resp == nil {
		return nil, out, nil
	}
	for _, connection := range resp.Connections {
		if connection != nil {
			out.Connections = append(out.Connections, mcpConnectionOutput{ID: connection.Id, WorkspaceID: connection.WorkspaceId, Name: connection.Name, AdapterType: connection.AdapterType})
		}
	}
	return nil, out, nil
}

func (s *Server) mcpListWorkspaces(ctx context.Context, _ *mcp.CallToolRequest, input mcpWorkspacesInput) (*mcp.CallToolResult, mcpWorkspacesOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	resp, err := s.metaCli.ListWorkspaces(ctx, &pb.ListWorkspacesRequest{TenantId: mcpTenantID(ctx), Pagination: &pb.Pagination{Limit: int32(limit), Offset: int32(max(input.Offset, 0))}})
	if err != nil {
		return nil, mcpWorkspacesOutput{}, err
	}
	out := mcpWorkspacesOutput{Workspaces: make([]mcpWorkspaceOutput, 0)}
	if resp == nil {
		return nil, out, nil
	}
	for _, workspace := range resp.Workspaces {
		if workspace != nil {
			out.Workspaces = append(out.Workspaces, mcpWorkspaceOutput{ID: workspace.Id, Name: workspace.Name})
		}
	}
	return nil, out, nil
}

func mcpTenantID(ctx context.Context) string {
	if value := ctx.Value("user_id"); value != nil {
		if tenantID, ok := value.(string); ok {
			return tenantID
		}
	}
	return ""
}

func requiredMCPValue(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func protoJSONMap(value any) map[string]any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return map[string]any{}
	}
	return result
}
