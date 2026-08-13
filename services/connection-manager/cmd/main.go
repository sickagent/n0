package main

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sickagent/n0/pkg/shared/config"
	"github.com/sickagent/n0/pkg/shared/discovery"
	"github.com/sickagent/n0/pkg/shared/graceful"
	"github.com/sickagent/n0/pkg/shared/logger"
	"github.com/sickagent/n0/pkg/shared/natsclient"
	"github.com/sickagent/n0/pkg/shared/observability"
	"github.com/sickagent/n0/services/connection-manager/internal/registry"
	"github.com/sickagent/n0/services/connection-manager/internal/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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

			reg := registry.NewRegistry(log)
			defer reg.Close()
			type pluginRoute struct {
				AdapterType string `json:"adapter_type"`
				Endpoint    string `json:"endpoint"`
				Status      string `json:"status"`
			}
			sub, err := nc.Conn.Subscribe("events.plugin.route", func(msg *nats.Msg) {
				var route pluginRoute
				if err := json.Unmarshal(msg.Data, &route); err != nil {
					log.Error("invalid plugin route", zap.Error(err))
					return
				}
				if err := reg.RouteExternal(route.AdapterType, route.Endpoint, route.Status); err != nil {
					log.Error("route plugin failed", zap.Error(err))
				}
			})
			if err != nil {
				log.Fatal("subscribe failed", zap.Error(err))
			}
			defer sub.Unsubscribe()

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
			defer observability.Shutdown(metrics, log)

			<-ctx.Done()
			log.Info("shutting down connection-manager")
		},
	}

	cmd.Flags().String("app_name", "connection-manager", "application name")
	cmd.Flags().String("environment", "development", "runtime environment")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("grpc_advertise_addr", "", "advertised gRPC address for discovery")
	cmd.Flags().String("http_addr", ":8082", "HTTP listen address")
	cmd.Flags().String("vault_addr", "http://localhost:8200", "Vault address")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}
