package server

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/meta-service/internal/app"
)

type fakeRepo struct {
	lastConnection app.Connection
	connectionID   uuid.UUID
	connection     *app.Connection
}

func (r *fakeRepo) ListWorkspaces(ctx context.Context, userID string, limit, offset int) ([]app.Workspace, error) {
	return nil, nil
}

func (r *fakeRepo) GetSchemaSnapshot(ctx context.Context, connectionID string) (*app.SchemaSnapshot, error) {
	return nil, nil
}

func (r *fakeRepo) SaveSchemaSnapshot(ctx context.Context, connectionID string, tables []app.TableInfo) error {
	return nil
}

func (r *fakeRepo) CreateConnection(ctx context.Context, c app.Connection) (uuid.UUID, error) {
	r.lastConnection = c
	if r.connectionID == uuid.Nil {
		r.connectionID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	}
	return r.connectionID, nil
}

func (r *fakeRepo) GetConnection(ctx context.Context, connectionID string) (*app.Connection, error) {
	if r.connection != nil {
		return r.connection, nil
	}
	return &app.Connection{
		ID:          r.connectionID,
		WorkspaceID: "22222222-2222-2222-2222-222222222222",
		UserID:      r.lastConnection.UserID,
		TenantID:    r.lastConnection.TenantID,
		Name:        r.lastConnection.Name,
		AdapterType: r.lastConnection.AdapterType,
		Params:      r.lastConnection.Params,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (r *fakeRepo) ListConnections(ctx context.Context, userID, workspaceID string, limit, offset int) ([]app.Connection, error) {
	return nil, nil
}

func (r *fakeRepo) DeleteConnection(ctx context.Context, connectionID string) error {
	return nil
}

func (r *fakeRepo) RegisterPlugin(ctx context.Context, p app.PluginDefinition) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (r *fakeRepo) CreateUser(ctx context.Context, email, passwordHash, passwordSalt, role string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (r *fakeRepo) GetUserByEmail(ctx context.Context, email string) (*app.User, error) {
	return nil, nil
}

func (r *fakeRepo) GetUserByID(ctx context.Context, id string) (*app.User, error) {
	return nil, nil
}

func (r *fakeRepo) CreateWorkspace(ctx context.Context, userID, tenantID, name string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (r *fakeRepo) CreateAgent(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (r *fakeRepo) ListAgentsByUser(ctx context.Context, userID string) ([]app.Agent, error) {
	return nil, nil
}

func (r *fakeRepo) GetAgent(ctx context.Context, id string) (*app.Agent, error) {
	return nil, nil
}

func (r *fakeRepo) UpdateAgentTokenJTI(ctx context.Context, id string, jti string) error {
	return nil
}

type fakeCM struct{}

func (c *fakeCM) GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error) {
	return &pb.GetConnectionSchemaResponse{}, nil
}

func TestGRPCServer_CreateConnectionUsesTenantAsUserID(t *testing.T) {
	repo := &fakeRepo{}
	svc := app.NewMetaService(repo, &fakeCM{}, nil)
	server := NewGRPCServer(svc)

	params, err := structpb.NewStruct(map[string]any{"host": "postgres"})
	if err != nil {
		t.Fatalf("new struct: %v", err)
	}

	resp, err := server.CreateConnection(context.Background(), &pb.CreateConnectionRequest{
		WorkspaceId: "22222222-2222-2222-2222-222222222222",
		TenantId:    "00000000-0000-0000-0000-000000000001",
		Name:        "test-conn",
		AdapterType: "postgres",
		Params:      params,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if repo.lastConnection.UserID != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("expected user id to be propagated, got %q", repo.lastConnection.UserID)
	}
	if resp.Connection == nil || resp.Connection.Id == "" {
		t.Fatalf("expected connection in response")
	}
}
