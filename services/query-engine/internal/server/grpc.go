package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/nats-io/nuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/query-engine/internal/job"
	"n0/services/query-engine/internal/worker"
)

const querySubject = "QUERIES.jobs"

type publisher interface {
	Publish(subject string, data []byte) error
}

// GRPCServer implements lensagent.v1.QueryEngine.
type GRPCServer struct {
	pb.UnimplementedQueryEngineServer
	log       *zap.Logger
	store     *job.Store
	publisher publisher
}

// NewGRPCServer creates a new QueryEngine gRPC server.
func NewGRPCServer(log *zap.Logger, store *job.Store, publisher publisher) *GRPCServer {
	if store == nil {
		store = job.NewStore()
	}
	return &GRPCServer{
		log:       log,
		store:     store,
		publisher: publisher,
	}
}

// SubmitQuery enqueues a query job.
func (s *GRPCServer) SubmitQuery(ctx context.Context, req *pb.SubmitQueryRequest) (*pb.SubmitQueryResponse, error) {
	if strings.TrimSpace(req.ConnectionId) == "" {
		return nil, status.Error(codes.InvalidArgument, "connection_id is required")
	}
	if strings.TrimSpace(req.Sql) == "" {
		return nil, status.Error(codes.InvalidArgument, "sql is required")
	}
	if s.publisher == nil {
		return nil, status.Error(codes.FailedPrecondition, "query publisher is not configured")
	}

	jobID := "job-" + strings.ToLower(nuid.Next())
	record := s.store.Create(job.Record{
		ID:           jobID,
		TenantID:     req.TenantId,
		ConnectionID: req.ConnectionId,
		SQL:          req.Sql,
	})

	payload, err := json.Marshal(worker.Job{
		ID:           record.ID,
		TenantID:     record.TenantID,
		ConnectionID: record.ConnectionID,
		SQL:          record.SQL,
	})
	if err != nil {
		_ = s.store.MarkFailed(record.ID, "marshal job payload: "+err.Error())
		return nil, status.Errorf(codes.Internal, "marshal job payload: %v", err)
	}

	if err := s.publisher.Publish(querySubject, payload); err != nil {
		_ = s.store.MarkFailed(record.ID, "publish job: "+err.Error())
		return nil, status.Errorf(codes.Unavailable, "publish job: %v", err)
	}

	return &pb.SubmitQueryResponse{JobId: record.ID, Status: record.Status}, nil
}

// GetJobStatus returns job status.
func (s *GRPCServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	record, err := s.store.Get(req.JobId)
	if err != nil {
		if err == job.ErrJobNotFound {
			return nil, status.Error(codes.NotFound, "job not found")
		}
		return nil, status.Errorf(codes.Internal, "get job status: %v", err)
	}
	return &pb.GetJobStatusResponse{
		JobId:        record.ID,
		Status:       record.Status,
		ErrorMessage: record.ErrorMessage,
	}, nil
}

// GetJobResult returns query results.
func (s *GRPCServer) GetJobResult(ctx context.Context, req *pb.GetJobResultRequest) (*pb.GetJobResultResponse, error) {
	record, rows, nextToken, err := s.store.GetResultPage(req.JobId, req.Page, req.PageSize)
	if err != nil {
		if err == job.ErrJobNotFound {
			return nil, status.Error(codes.NotFound, "job not found")
		}
		return nil, status.Errorf(codes.Internal, "get job result: %v", err)
	}
	if record.Status != job.StatusSuccess {
		return nil, status.Errorf(codes.FailedPrecondition, "job status is %s", record.Status)
	}

	pbRows := make([]*structpb.Struct, 0, len(rows))
	for _, row := range rows {
		pbRow, err := structpb.NewStruct(row)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal job row: %v", err)
		}
		pbRows = append(pbRows, pbRow)
	}

	return &pb.GetJobResultResponse{
		JobId:         record.ID,
		Rows:          pbRows,
		NextPageToken: nextToken,
		Truncated:     record.Truncated,
	}, nil
}

// SuggestChartConfig suggests a chart config.
func (s *GRPCServer) SuggestChartConfig(ctx context.Context, req *pb.SuggestChartConfigRequest) (*pb.SuggestChartConfigResponse, error) {
	cfg, _ := structpb.NewStruct(map[string]any{
		"x": "date",
		"y": "revenue",
	})
	return &pb.SuggestChartConfigResponse{ChartType: "line", Config: cfg}, nil
}

// StartGRPC starts the QueryEngine gRPC server.
func StartGRPC(addr string, svc *GRPCServer, log *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	s := grpc.NewServer()
	pb.RegisterQueryEngineServer(s, svc)

	go func() {
		log.Info("query-engine gRPC listening", zap.String("addr", addr))
		if err := s.Serve(lis); err != nil {
			log.Error("grpc serve error", zap.Error(err))
		}
	}()

	return s, nil
}
