package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"n0/pkg/shared/discovery"
	"n0/pkg/shared/natsclient"
	pb "n0/proto/gen/go/lensagent/v1"
)

// QueryEngineClient wraps the QueryEngine gRPC client.
type QueryEngineClient struct {
	conn   *grpc.ClientConn
	client pb.QueryEngineClient
}

// NewQueryEngineClient creates a new QueryEngine client.
func NewQueryEngineClient(ctx context.Context, nc *natsclient.Client, addr string) (*QueryEngineClient, error) {
	target, err := discovery.ResolveGRPCAddr(ctx, nc, "query-engine", addr)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial query-engine at %s: %w", target, err)
	}
	return &QueryEngineClient{
		conn:   conn,
		client: pb.NewQueryEngineClient(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *QueryEngineClient) Close() error {
	return c.conn.Close()
}

// SubmitQuery proxies SubmitQuery.
func (c *QueryEngineClient) SubmitQuery(ctx context.Context, req *pb.SubmitQueryRequest) (*pb.SubmitQueryResponse, error) {
	return c.client.SubmitQuery(ctx, req)
}

// GetJobStatus proxies GetJobStatus.
func (c *QueryEngineClient) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	return c.client.GetJobStatus(ctx, req)
}

// GetJobResult proxies GetJobResult.
func (c *QueryEngineClient) GetJobResult(ctx context.Context, req *pb.GetJobResultRequest) (*pb.GetJobResultResponse, error) {
	return c.client.GetJobResult(ctx, req)
}
