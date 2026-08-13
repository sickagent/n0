package server

import (
	"context"
	"strings"

	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	"github.com/sickagent/n0/services/meta-service/internal/app"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCServer implements n0.platform.v1.MetaService.
type GRPCServer struct {
	pb.UnimplementedMetaServiceServer
	svc *app.MetaService
}

// NewGRPCServer creates a new MetaService gRPC server.
func NewGRPCServer(svc *app.MetaService) *GRPCServer {
	return &GRPCServer{svc: svc}
}

// GetSchema returns the schema snapshot for a connection.
func (s *GRPCServer) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	if strings.TrimSpace(req.ConnectionId) == "" || strings.TrimSpace(req.TenantId) == "" {
		return nil, status.Error(codes.InvalidArgument, "connection_id and tenant_id are required")
	}
	snap, err := s.svc.GetSchema(ctx, req.ConnectionId, req.TenantId)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return &pb.GetSchemaResponse{}, nil
	}

	var tables []*pb.Table
	for _, t := range snap.Tables {
		var cols []*pb.Column
		for _, c := range t.Columns {
			cols = append(cols, &pb.Column{
				Name:     c.Name,
				DataType: c.DataType,
				Nullable: c.Nullable,
			})
		}
		tables = append(tables, &pb.Table{
			Name:    t.Name,
			Columns: cols,
		})
	}

	return &pb.GetSchemaResponse{
		Snapshot: &pb.SchemaSnapshot{
			ConnectionId: snap.ConnectionID,
			Tables:       tables,
		},
	}, nil
}

// ListWorkspaces returns workspaces for a tenant.
func (s *GRPCServer) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.ListWorkspacesResponse, error) {
	limit, offset := int32(20), int32(0)
	if req.Pagination != nil {
		limit = req.Pagination.Limit
		offset = req.Pagination.Offset
	}
	workspaces, err := s.svc.ListWorkspaces(ctx, req.TenantId, int(limit), int(offset))
	if err != nil {
		return nil, err
	}

	var items []*pb.Workspace
	for _, w := range workspaces {
		items = append(items, &pb.Workspace{
			Id:        w.ID.String(),
			TenantId:  w.TenantID,
			Name:      w.Name,
			CreatedAt: timestamppb.New(w.CreatedAt),
		})
	}

	meta := &pb.ListMeta{Total: int32(len(items)), Limit: limit, Offset: offset}
	return &pb.ListWorkspacesResponse{Workspaces: items, Meta: meta}, nil
}

// CreateConnection creates a new connection.
func (s *GRPCServer) CreateConnection(ctx context.Context, req *pb.CreateConnectionRequest) (*pb.CreateConnectionResponse, error) {
	id, err := s.svc.CreateConnection(ctx, app.Connection{
		WorkspaceID: req.WorkspaceId,
		UserID:      req.TenantId,
		TenantID:    req.TenantId,
		Name:        req.Name,
		AdapterType: req.AdapterType,
		Params:      req.Params.AsMap(),
	})
	if err != nil {
		return nil, err
	}
	conn, err := s.svc.GetConnection(ctx, id.String(), req.TenantId)
	if err != nil {
		return nil, err
	}
	return &pb.CreateConnectionResponse{Connection: toProtoConnection(conn)}, nil
}

// GetConnection returns a connection by ID.
func (s *GRPCServer) GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error) {
	if strings.TrimSpace(req.ConnectionId) == "" || strings.TrimSpace(req.TenantId) == "" {
		return nil, status.Error(codes.InvalidArgument, "connection_id and tenant_id are required")
	}
	conn, err := s.svc.GetConnection(ctx, req.ConnectionId, req.TenantId)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return &pb.GetConnectionResponse{}, nil
	}
	return &pb.GetConnectionResponse{Connection: toProtoConnection(conn)}, nil
}

// ListConnections returns connections for a tenant/workspace.
func (s *GRPCServer) ListConnections(ctx context.Context, req *pb.ListConnectionsRequest) (*pb.ListConnectionsResponse, error) {
	limit, offset := int32(20), int32(0)
	if req.Pagination != nil {
		limit = req.Pagination.Limit
		offset = req.Pagination.Offset
	}
	conns, err := s.svc.ListConnections(ctx, req.TenantId, req.WorkspaceId, int(limit), int(offset))
	if err != nil {
		return nil, err
	}
	var items []*pb.Connection
	for _, c := range conns {
		items = append(items, toProtoConnection(&c))
	}
	meta := &pb.ListMeta{Total: int32(len(items)), Limit: limit, Offset: offset}
	return &pb.ListConnectionsResponse{Connections: items, Meta: meta}, nil
}

// DeleteConnection removes a connection.
func (s *GRPCServer) DeleteConnection(ctx context.Context, req *pb.DeleteConnectionRequest) (*pb.DeleteConnectionResponse, error) {
	if strings.TrimSpace(req.ConnectionId) == "" || strings.TrimSpace(req.TenantId) == "" {
		return nil, status.Error(codes.InvalidArgument, "connection_id and tenant_id are required")
	}
	if err := s.svc.DeleteConnection(ctx, req.ConnectionId, req.TenantId); err != nil {
		return nil, err
	}
	return &pb.DeleteConnectionResponse{Deleted: true}, nil
}

// RegisterPlugin registers a new plugin.
func (s *GRPCServer) RegisterPlugin(ctx context.Context, req *pb.RegisterPluginRequest) (*pb.RegisterPluginResponse, error) {
	id, err := s.svc.RegisterPlugin(ctx, app.PluginDefinition{
		PluginType: req.PluginType,
		Name:       req.Name,
		Version:    req.Version,
		Endpoint:   req.Endpoint,
		Protocol:   req.Protocol,
		TenantID:   req.TenantId,
		IsGlobal:   req.Global,
	})
	if err != nil {
		return nil, err
	}
	return &pb.RegisterPluginResponse{
		PluginId: id.String(),
		Status:   "validated",
	}, nil
}

func toProtoConnection(c *app.Connection) *pb.Connection {
	if c == nil {
		return nil
	}
	params, _ := structpb.NewStruct(c.Params)
	return &pb.Connection{
		Id:          c.ID.String(),
		WorkspaceId: c.WorkspaceID,
		TenantId:    c.TenantID,
		Name:        c.Name,
		AdapterType: c.AdapterType,
		Params:      params,
		CreatedAt:   timestamppb.New(c.CreatedAt),
	}
}
