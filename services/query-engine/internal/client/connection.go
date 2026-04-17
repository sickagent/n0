package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "n0/proto/gen/go/lensagent/v1"
)

// ConnectionManagerClient wraps the ConnectionManager gRPC client.
type ConnectionManagerClient struct {
	conn   *grpc.ClientConn
	client pb.ConnectionManagerClient
}

// NewConnectionManagerClient creates a new ConnectionManager client.
func NewConnectionManagerClient(addr string) (*ConnectionManagerClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial connection-manager: %w", err)
	}
	return &ConnectionManagerClient{
		conn:   conn,
		client: pb.NewConnectionManagerClient(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *ConnectionManagerClient) Close() error {
	return c.conn.Close()
}

// ExecuteQuery proxies ExecuteQuery.
func (c *ConnectionManagerClient) ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error) {
	return c.client.ExecuteQuery(ctx, req)
}
