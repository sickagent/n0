package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	createdUser struct {
		email        string
		passwordHash string
		passwordSalt string
		role         string
	}
	createdWorkspace struct {
		userID   string
		tenantID string
		name     string
	}
	user *User
}

func (r *fakeRepo) ListWorkspaces(ctx context.Context, userID string, limit, offset int) ([]Workspace, error) {
	return nil, nil
}

func (r *fakeRepo) GetSchemaSnapshot(ctx context.Context, connectionID string) (*SchemaSnapshot, error) {
	return nil, nil
}

func (r *fakeRepo) SaveSchemaSnapshot(ctx context.Context, connectionID string, tables []TableInfo) error {
	return nil
}

func (r *fakeRepo) CreateConnection(ctx context.Context, c Connection) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (r *fakeRepo) GetConnection(ctx context.Context, connectionID string) (*Connection, error) {
	return nil, nil
}

func (r *fakeRepo) ListConnections(ctx context.Context, userID, workspaceID string, limit, offset int) ([]Connection, error) {
	return nil, nil
}

func (r *fakeRepo) DeleteConnection(ctx context.Context, connectionID string) error {
	return nil
}

func (r *fakeRepo) RegisterPlugin(ctx context.Context, p PluginDefinition) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (r *fakeRepo) CreateUser(ctx context.Context, email, passwordHash, passwordSalt, role string) (uuid.UUID, error) {
	r.createdUser.email = email
	r.createdUser.passwordHash = passwordHash
	r.createdUser.passwordSalt = passwordSalt
	r.createdUser.role = role
	return uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), nil
}

func (r *fakeRepo) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return r.user, nil
}

func (r *fakeRepo) GetUserByID(ctx context.Context, id string) (*User, error) {
	return r.user, nil
}

func (r *fakeRepo) CreateWorkspace(ctx context.Context, userID, tenantID, name string) (uuid.UUID, error) {
	r.createdWorkspace.userID = userID
	r.createdWorkspace.tenantID = tenantID
	r.createdWorkspace.name = name
	return uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), nil
}

func (r *fakeRepo) CreateAgent(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (r *fakeRepo) ListAgentsByUser(ctx context.Context, userID string) ([]Agent, error) {
	return nil, nil
}

func (r *fakeRepo) GetAgent(ctx context.Context, id string) (*Agent, error) {
	return nil, nil
}

func (r *fakeRepo) UpdateAgentTokenJTI(ctx context.Context, id string, jti string) error {
	return nil
}

type fakeCMClient struct{}

func (c *fakeCMClient) GetSchema(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}

func TestRegisterUserStoresSaltAndCreatesDefaultWorkspace(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewMetaService(repo, nil, nil)

	id, err := svc.RegisterUser(context.Background(), "user@example.com", "secret123", "")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if id.String() != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("unexpected user id: %s", id)
	}
	if repo.createdUser.passwordHash == "" {
		t.Fatal("expected password hash to be stored")
	}
	if repo.createdUser.passwordSalt == "" {
		t.Fatal("expected password salt to be stored")
	}
	if repo.createdWorkspace.userID != id.String() {
		t.Fatalf("expected workspace user id %s, got %s", id, repo.createdWorkspace.userID)
	}
	if repo.createdWorkspace.name != "Default Workspace" {
		t.Fatalf("expected default workspace name, got %s", repo.createdWorkspace.name)
	}
}

func TestLoginUserUsesStoredSalt(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewMetaService(repo, nil, nil)

	_, err := svc.RegisterUser(context.Background(), "user@example.com", "secret123", "")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	repo.user = &User{
		ID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Email:        "user@example.com",
		PasswordHash: repo.createdUser.passwordHash,
		PasswordSalt: repo.createdUser.passwordSalt,
		Role:         "user",
	}

	user, err := svc.LoginUser(context.Background(), "user@example.com", "secret123")
	if err != nil {
		t.Fatalf("login user: %v", err)
	}
	if user == nil || user.Email != "user@example.com" {
		t.Fatalf("expected authenticated user, got %+v", user)
	}
}
