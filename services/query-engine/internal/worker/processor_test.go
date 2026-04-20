package worker

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/query-engine/internal/job"
)

type fakeExecutor struct {
	req *pb.ExecuteQueryRequest
}

func (f *fakeExecutor) ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error) {
	f.req = req
	return &pb.ExecuteQueryResponse{
		Columns: []string{"value"},
		Rows: []*pb.Row{
			{Values: []*structpb.Value{structpb.NewNumberValue(1)}},
		},
		RowCount: 1,
	}, nil
}

type fakeLookup struct{}

func (f *fakeLookup) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error) {
	params, err := structpb.NewStruct(map[string]any{"host": "postgres"})
	if err != nil {
		return nil, err
	}
	return &pb.GetConnectionResponse{
		Connection: &pb.Connection{
			Id:          req.ConnectionId,
			AdapterType: "postgres",
			Params:      params,
		},
	}, nil
}

func TestQueryProcessorUsesSanitizedSQLWithoutSecondaryLimit(t *testing.T) {
	store := job.NewStore()
	store.Create(job.Record{
		ID:           "job-1",
		ConnectionID: "conn-1",
		TenantID:     "user-1",
		SQL:          "SELECT 1",
	})

	exec := &fakeExecutor{}
	proc := NewQueryProcessor(zap.NewNop(), exec, &fakeLookup{}, store)

	if err := proc.Process(context.Background(), Job{
		ID:           "job-1",
		ConnectionID: "conn-1",
		TenantID:     "user-1",
		SQL:          "SELECT 1",
	}); err != nil {
		t.Fatalf("process job: %v", err)
	}

	if exec.req == nil {
		t.Fatal("expected execute query request")
	}
	if exec.req.Limit != 0 {
		t.Fatalf("expected limit 0, got %d", exec.req.Limit)
	}
	if exec.req.Sql != "SELECT 1 LIMIT 10000" {
		t.Fatalf("expected sanitized SQL with single limit, got %q", exec.req.Sql)
	}

	result, err := store.Get("job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if result.Status != job.StatusSuccess {
		t.Fatalf("expected success, got %s", result.Status)
	}
}
