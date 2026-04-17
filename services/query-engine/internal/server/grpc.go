package server

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
)

// GRPCServer implements lensagent.v1.QueryEngine.
type GRPCServer struct {
	pb.UnimplementedQueryEngineServer
	log *zap.Logger
}

// NewGRPCServer creates a new QueryEngine gRPC server.
func NewGRPCServer(log *zap.Logger) *GRPCServer {
	return &GRPCServer{log: log}
}

// SubmitQuery enqueues a query job.
func (s *GRPCServer) SubmitQuery(ctx context.Context, req *pb.SubmitQueryRequest) (*pb.SubmitQueryResponse, error) {
	return &pb.SubmitQueryResponse{JobId: "job-123", Status: "pending"}, nil
}

// GetJobStatus returns job status.
func (s *GRPCServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	return &pb.GetJobStatusResponse{JobId: req.JobId, Status: "success"}, nil
}

// GetJobResult returns query results.
func (s *GRPCServer) GetJobResult(ctx context.Context, req *pb.GetJobResultRequest) (*pb.GetJobResultResponse, error) {
	return &pb.GetJobResultResponse{JobId: req.JobId, Rows: nil, NextPageToken: "", Truncated: false}, nil
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
func StartGRPC(addr string, log *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	s := grpc.NewServer()
	pb.RegisterQueryEngineServer(s, NewGRPCServer(log))

	go func() {
		log.Info("query-engine gRPC listening", zap.String("addr", addr))
		if err := s.Serve(lis); err != nil {
			log.Error("grpc serve error", zap.Error(err))
		}
	}()

	return s, nil
}
