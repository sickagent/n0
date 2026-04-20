package main

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"n0/pkg/shared/config"
	"n0/pkg/shared/discovery"
	"n0/pkg/shared/graceful"
	"n0/pkg/shared/logger"
	"n0/pkg/shared/natsclient"
	"n0/pkg/shared/observability"
	"n0/services/connection-manager/internal/registry"
	"n0/services/connection-manager/internal/server"
)

type Config struct {
	config.BaseConfig
	GRPCAddr          string `mapstructure:"grpc_addr"`
	GRPCAdvertiseAddr string `mapstructure:"grpc_advertise_addr"`
	HTTPAddr          string `mapstructure:"http_addr"`
	VaultAddr         string `mapstructure:"vault_addr"`
}

func main() {
	var cfg Config
	cmd := &cobra.Command{
		Use:   "connection-manager",
		Short: "n0 Connection Manager",
		Run: func(_ *cobra.Command, _ []string) {
			cobra.CheckErr(config.Load(&cfg))
			log := logger.New(cfg.LogLevel)
			log.Info("starting connection-manager",
				zap.String("grpc", cfg.GRPCAddr),
				zap.String("http", cfg.HTTPAddr),
			)

			ctx, cancel := graceful.ContextWithShutdown(30 * time.Second)
			defer cancel()

			nc, err := natsclient.New(cfg.NATSURL, 5*time.Second, log)
			if err != nil {
				log.Fatal("nats connect failed", zap.Error(err))
			}
			defer nc.Close()

			sub, err := nc.Conn.Subscribe("events.plugin.registered", func(msg *nats.Msg) {
				log.Info("plugin registered", zap.String("data", string(msg.Data)))
			})
			if err != nil {
				log.Fatal("subscribe failed", zap.Error(err))
			}
			defer sub.Unsubscribe()

			reg := registry.NewRegistry(log)
			grpcSrv, err := server.StartGRPC(cfg.GRPCAddr, log, reg)
			if err != nil {
				log.Fatal("grpc start failed", zap.Error(err))
			}
			defer grpcSrv.GracefulStop()

			discoverySub, err := discovery.RegisterGRPCResponder(nc, "connection-manager", cfg.GRPCAddr, cfg.GRPCAdvertiseAddr, log)
			if err != nil {
				log.Fatal("grpc discovery register failed", zap.Error(err))
			}
			defer discoverySub.Unsubscribe()

			httpSrv := server.NewHTTPServer(cfg.HTTPAddr, log, reg)
			go func() {
				if err := httpSrv.Start(ctx); err != nil {
					log.Error("http server error", zap.Error(err))
				}
			}()

			metrics := observability.StartMetricsServer(":9090", log)
			defer func() { _ = metrics.Close() }()

			<-ctx.Done()
			log.Info("shutting down connection-manager")
		},
	}

	cmd.Flags().String("app_name", "connection-manager", "application name")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("grpc_advertise_addr", "", "advertised gRPC address for discovery")
	cmd.Flags().String("http_addr", ":8082", "HTTP listen address")
	cmd.Flags().String("vault_addr", "http://localhost:8200", "Vault address")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}
