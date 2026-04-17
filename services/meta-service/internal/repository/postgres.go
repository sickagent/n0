package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"n0/services/meta-service/internal/app"
)

// PostgresRepository provides PostgreSQL-backed persistence.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a repository from a DSN.
func NewPostgresRepositoryFromDSN(dsn string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

// Close closes the underlying pool.
func (r *PostgresRepository) Close() {
	r.pool.Close()
}

// ListWorkspaces returns workspaces for a user.
func (r *PostgresRepository) ListWorkspaces(ctx context.Context, userID string, limit, offset int) ([]app.Workspace, error) {
	const q = `SELECT id, tenant_id, name, created_at FROM workspaces WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query workspaces: %w", err)
	}
	defer rows.Close()

	var out []app.Workspace
	for rows.Next() {
		var w app.Workspace
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return out, nil
}

// GetSchemaSnapshot returns the latest schema snapshot for a connection.
func (r *PostgresRepository) GetSchemaSnapshot(ctx context.Context, connectionID string) (*app.SchemaSnapshot, error) {
	const q = `SELECT tables, captured_at FROM schema_snapshots WHERE connection_id = $1 ORDER BY captured_at DESC LIMIT 1`
	var snap app.SchemaSnapshot
	snap.ConnectionID = connectionID
	var tablesJSON []byte
	if err := r.pool.QueryRow(ctx, q, connectionID).Scan(&tablesJSON, &snap.CapturedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query schema snapshot: %w", err)
	}
	if err := json.Unmarshal(tablesJSON, &snap.Tables); err != nil {
		return nil, fmt.Errorf("unmarshal tables: %w", err)
	}
	return &snap, nil
}

// SaveSchemaSnapshot inserts a new schema snapshot for a connection.
func (r *PostgresRepository) SaveSchemaSnapshot(ctx context.Context, connectionID string, tables []app.TableInfo) error {
	const q = `INSERT INTO schema_snapshots (connection_id, tables) VALUES ($1, $2)`
	tablesJSON, err := json.Marshal(tables)
	if err != nil {
		return fmt.Errorf("marshal tables: %w", err)
	}
	if _, err := r.pool.Exec(ctx, q, connectionID, tablesJSON); err != nil {
		return fmt.Errorf("insert schema snapshot: %w", err)
	}
	return nil
}

// CreateConnection inserts a new connection.
func (r *PostgresRepository) CreateConnection(ctx context.Context, c app.Connection) (uuid.UUID, error) {
	const q = `
		INSERT INTO connections (workspace_id, user_id, tenant_id, name, adapter_type, config)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	configJSON, err := json.Marshal(c.Params)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal config: %w", err)
	}
	wsID, err := uuid.Parse(c.WorkspaceID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse workspace_id: %w", err)
	}
	userID, err := uuid.Parse(c.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse user_id: %w", err)
	}
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, wsID, userID, c.TenantID, c.Name, c.AdapterType, configJSON).Scan(&id); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return uuid.Nil, fmt.Errorf("connection already exists")
		}
		return uuid.Nil, fmt.Errorf("insert connection: %w", err)
	}
	return id, nil
}

// GetConnection returns a connection by ID.
func (r *PostgresRepository) GetConnection(ctx context.Context, connectionID string) (*app.Connection, error) {
	const q = `SELECT id, workspace_id, tenant_id, name, adapter_type, config, created_at FROM connections WHERE id = $1`
	var c app.Connection
	var configJSON []byte
	var id, wsID uuid.UUID
	if err := r.pool.QueryRow(ctx, q, connectionID).Scan(&id, &wsID, &c.TenantID, &c.Name, &c.AdapterType, &configJSON, &c.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query connection: %w", err)
	}
	c.ID = id
	c.WorkspaceID = wsID.String()
	if err := json.Unmarshal(configJSON, &c.Params); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &c, nil
}

