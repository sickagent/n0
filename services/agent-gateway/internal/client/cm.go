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

// TestConnection proxies TestConnection.
func (c *ConnectionManagerClient) TestConnection(ctx context.Context, req *pb.TestConnectionRequest) (*pb.TestConnectionResponse, error) {
	return c.client.TestConnection(ctx, req)
}

// GetSchema proxies GetSchema.
func (c *ConnectionManagerClient) GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error) {
	return c.client.GetSchema(ctx, req)
}

// ExecuteQuery proxies ExecuteQuery.
func (c *ConnectionManagerClient) ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error) {
	return c.client.ExecuteQuery(ctx, req)
}
