package server

import (
	"fmt"
	"net"

	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	"github.com/sickagent/n0/services/meta-service/internal/app"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
