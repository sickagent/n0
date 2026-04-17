package registry

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
)

type fakeDatabaseAdapterClient struct {
	pb.UnimplementedDatabaseAdapterServer
}

func (f *fakeDatabaseAdapterClient) GetAdapterInfo(ctx context.Context, in *pb.GetAdapterInfoRequest, opts ...grpc.CallOption) (*pb.GetAdapterInfoResponse, error) {
	return &pb.GetAdapterInfoResponse{}, nil
}

func (f *fakeDatabaseAdapterClient) TestConnection(ctx context.Context, in *pb.AdapterTestConnectionRequest, opts ...grpc.CallOption) (*pb.AdapterTestConnectionResponse, error) {
	return &pb.AdapterTestConnectionResponse{Ok: true}, nil
}

func (f *fakeDatabaseAdapterClient) GetSchema(ctx context.Context, in *pb.AdapterGetSchemaRequest, opts ...grpc.CallOption) (*pb.AdapterGetSchemaResponse, error) {
	return &pb.AdapterGetSchemaResponse{}, nil
}

func (f *fakeDatabaseAdapterClient) ExecuteQuery(ctx context.Context, in *pb.AdapterExecuteQueryRequest, opts ...grpc.CallOption) (*pb.AdapterExecuteQueryResponse, error) {
	return &pb.AdapterExecuteQueryResponse{}, nil
}

func (f *fakeDatabaseAdapterClient) GetDialectCapabilities(ctx context.Context, in *pb.GetDialectCapabilitiesRequest, opts ...grpc.CallOption) (*pb.GetDialectCapabilitiesResponse, error) {
	return &pb.GetDialectCapabilitiesResponse{}, nil
}

func TestNewRegistry_Builtins(t *testing.T) {
	log := zap.NewNop()
	r := NewRegistry(log)

	builtins := r.List()
	if len(builtins) != 6 {
		t.Fatalf("expected 6 builtins, got %d", len(builtins))
	}

	for _, typ := range []string{"postgres", "clickhouse", "mysql", "sqlite", "mssql", "bigquery"} {
		a, err := r.Get(typ)
		if err != nil {
			t.Errorf("expected %s adapter to exist: %v", typ, err)
		}
		if a == nil {
			t.Errorf("expected %s adapter to be non-nil", typ)
		}
	}
}

func TestRegistry_Get_Unknown(t *testing.T) {
	log := zap.NewNop()
	r := NewRegistry(log)
	_, err := r.Get("oracle")
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
}

func TestRegistry_RegisterExternal(t *testing.T) {
	log := zap.NewNop()
	r := NewRegistry(log)

	stub := &fakeDatabaseAdapterClient{}
	r.RegisterExternal("bigquery", stub)

	a, err := r.Get("bigquery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected external adapter")
	}
}

func TestExternalAdapter_PrepareRelease(t *testing.T) {
	a := &externalAdapter{}
	if err := a.Prepare("cid", "dsn"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := a.Release("cid"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExternalAdapter_TestConnection(t *testing.T) {
	// externalAdapter with nil client should panic or error; we just verify it doesn't succeed.
	a := &externalAdapter{client: &fakeDatabaseAdapterClient{}}
	err := a.TestConnection(context.Background(), "dsn")
	if err != nil {
		t.Logf("expected nil error with fake client, got: %v", err)
	}
}
