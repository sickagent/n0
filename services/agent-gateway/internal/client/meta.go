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

// GetSchema proxies GetSchema.
func (c *MetaClient) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	return c.client.GetSchema(ctx, req)
}

// ListWorkspaces proxies ListWorkspaces.
func (c *MetaClient) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.ListWorkspacesResponse, error) {
	return c.client.ListWorkspaces(ctx, req)
}

// CreateConnection proxies CreateConnection.
func (c *MetaClient) CreateConnection(ctx context.Context, req *pb.CreateConnectionRequest) (*pb.CreateConnectionResponse, error) {
	return c.client.CreateConnection(ctx, req)
}

// GetConnection proxies GetConnection.
func (c *MetaClient) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error) {
	return c.client.GetConnection(ctx, req)
}

// ListConnections proxies ListConnections.
func (c *MetaClient) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	return c.client.ListConnections(ctx, req)
}

// DeleteConnection proxies DeleteConnection.
func (c *MetaClient) DeleteConnection(ctx context.Context, req *pb.DeleteConnectionRequest) (*pb.DeleteConnectionResponse, error) {
	return c.client.DeleteConnection(ctx, req)
}

// RegisterPlugin proxies RegisterPlugin.
func (c *MetaClient) RegisterPlugin(ctx context.Context, req *pb.RegisterPluginRequest) (*pb.RegisterPluginResponse, error) {
	return c.client.RegisterPlugin(ctx, req)
}
