package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"n0/pkg/shared/adapter"
)

// MySQLAdapter implements a built-in adapter for MySQL.
type MySQLAdapter struct {
	pools map[string]*sql.DB
}

// NewMySQLAdapter creates a new MySQL adapter.
func NewMySQLAdapter() *MySQLAdapter {
	return &MySQLAdapter{pools: make(map[string]*sql.DB)}
}

// TestConnection validates connectivity with the provided DSN.
func (a *MySQLAdapter) TestConnection(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// Prepare ensures a connection pool exists for the given connection ID.
func (a *MySQLAdapter) Prepare(connectionID string, dsn string) error {
	if _, ok := a.pools[connectionID]; ok {
		return nil
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	a.pools[connectionID] = db
	return nil
}

// Release closes the pool associated with the connection ID.
func (a *MySQLAdapter) Release(connectionID string) error {
	pool, ok := a.pools[connectionID]
	if !ok {
		return nil
	}
	pool.Close()
	delete(a.pools, connectionID)
	return nil
}

// ExecuteQuery runs a read-only query and returns rows as a slice of maps.
func (a *MySQLAdapter) ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error) {
	db, ok := a.pools[connectionID]
	if !ok {
		return nil, fmt.Errorf("pool not found for connection %s", connectionID)
	}

	if limit > 0 {
		sql = fmt.Sprintf("%s LIMIT %d", sql, limit)
	}

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query execution: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rowMap := make(map[string]any, len(cols))
		for i, name := range cols {
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
func (a *MySQLAdapter) GetSchema(ctx context.Context, dsn string) ([]adapter.TableInfo, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	const q = `
		SELECT table_name, column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
	`
	rows, err := db.QueryContext(ctx, q)
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
