package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const serviceName = "n0.platform.v1.DatabaseAdapter"

type demoAdapter struct {
	pb.UnimplementedDatabaseAdapterServer
}

func (demoAdapter) GetAdapterInfo(context.Context, *pb.GetAdapterInfoRequest) (*pb.GetAdapterInfoResponse, error) {
	return &pb.GetAdapterInfoResponse{
		Name:              "example",
		Version:           "0.1.0",
		Author:            "n0 contributors",
		SupportedFeatures: []string{"test_connection", "schema", "select_literal"},
	}, nil
}

func (demoAdapter) TestConnection(_ context.Context, req *pb.AdapterTestConnectionRequest) (*pb.AdapterTestConnectionResponse, error) {
	if req.GetParams().GetFields()["api_key"].GetStringValue() == "" {
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}
	return &pb.AdapterTestConnectionResponse{Ok: true}, nil
}

func (demoAdapter) GetSchema(context.Context, *pb.AdapterGetSchemaRequest) (*pb.AdapterGetSchemaResponse, error) {
	return &pb.AdapterGetSchemaResponse{Tables: []*pb.Table{{
		Name: "demo_metrics",
		Columns: []*pb.Column{
			{Name: "metric", DataType: "text", Nullable: false},
			{Name: "value", DataType: "double", Nullable: false},
		},
	}}}, nil
}

func (demoAdapter) ExecuteQuery(_ context.Context, req *pb.AdapterExecuteQueryRequest) (*pb.AdapterExecuteQueryResponse, error) {
	query := strings.TrimSpace(strings.TrimSuffix(req.GetQuery(), ";"))
	if !strings.EqualFold(query, "SELECT * FROM demo_metrics") {
		return nil, status.Error(codes.InvalidArgument, "example plugin only supports SELECT * FROM demo_metrics")
	}

	rows := []*pb.Row{
		{Values: []*structpb.Value{structpb.NewStringValue("requests"), structpb.NewNumberValue(42)}},
		{Values: []*structpb.Value{structpb.NewStringValue("latency_ms"), structpb.NewNumberValue(12.5)}},
	}
	return &pb.AdapterExecuteQueryResponse{
		Columns:  []string{"metric", "value"},
		Rows:     rows,
		RowCount: int64(len(rows)),
	}, nil
}

func (demoAdapter) GetDialectCapabilities(context.Context, *pb.GetDialectCapabilitiesRequest) (*pb.GetDialectCapabilitiesResponse, error) {
	return &pb.GetDialectCapabilitiesResponse{
		Capabilities: []*pb.DialectCapability{{
			FunctionName: "select",
			Supported:    true,
			Notes:        "The example accepts only SELECT * FROM demo_metrics",
		}},
		SupportedTypes: []string{"text", "double"},
	}, nil
}

func run(ctx context.Context, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	server := grpc.NewServer()
	pb.RegisterDatabaseAdapterServer(server, demoAdapter{})
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)

	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	log.Printf("example n0 plugin listening on %s", listener.Addr())

	select {
	case <-ctx.Done():
		server.GracefulStop()
		return nil
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

func main() {
	address := flag.String("addr", ":50051", "gRPC listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *address); err != nil {
		log.Fatal(err)
	}
}
