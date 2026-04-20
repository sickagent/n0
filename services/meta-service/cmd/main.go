package main

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"n0/pkg/shared/config"
	"n0/pkg/shared/crypto"
	"n0/pkg/shared/discovery"
	"n0/pkg/shared/graceful"
	"n0/pkg/shared/logger"
	"n0/pkg/shared/natsclient"
	"n0/pkg/shared/observability"
	"n0/services/meta-service/internal/app"
	"n0/services/meta-service/internal/client"
	"n0/services/meta-service/internal/repository"
	"n0/services/meta-service/internal/server"
)

type Config struct {
	config.BaseConfig
	HTTPAddr              string `mapstructure:"http_addr"`
	GRPCAddr              string `mapstructure:"grpc_addr"`
	GRPCAdvertiseAddr     string `mapstructure:"grpc_advertise_addr"`
	PostgresDSN           string `mapstructure:"postgres_dsn"`
	ConnectionManagerAddr string `mapstructure:"connection_manager_addr"`
	EncryptionKey         string `mapstructure:"encryption_key"`
}

func main() {
	var cfg Config
	cmd := &cobra.Command{
		Use:   "meta-service",
		Short: "n0 Meta Service",
		Run: func(_ *cobra.Command, _ []string) {
			cobra.CheckErr(config.Load(&cfg))
			log := logger.New(cfg.LogLevel)
			log.Info("starting meta-service",
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

			_, err = nc.EnsureStream(ctx, streamConfig("AUDIT"))
			if err != nil {
				log.Fatal("stream ensure failed", zap.Error(err))
			}
			_, err = nc.EnsureStream(ctx, streamConfig("QUERIES"))
			if err != nil {
				log.Fatal("stream ensure failed", zap.Error(err))
			}

			repo, err := repository.NewPostgresRepositoryFromDSN(cfg.PostgresDSN)
			if err != nil {
				log.Fatal("repository init failed", zap.Error(err))
			}
			defer repo.Close()

			cmCli, err := client.NewCMClient(ctx, nc, cfg.ConnectionManagerAddr)
			if err != nil {
				log.Fatal("cm client init failed", zap.Error(err))
			}
			defer cmCli.Close()

			var encrypter *crypto.Encrypter
			if cfg.EncryptionKey != "" {
				var err error
				encrypter, err = crypto.NewEncrypterFromBase64(cfg.EncryptionKey)
				if err != nil {
					log.Fatal("invalid encryption key", zap.Error(err))
				}
			}
			metaSvc := app.NewMetaService(repo, cmCli, encrypter)

			grpcSrv, err := server.StartGRPC(cfg.GRPCAddr, metaSvc, log)
			if err != nil {
				log.Fatal("grpc start failed", zap.Error(err))
			}
			defer grpcSrv.GracefulStop()

			discoverySub, err := discovery.RegisterGRPCResponder(nc, "meta-service", cfg.GRPCAddr, cfg.GRPCAdvertiseAddr, log)
			if err != nil {
				log.Fatal("grpc discovery register failed", zap.Error(err))
			}
			defer discoverySub.Unsubscribe()

			grpcHandler := server.NewGRPCServer(metaSvc)
			httpSrv := server.NewHTTPServer(cfg.HTTPAddr, log, grpcHandler, metaSvc)
			go func() {
				if err := httpSrv.Start(ctx); err != nil {
					log.Error("http server error", zap.Error(err))
				}
			}()

			metrics := observability.StartMetricsServer(":9090", log)
			defer func() { _ = metrics.Close() }()
			<-ctx.Done()
			log.Info("shutting down meta-service")
		},
	}

	cmd.Flags().String("app_name", "meta-service", "application name")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("grpc_advertise_addr", "", "advertised gRPC address for discovery")
	cmd.Flags().String("http_addr", ":8081", "HTTP listen address")
	cmd.Flags().String("postgres_dsn", "postgres://postgres:postgres@localhost:5432/meta?sslmode=disable", "Postgres DSN")
	cmd.Flags().String("connection_manager_addr", "localhost:8081", "Connection Manager gRPC address")
	cmd.Flags().String("encryption_key", "", "Base64-encoded 32-byte AES-256 encryption key")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}

func streamConfig(name string) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:     name,
		Subjects: []string{name + ".*"},
		Replicas: 1,
	}
}
