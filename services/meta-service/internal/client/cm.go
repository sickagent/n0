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

// CMClient wraps the ConnectionManager gRPC client.
type CMClient struct {
	conn   *grpc.ClientConn
	client pb.ConnectionManagerClient
}

// NewCMClient creates a new ConnectionManager client.
func NewCMClient(ctx context.Context, nc *natsclient.Client, addr string) (*CMClient, error) {
	target, err := discovery.ResolveGRPCAddr(ctx, nc, "connection-manager", addr)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial connection-manager at %s: %w", target, err)
	}
	return &CMClient{
		conn:   conn,
		client: pb.NewConnectionManagerClient(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *CMClient) Close() error {
	return c.conn.Close()
}

// GetSchema proxies GetSchema.
func (c *CMClient) GetSchema(ctx context.Context, req *pb.GetConnectionSchemaRequest) (*pb.GetConnectionSchemaResponse, error) {
	return c.client.GetSchema(ctx, req)
}
