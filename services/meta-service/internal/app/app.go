package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/structpb"
	"n0/pkg/shared/crypto"
	pb "n0/proto/gen/go/lensagent/v1"
)

// Repository defines persistence operations required by Meta Service.
type Repository interface {
	ListWorkspaces(ctx context.Context, userID string, limit, offset int) ([]Workspace, error)
	GetSchemaSnapshot(ctx context.Context, connectionID string) (*SchemaSnapshot, error)
	SaveSchemaSnapshot(ctx context.Context, connectionID string, tables []TableInfo) error
	CreateConnection(ctx context.Context, c Connection) (uuid.UUID, error)
	GetConnection(ctx context.Context, connectionID string) (*Connection, error)
	ListConnections(ctx context.Context, userID, workspaceID string, limit, offset int) ([]Connection, error)
	DeleteConnection(ctx context.Context, connectionID string) error
	RegisterPlugin(ctx context.Context, p PluginDefinition) (uuid.UUID, error)

	// Auth / Users
	CreateUser(ctx context.Context, email, passwordHash, passwordSalt, role string) (uuid.UUID, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	CreateWorkspace(ctx context.Context, userID, tenantID, name string) (uuid.UUID, error)

	// Agents
	CreateAgent(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error)
	ListAgentsByUser(ctx context.Context, userID string) ([]Agent, error)
	GetAgent(ctx context.Context, id string) (*Agent, error)
	UpdateAgentTokenJTI(ctx context.Context, id string, jti string) error
}

// Workspace represents a tenant-scoped data workspace.
type Workspace struct {
	ID        uuid.UUID
	TenantID  string
	Name      string
	CreatedAt time.Time
}

// SchemaSnapshot is a cached view of a connection schema.
type SchemaSnapshot struct {
	ConnectionID string
	Tables       []TableInfo
	CapturedAt   time.Time
}

// TableInfo describes a table in a schema snapshot.
type TableInfo struct {
	Name    string
	Columns []ColumnInfo
}

// ColumnInfo describes a column.
type ColumnInfo struct {
	Name     string
	DataType string
	Nullable bool
}

// Connection represents a database connection configuration.
type Connection struct {
	ID          uuid.UUID
	WorkspaceID string
	UserID      string
	TenantID    string
	Name        string
	AdapterType string
	Params      map[string]any
	CreatedAt   time.Time
}

// PluginDefinition represents a registered plugin.
type PluginDefinition struct {
	ID         uuid.UUID
	PluginType string
	Name       string
	Version    string
	Author     string
	Endpoint   string
	Protocol   string
	Status     string
	CreatedAt  time.Time
}

// User represents a platform user.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	PasswordSalt string
	Role         string
	CreatedAt    time.Time
}

// Agent represents a registered agent belonging to a user.
type Agent struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	TokenJTI  *string
	Status    string
	CreatedAt time.Time
}

// CMClient defines the subset of ConnectionManager client used by MetaService.
type CMClient interface {
	GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error)
}

// MetaService implements core business logic.
type MetaService struct {
	repo   Repository
	cmCli  CMClient
	crypto *crypto.Encrypter
}

// NewMetaService creates a new MetaService instance.
func NewMetaService(repo Repository, cmCli CMClient, cr *crypto.Encrypter) *MetaService {
	return &MetaService{repo: repo, cmCli: cmCli, crypto: cr}
}

// GetSchema returns the latest schema snapshot for a connection.
// If no cached snapshot exists, it fetches fresh schema from the connection manager and persists it.
func (s *MetaService) GetSchema(ctx context.Context, connectionID string) (*SchemaSnapshot, error) {
	snap, err := s.repo.GetSchemaSnapshot(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get schema snapshot: %w", err)
	}
	if snap != nil {
		return snap, nil
	}

	// No cached snapshot — fetch from connection manager.
	conn, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if conn == nil {
		return nil, fmt.Errorf("connection not found: %s", connectionID)
	}

	decryptedParams, err := s.crypto.DecryptJSON(conn.Params)
	if err != nil {
		return nil, fmt.Errorf("decrypt params: %w", err)
	}

	params, err := structpb.NewStruct(decryptedParams)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	resp, err := s.cmCli.GetSchema(ctx, &pb.GetConnectionSchemaRequest{
		ConnectionId: connectionID,
		AdapterType:  conn.AdapterType,
		Params:       params,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch schema from cm: %w", err)
	}

	var tables []TableInfo
	for _, t := range resp.Tables {
		var cols []ColumnInfo
		for _, c := range t.Columns {
			cols = append(cols, ColumnInfo{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
			})
		}
		tables = append(tables, TableInfo{
			Name:    t.Name,
			Columns: cols,
		})
	}

	if err := s.repo.SaveSchemaSnapshot(ctx, connectionID, tables); err != nil {
		return nil, fmt.Errorf("save schema snapshot: %w", err)
	}

	return &SchemaSnapshot{
		ConnectionID: connectionID,
		Tables:       tables,
		CapturedAt:   time.Now().UTC(),
	}, nil
}

// ListWorkspaces returns workspaces accessible to the user.
func (s *MetaService) ListWorkspaces(ctx context.Context, userID string, limit, offset int) ([]Workspace, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repo.ListWorkspaces(ctx, userID, limit, offset)
}

// CreateConnection creates a new connection configuration.
func (s *MetaService) CreateConnection(ctx context.Context, c Connection) (uuid.UUID, error) {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	encryptedParams, err := s.crypto.EncryptJSON(c.Params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt params: %w", err)
	}
	c.Params = encryptedParams
	id, err := s.repo.CreateConnection(ctx, c)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create connection: %w", err)
	}
	return id, nil
}

