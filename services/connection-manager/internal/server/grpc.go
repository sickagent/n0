package server

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/connection-manager/internal/dsn"
	"n0/services/connection-manager/internal/registry"
)

// GRPCServer implements lensagent.v1.ConnectionManager.
type GRPCServer struct {
	pb.UnimplementedConnectionManagerServer
	log      *zap.Logger
	registry *registry.Registry
}

// NewGRPCServer creates a new ConnectionManager gRPC server.
func NewGRPCServer(log *zap.Logger, reg *registry.Registry) *GRPCServer {
	return &GRPCServer{log: log, registry: reg}
}

// TestConnection validates connectivity.
func (s *GRPCServer) TestConnection(ctx context.Context, req *pb.TestConnectionRequest) (*pb.TestConnectionResponse, error) {
	a, err := s.registry.Get(req.AdapterType)
	if err != nil {
		return &pb.TestConnectionResponse{Ok: false, ErrorMessage: err.Error(), LatencyMs: 0}, nil
	}
	dsnStr, err := dsn.BuildDSN(req.AdapterType, req.Params)
	if err != nil {
		return &pb.TestConnectionResponse{Ok: false, ErrorMessage: err.Error(), LatencyMs: 0}, nil
	}
	start := time.Now()
	err = a.TestConnection(ctx, dsnStr)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &pb.TestConnectionResponse{Ok: false, ErrorMessage: err.Error(), LatencyMs: latency}, nil
	}
	return &pb.TestConnectionResponse{Ok: true, LatencyMs: latency}, nil
}

// GetPoolHealth returns pool health.
func (s *GRPCServer) GetPoolHealth(ctx context.Context, req *pb.GetPoolHealthRequest) (*pb.GetPoolHealthResponse, error) {
	return &pb.GetPoolHealthResponse{Healthy: true, ActiveConnections: 0, IdleConnections: 0}, nil
}

// ExecuteQuery runs a query.
func (s *GRPCServer) ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error) {
	a, err := s.registry.Get(req.AdapterType)
	if err != nil {
		return nil, err
	}
	dsnStr, err := dsn.BuildDSN(req.AdapterType, req.Params)
	if err != nil {
		return nil, fmt.Errorf("build dsn: %w", err)
	}

	if err := a.Prepare(req.ConnectionId, dsnStr); err != nil {
		return nil, fmt.Errorf("prepare adapter: %w", err)
	}

	rows, err := a.ExecuteQuery(ctx, req.ConnectionId, req.Sql, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	var pbRows []*pb.Row
	var columns []string
	for _, r := range rows {
		if len(columns) == 0 {
			for k := range r {
				columns = append(columns, k)
			}
		}
		var vals []*structpb.Value
		for _, c := range columns {
			vals = append(vals, structpb.NewStringValue(fmt.Sprintf("%v", r[c])))
		}
		pbRows = append(pbRows, &pb.Row{Values: vals})
	}

	return &pb.ExecuteQueryResponse{
		Columns:   columns,
		Rows:      pbRows,
		RowCount:  int64(len(pbRows)),
		Truncated: false,
	}, nil
}

// GetSchema introspects the database schema.
func (s *GRPCServer) GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error) {
	a, err := s.registry.Get(req.AdapterType)
	if err != nil {
		return nil, err
	}
	dsnStr, err := dsn.BuildDSN(req.AdapterType, req.Params)
	if err != nil {
		return nil, err
	}
	tables, err := a.GetSchema(ctx, dsnStr)
	if err != nil {
		return nil, fmt.Errorf("get schema: %w", err)
	}

	var pbTables []*pb.Table
	for _, t := range tables {
		var cols []*pb.Column
		for _, c := range t.Columns {
			cols = append(cols, &pb.Column{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
			})
		}
		pbTables = append(pbTables, &pb.Table{
			Name:    t.Name,
			Columns: cols,
		})
	}

	return &pb.GetConnectionSchemaResponse{Tables: pbTables}, nil
}

// StartGRPC starts the ConnectionManager gRPC server.
func StartGRPC(addr string, log *zap.Logger, reg *registry.Registry) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	s := grpc.NewServer()
	pb.RegisterConnectionManagerServer(s, NewGRPCServer(log, reg))

	go func() {
		log.Info("connection-manager gRPC listening", zap.String("addr", addr))
		if err := s.Serve(lis); err != nil {
			log.Error("grpc serve error", zap.Error(err))
		}
	}()

	return s, nil
}
