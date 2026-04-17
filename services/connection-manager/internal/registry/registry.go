package registry

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/pkg/shared/adapter"
	a "n0/services/connection-manager/internal/adapter"
)

// Registry holds built-in and external adapters.
type Registry struct {
	mu       sync.RWMutex
	builtins map[string]adapter.Adapter
	external map[string]pb.DatabaseAdapterClient
	log      *zap.Logger
}

// NewRegistry creates a new adapter registry.
func NewRegistry(log *zap.Logger) *Registry {
	r := &Registry{
		builtins: make(map[string]adapter.Adapter),
		external: make(map[string]pb.DatabaseAdapterClient),
		log:      log,
	}
	r.registerBuiltins()
	return r
}

func (r *Registry) registerBuiltins() {
	r.builtins["postgres"] = a.NewPostgresAdapter()
	r.builtins["clickhouse"] = a.NewClickHouseAdapter()
	r.builtins["mysql"] = a.NewMySQLAdapter()
	r.builtins["sqlite"] = a.NewSQLiteAdapter()
	r.builtins["mssql"] = a.NewMSSQLAdapter()
	r.builtins["bigquery"] = a.NewBigQueryAdapter()
}

// RegisterExternal adds an external gRPC adapter.
func (r *Registry) RegisterExternal(adapterType string, client pb.DatabaseAdapterClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.external[adapterType] = client
	r.log.Info("registered external adapter", zap.String("type", adapterType))
}

// Get returns the adapter for the given type.
func (r *Registry) Get(adapterType string) (adapter.Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if a, ok := r.builtins[adapterType]; ok {
		return a, nil
	}
	if c, ok := r.external[adapterType]; ok {
		return &externalAdapter{client: c}, nil
	}
	return nil, fmt.Errorf("unknown adapter type: %s", adapterType)
}

// List returns available adapter types.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for k := range r.builtins {
		out = append(out, k)
	}
	for k := range r.external {
		out = append(out, k)
	}
	return out
}

// externalAdapter wraps a gRPC DatabaseAdapterClient to implement Adapter.
type externalAdapter struct {
	client pb.DatabaseAdapterClient
}

func (a *externalAdapter) TestConnection(ctx context.Context, dsn string) error {
	_, err := a.client.TestConnection(ctx, &pb.AdapterTestConnectionRequest{
		Params: nil, // TODO: map DSN to params
	})
	return err
}

func (a *externalAdapter) GetSchema(ctx context.Context, dsn string) ([]adapter.TableInfo, error) {
	resp, err := a.client.GetSchema(ctx, &pb.AdapterGetSchemaRequest{
		Params: nil, // TODO
	})
	if err != nil {
		return nil, err
	}
	var out []adapter.TableInfo
	for _, t := range resp.Tables {
		var cols []adapter.ColumnInfo
		for _, c := range t.Columns {
			cols = append(cols, adapter.ColumnInfo{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
			})
		}
		out = append(out, adapter.TableInfo{Name: t.Name, Columns: cols})
	}
	return out, nil
}

func (a *externalAdapter) Prepare(connectionID string, dsn string) error {
	// External adapters manage their own pools via the remote endpoint.
	return nil
}

func (a *externalAdapter) ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error) {
	resp, err := a.client.ExecuteQuery(ctx, &pb.AdapterExecuteQueryRequest{
		Query:   sql,
		Options: nil,
	})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, r := range resp.Rows {
		row := make(map[string]any, len(resp.Columns))
		for i, col := range resp.Columns {
			if i < len(r.Values) {
				row[col] = r.Values[i]
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func (a *externalAdapter) Release(connectionID string) error {
	return nil
}