// ListConnections returns connections for a user/workspace.
func (r *PostgresRepository) ListConnections(ctx context.Context, userID, workspaceID string, limit, offset int) ([]app.Connection, error) {
	const q = `
		SELECT id, workspace_id, tenant_id, name, adapter_type, config, created_at
		FROM connections
		WHERE user_id = $1 AND ($2::uuid IS NULL OR workspace_id = $2::uuid)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	var wsID *uuid.UUID
	if workspaceID != "" {
		parsed, err := uuid.Parse(workspaceID)
		if err != nil {
			return nil, fmt.Errorf("parse workspace_id: %w", err)
		}
		wsID = &parsed
	}
	rows, err := r.pool.Query(ctx, q, userID, wsID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query connections: %w", err)
	}
	defer rows.Close()

	var out []app.Connection
	for rows.Next() {
		var c app.Connection
		var configJSON []byte
		var id, ws uuid.UUID
		if err := rows.Scan(&id, &ws, &c.TenantID, &c.Name, &c.AdapterType, &configJSON, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		c.ID = id
		c.WorkspaceID = ws.String()
		if err := json.Unmarshal(configJSON, &c.Params); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connections: %w", err)
	}
	return out, nil
}

// DeleteConnection removes a connection.
func (r *PostgresRepository) DeleteConnection(ctx context.Context, connectionID string) error {
	const q = `DELETE FROM connections WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, connectionID); err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	return nil
}

// RegisterPlugin inserts a new plugin definition.
func (r *PostgresRepository) RegisterPlugin(ctx context.Context, p app.PluginDefinition) (uuid.UUID, error) {
	const q = `
		INSERT INTO plugin_definitions (plugin_type, name, version, author, endpoint, protocol, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, p.PluginType, p.Name, p.Version, p.Author, p.Endpoint, p.Protocol, p.Status).Scan(&id); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return uuid.Nil, fmt.Errorf("plugin already exists")
		}
		return uuid.Nil, fmt.Errorf("insert plugin: %w", err)
	}
	return id, nil
}

// CreateUser inserts a new user.
func (r *PostgresRepository) CreateUser(ctx context.Context, email, passwordHash, role string) (uuid.UUID, error) {
	const q = `INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, email, passwordHash, role).Scan(&id); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return uuid.Nil, fmt.Errorf("user already exists")
		}
		return uuid.Nil, fmt.Errorf("insert user: %w", err)
	}
	return id, nil
}

// GetUserByEmail fetches a user by email.
func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (*app.User, error) {
	const q = `SELECT id, email, password_hash, role, created_at FROM users WHERE email = $1`
	var u app.User
	if err := r.pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

// GetUserByID fetches a user by ID.
func (r *PostgresRepository) GetUserByID(ctx context.Context, id string) (*app.User, error) {
	const q = `SELECT id, email, password_hash, role, created_at FROM users WHERE id = $1`
	var u app.User
	if err := r.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &u, nil
}

// CreateAgent inserts a new agent.
func (r *PostgresRepository) CreateAgent(ctx context.Context, userID uuid.UUID, name string) (uuid.UUID, error) {
	const q = `INSERT INTO agents (user_id, name) VALUES ($1, $2) RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, q, userID, name).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("insert agent: %w", err)
	}
	return id, nil
}

// ListAgentsByUser returns agents for a user.
func (r *PostgresRepository) ListAgentsByUser(ctx context.Context, userID string) ([]app.Agent, error) {
	const q = `SELECT id, user_id, name, token_jti, status, created_at FROM agents WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var out []app.Agent
	for rows.Next() {
		var a app.Agent
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.TokenJTI, &a.Status, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return out, nil
}

// GetAgent returns an agent by ID.
func (r *PostgresRepository) GetAgent(ctx context.Context, id string) (*app.Agent, error) {
	const q = `SELECT id, user_id, name, token_jti, status, created_at FROM agents WHERE id = $1`
	var a app.Agent
	if err := r.pool.QueryRow(ctx, q, id).Scan(&a.ID, &a.UserID, &a.Name, &a.TokenJTI, &a.Status, &a.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query agent: %w", err)
	}
	return &a, nil
}

// UpdateAgentTokenJTI updates the token JTI for an agent.
func (r *PostgresRepository) UpdateAgentTokenJTI(ctx context.Context, id string, jti string) error {
	const q = `UPDATE agents SET token_jti = $1 WHERE id = $2`
	if _, err := r.pool.Exec(ctx, q, jti, id); err != nil {
		return fmt.Errorf("update agent jti: %w", err)
	}
	return nil
}
