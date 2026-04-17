package adapter

import "context"

// TableInfo describes a database table.
type TableInfo struct {
	Name    string
	Columns []ColumnInfo
}

// ColumnInfo describes a database column.
type ColumnInfo struct {
	Name     string
	DataType string
	Nullable bool
}

// Adapter is the common interface for database adapters.
type Adapter interface {
	TestConnection(ctx context.Context, dsn string) error
	GetSchema(ctx context.Context, dsn string) ([]TableInfo, error)
	Prepare(connectionID string, dsn string) error
	ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error)
	Release(connectionID string) error
}