// GetConnection returns a connection by ID.
func (s *MetaService) GetConnection(ctx context.Context, connectionID string) (*Connection, error) {
	c, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if c != nil {
		decryptedParams, err := s.crypto.DecryptJSON(c.Params)
		if err != nil {
			return nil, fmt.Errorf("decrypt params: %w", err)
		}
		c.Params = decryptedParams
	}
	return c, nil
}

// ListConnections returns connections for a user/workspace.
func (s *MetaService) ListConnections(ctx context.Context, userID, workspaceID string, limit, offset int) ([]Connection, error) {
	if limit <= 0 {
		limit = 20
	}
	conns, err := s.repo.ListConnections(ctx, userID, workspaceID, limit, offset)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		decryptedParams, err := s.crypto.DecryptJSON(conns[i].Params)
		if err != nil {
			return nil, fmt.Errorf("decrypt params for connection %s: %w", conns[i].ID, err)
		}
		conns[i].Params = decryptedParams
	}
	return conns, nil
}

// DeleteConnection removes a connection.
func (s *MetaService) DeleteConnection(ctx context.Context, connectionID string) error {
	if err := s.repo.DeleteConnection(ctx, connectionID); err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

// RegisterPlugin registers a new plugin and returns its generated ID.
func (s *MetaService) RegisterPlugin(ctx context.Context, p PluginDefinition) (uuid.UUID, error) {
	if p.Status == "" {
		p.Status = "registered"
	}
	if p.Protocol == "" {
		p.Protocol = "grpc"
	}
	p.CreatedAt = time.Now().UTC()
	id, err := s.repo.RegisterPlugin(ctx, p)
	if err != nil {
		return uuid.Nil, fmt.Errorf("register plugin: %w", err)
	}
	return id, nil
}

// RegisterUser creates a new user with a hashed password.
func (s *MetaService) RegisterUser(ctx context.Context, email, password, role string) (uuid.UUID, error) {
	if role == "" {
		role = "user"
	}
	salt, err := generatePasswordSalt()
	if err != nil {
		return uuid.Nil, fmt.Errorf("generate password salt: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(hashPasswordInput(password, salt)), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}
	id, err := s.repo.CreateUser(ctx, email, string(hash), salt, role)
	if err != nil {
		return uuid.Nil, err
	}
	if _, err := s.repo.CreateWorkspace(ctx, id.String(), id.String(), "Default Workspace"); err != nil {
		return uuid.Nil, fmt.Errorf("create default workspace: %w", err)
	}
	return id, nil
}

// LoginUser validates credentials and returns the user.
func (s *MetaService) LoginUser(ctx context.Context, email, password string) (*User, error) {
	u, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(hashPasswordInput(password, u.PasswordSalt))); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return u, nil
}

func generatePasswordSalt() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashPasswordInput(password, salt string) string {
	if salt == "" {
		return password
	}
	return password + ":" + salt
}

// CreateAgent registers a new agent for a user.
func (s *MetaService) CreateAgent(ctx context.Context, userID uuid.UUID, name string) (*Agent, error) {
	id, err := s.repo.CreateAgent(ctx, userID, name)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return s.repo.GetAgent(ctx, id.String())
}

// ListAgents returns agents belonging to a user.
func (s *MetaService) ListAgents(ctx context.Context, userID string) ([]Agent, error) {
	return s.repo.ListAgentsByUser(ctx, userID)
}

// GetUserByID returns a user by ID.
func (s *MetaService) GetUserByID(ctx context.Context, id string) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}

// UpdateAgentTokenJTI updates the token JTI for an agent.
func (s *MetaService) UpdateAgentTokenJTI(ctx context.Context, agentID string, jti string) error {
	return s.repo.UpdateAgentTokenJTI(ctx, agentID, jti)
}
