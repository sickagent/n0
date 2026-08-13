package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sickagent/n0/pkg/shared/adapter"
	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	a "github.com/sickagent/n0/services/connection-manager/internal/adapter"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// Registry holds built-in and external adapters.
type Registry struct {
	mu       sync.RWMutex
	builtins map[string]adapter.Adapter
	external map[string]pb.DatabaseAdapterClient
	conns    map[string]*grpc.ClientConn
	log      *zap.Logger
}

// NewRegistry creates a new adapter registry.
func NewRegistry(log *zap.Logger) *Registry {
	r := &Registry{
		builtins: make(map[string]adapter.Adapter),
		external: make(map[string]pb.DatabaseAdapterClient),
		conns:    make(map[string]*grpc.ClientConn),
		log:      log,
	}
	r.registerBuiltins()
	return r
}

// RouteExternal atomically installs or removes a healthy external adapter route.
func (r *Registry) RouteExternal(adapterType, endpoint, status string) error {
	if status != "active" {
		r.RemoveExternal(adapterType)
		return nil
	}
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial external adapter: %w", err)
	}
	r.mu.Lock()
	old := r.conns[adapterType]
	r.conns[adapterType] = conn
	r.external[adapterType] = pb.NewDatabaseAdapterClient(conn)
	r.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	r.log.Info("routed external adapter", zap.String("type", adapterType), zap.String("endpoint", endpoint))
	return nil
}

// RemoveExternal immediately removes a degraded or disabled route.
func (r *Registry) RemoveExternal(adapterType string) {
	r.mu.Lock()
	conn := r.conns[adapterType]
	delete(r.external, adapterType)
	delete(r.conns, adapterType)
	r.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// Close releases external gRPC connections.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, conn := range r.conns {
		_ = conn.Close()
		delete(r.conns, key)
		delete(r.external, key)
	}
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

// IsExternal reports whether routing should use the external plugin parameter contract.
func (r *Registry) IsExternal(adapterType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.external[adapterType]
	return ok
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
	params sync.Map
}

func (a *externalAdapter) TestConnection(ctx context.Context, dsn string) error {
	params, err := paramsFromJSON(dsn)
	if err != nil {
		return err
	}
	_, err = a.client.TestConnection(ctx, &pb.AdapterTestConnectionRequest{
		Params: params,
	})
	return err
}

func (a *externalAdapter) GetSchema(ctx context.Context, dsn string) ([]adapter.TableInfo, error) {
	params, err := paramsFromJSON(dsn)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.GetSchema(ctx, &pb.AdapterGetSchemaRequest{
		Params: params,
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
	params, err := paramsFromJSON(dsn)
	if err != nil {
		return err
	}
	a.params.Store(connectionID, params)
	return nil
}

func (a *externalAdapter) ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error) {
	params, _ := a.params.Load(connectionID)
	pluginParams, _ := params.(*structpb.Struct)
	resp, err := a.client.ExecuteQuery(ctx, &pb.AdapterExecuteQueryRequest{
		Params:  pluginParams,
		Query:   sql,
		Options: &structpb.Struct{Fields: map[string]*structpb.Value{"limit": structpb.NewNumberValue(float64(limit))}},
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
	a.params.Delete(connectionID)
	return nil
}

func paramsFromJSON(value string) (*structpb.Struct, error) {
	var params map[string]any
	if err := json.Unmarshal([]byte(value), &params); err != nil {
		return nil, fmt.Errorf("decode plugin params: %w", err)
	}
	return structpb.NewStruct(params)
}
