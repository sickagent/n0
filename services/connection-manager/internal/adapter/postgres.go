package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"n0/pkg/shared/adapter"
)

// PostgresAdapter implements a built-in adapter for PostgreSQL.
type PostgresAdapter struct {
	pools map[string]*pgxpool.Pool
}

// NewPostgresAdapter creates a new PostgreSQL adapter.
func NewPostgresAdapter() *PostgresAdapter {
	return &PostgresAdapter{pools: make(map[string]*pgxpool.Pool)}
}

// TestConnection validates connectivity with the provided DSN.
func (a *PostgresAdapter) TestConnection(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// Prepare ensures a connection pool exists for the given connection ID.
func (a *PostgresAdapter) Prepare(connectionID string, dsn string) error {
	if _, ok := a.pools[connectionID]; ok {
		return nil
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	a.pools[connectionID] = pool
	return nil
}

// Release closes the pool associated with the connection ID.
func (a *PostgresAdapter) Release(connectionID string) error {
	pool, ok := a.pools[connectionID]
	if !ok {
		return nil
	}
	pool.Close()
	delete(a.pools, connectionID)
	return nil
}

// ExecuteQuery runs a read-only query and returns rows as a slice of maps.
func (a *PostgresAdapter) ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error) {
	pool, ok := a.pools[connectionID]
	if !ok {
		return nil, fmt.Errorf("pool not found for connection %s", connectionID)
	}

	if limit > 0 {
		sql = fmt.Sprintf("%s LIMIT %d", sql, limit)
	}

	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query execution: %w", err)
	}
	defer rows.Close()

	descriptions := rows.FieldDescriptions()
	colNames := make([]string, len(descriptions))
	for i, fd := range descriptions {
		colNames[i] = fd.Name
	}

	var result []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("read values: %w", err)
		}
		rowMap := make(map[string]any, len(colNames))
		for i, name := range colNames {
			rowMap[name] = values[i]
		}
		result = append(result, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration: %w", err)
	}
	return result, nil
}

// GetSchema introspects the database and returns table/column metadata.
func (a *PostgresAdapter) GetSchema(ctx context.Context, dsn string) ([]adapter.TableInfo, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	const q = `
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position
	`
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("schema query: %w", err)
	}
	defer rows.Close()

	tables := make(map[string]*adapter.TableInfo)
	for rows.Next() {
		var tableName, colName, dataType, isNullable string
		if err := rows.Scan(&tableName, &colName, &dataType, &isNullable); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		t, ok := tables[tableName]
		if !ok {
			t = &adapter.TableInfo{Name: tableName}
			tables[tableName] = t
		}
		t.Columns = append(t.Columns, adapter.ColumnInfo{
			Name:     colName,
			DataType: dataType,
			Nullable: isNullable == "YES",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration: %w", err)
	}

	result := make([]adapter.TableInfo, 0, len(tables))
	for _, t := range tables {
		result = append(result, *t)
	}
	return result, nil
}
