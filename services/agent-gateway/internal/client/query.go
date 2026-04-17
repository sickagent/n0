package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "n0/proto/gen/go/lensagent/v1"
)

// QueryEngineClient wraps the QueryEngine gRPC client.
type QueryEngineClient struct {
	conn   *grpc.ClientConn
	client pb.QueryEngineClient
}

// NewQueryEngineClient creates a new QueryEngine client.
func NewQueryEngineClient(addr string) (*QueryEngineClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial query-engine: %w", err)
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
