package server

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/meta-service/internal/app"
)

// StartGRPC starts the MetaService gRPC server.
func StartGRPC(addr string, svc *app.MetaService, log *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	s := grpc.NewServer()
	pb.RegisterMetaServiceServer(s, NewGRPCServer(svc))

	go func() {
		log.Info("meta-service gRPC listening", zap.String("addr", addr))
		if err := s.Serve(lis); err != nil {
			log.Error("grpc serve error", zap.Error(err))
		}
	}()

	return s, nil
}
