package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
)

type fakeMetaClient struct {
	schemaErr     error
	schemaResp    *pb.GetSchemaResponse
	workspacesErr error
}

func (c *fakeMetaClient) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	if c.schemaErr != nil {
		return nil, c.schemaErr
	}
	return c.schemaResp, nil
}

func (c *fakeMetaClient) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.ListWorkspacesResponse, error) {
	if c.workspacesErr != nil {
		return nil, c.workspacesErr
	}
	return &pb.ListWorkspacesResponse{Workspaces: []*pb.Workspace{}}, nil
}

func (c *fakeMetaClient) CreateConnection(ctx context.Context, req *pb.CreateConnectionRequest) (*pb.CreateConnectionResponse, error) {
	return &pb.CreateConnectionResponse{Connection: &pb.Connection{Id: "conn-new"}}, nil
}

func (c *fakeMetaClient) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error) {
	return &pb.GetConnectionResponse{Connection: &pb.Connection{Id: req.ConnectionId}}, nil
}

func (c *fakeMetaClient) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	return &pb.ListConnectionsResponse{Connections: []*pb.Connection{}}, nil
}

func (c *fakeMetaClient) DeleteConnection(ctx context.Context, req *pb.DeleteConnectionRequest) (*pb.DeleteConnectionResponse, error) {
	return &pb.DeleteConnectionResponse{Deleted: true}, nil
}

func (c *fakeMetaClient) RegisterPlugin(ctx context.Context, req *pb.RegisterPluginRequest) (*pb.RegisterPluginResponse, error) {
	return &pb.RegisterPluginResponse{PluginId: "plugin-1"}, nil
}

func (c *fakeMetaClient) Close() error { return nil }

type fakeQueryClient struct {
	submitResp *pb.SubmitQueryResponse
	submitErr  error
}

func (c *fakeQueryClient) SubmitQuery(ctx context.Context, req *pb.SubmitQueryRequest) (*pb.SubmitQueryResponse, error) {
	if c.submitErr != nil {
		return nil, c.submitErr
	}
	return c.submitResp, nil
}

func (c *fakeQueryClient) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	return &pb.GetJobStatusResponse{JobId: req.JobId, Status: "pending"}, nil
}

func (c *fakeQueryClient) GetJobResult(ctx context.Context, req *pb.GetJobResultRequest) (*pb.GetJobResultResponse, error) {
	return &pb.GetJobResultResponse{JobId: req.JobId}, nil
}

func (c *fakeQueryClient) Close() error { return nil }

type fakeCMClient struct{}

func (c *fakeCMClient) TestConnection(ctx context.Context, req *pb.TestConnectionRequest) (*pb.TestConnectionResponse, error) {
	return &pb.TestConnectionResponse{Ok: true}, nil
}

func (c *fakeCMClient) GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error) {
	return &pb.GetConnectionSchemaResponse{Tables: []*pb.Table{}}, nil
}

func (c *fakeCMClient) ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error) {
	return &pb.ExecuteQueryResponse{}, nil
}

func TestServer_Health(t *testing.T) {
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, &fakeQueryClient{}, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestServer_GetSchema(t *testing.T) {
	meta := &fakeMetaClient{
		schemaResp: &pb.GetSchemaResponse{
			Snapshot: &pb.SchemaSnapshot{
				ConnectionId: "conn-1",
				Tables: []*pb.Table{
					{Name: "users", Columns: []*pb.Column{{Name: "id", DataType: "int"}}},
				},
			},
		},
	}
	srv := NewServer(":0", ":0", zap.NewNop(), meta, &fakeQueryClient{}, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/schema?connection_id=conn-1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp pb.GetSchemaResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Snapshot.ConnectionId != "conn-1" {
		t.Errorf("expected conn-1, got %s", resp.Snapshot.ConnectionId)
	}
}

func TestServer_GetSchema_Error(t *testing.T) {
	meta := &fakeMetaClient{schemaErr: errors.New("boom")}
	srv := NewServer(":0", ":0", zap.NewNop(), meta, &fakeQueryClient{}, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/schema?connection_id=conn-1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestServer_SubmitQuery(t *testing.T) {
	query := &fakeQueryClient{
		submitResp: &pb.SubmitQueryResponse{JobId: "job-1", Status: "pending"},
	}
	srv := NewServer(":0", ":0", zap.NewNop(), &fakeMetaClient{}, query, &fakeCMClient{}, nil)
	r := srv.handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/query?tenant_id=t1&connection_id=c1&sql=SELECT+1", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp pb.SubmitQueryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.JobId != "job-1" {
		t.Errorf("expected job-1, got %s", resp.JobId)
	}
}
