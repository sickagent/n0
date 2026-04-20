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

// MetaClient wraps the MetaService gRPC client.
type MetaClient struct {
	conn   *grpc.ClientConn
	client pb.MetaServiceClient
}

// NewMetaClient creates a new MetaService client.
func NewMetaClient(ctx context.Context, nc *natsclient.Client, addr string) (*MetaClient, error) {
	target, err := discovery.ResolveGRPCAddr(ctx, nc, "meta-service", addr)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial meta-service at %s: %w", target, err)
	}
	return &MetaClient{
		conn:   conn,
		client: pb.NewMetaServiceClient(conn),
	}, nil
}

// Close closes the underlying connection.
func (c *MetaClient) Close() error {
	return c.conn.Close()
}

// GetConnection proxies GetConnection.
func (c *MetaClient) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error) {
	return c.client.GetConnection(ctx, req)
}
