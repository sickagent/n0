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

// ConnectionManagerClient wraps the ConnectionManager gRPC client.
type ConnectionManagerClient struct {
	conn   *grpc.ClientConn
	client pb.ConnectionManagerClient
}

// NewConnectionManagerClient creates a new ConnectionManager client.
func NewConnectionManagerClient(ctx context.Context, nc *natsclient.Client, addr string) (*ConnectionManagerClient, error) {
	target, err := discovery.ResolveGRPCAddr(ctx, nc, "connection-manager", addr)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial connection-manager at %s: %w", target, err)
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
