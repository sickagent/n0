package main

import (
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/sickagent/n0/pkg/shared/config"
	"github.com/sickagent/n0/pkg/shared/crypto"
	"github.com/sickagent/n0/pkg/shared/discovery"
	"github.com/sickagent/n0/pkg/shared/graceful"
	"github.com/sickagent/n0/pkg/shared/logger"
	"github.com/sickagent/n0/pkg/shared/natsclient"
	"github.com/sickagent/n0/pkg/shared/observability"
	"github.com/sickagent/n0/services/meta-service/internal/app"
	"github.com/sickagent/n0/services/meta-service/internal/auditsink"
	"github.com/sickagent/n0/services/meta-service/internal/client"
	"github.com/sickagent/n0/services/meta-service/internal/pluginlifecycle"
	"github.com/sickagent/n0/services/meta-service/internal/repository"
	"github.com/sickagent/n0/services/meta-service/internal/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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

			auditStream, err := nc.EnsureStream(ctx, jetstream.StreamConfig{
				Name: "AUDIT", Subjects: []string{"audit.events.>"}, Replicas: 1,
				Retention: jetstream.LimitsPolicy, MaxAge: 30 * 24 * time.Hour,
			})
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

			auditConsumer, err := auditStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
				Durable: "postgres-audit-sink", Name: "postgres-audit-sink",
				AckPolicy: jetstream.AckExplicitPolicy, AckWait: 30 * time.Second, MaxDeliver: 10,
			})
			if err != nil {
				log.Fatal("audit consumer create failed", zap.Error(err))
			}
			sink := auditsink.New(auditConsumer, repo, log)
			sink.Start(ctx)
			pluginManager := pluginlifecycle.New(repo, nc.Conn, 15*time.Second, log)
			go pluginManager.Run(ctx)

			cmCli, err := client.NewCMClient(ctx, nc, cfg.ConnectionManagerAddr)
			if err != nil {
				log.Fatal("cm client init failed", zap.Error(err))
			}
			defer cmCli.Close()

			if strings.EqualFold(cfg.Environment, "production") && cfg.EncryptionKey == "" {
				log.Fatal("encryption_key is required in production")
			}
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
			defer observability.Shutdown(metrics, log)
			<-ctx.Done()
			sink.Wait()
			log.Info("shutting down meta-service")
		},
	}

	cmd.Flags().String("app_name", "meta-service", "application name")
	cmd.Flags().String("environment", "development", "runtime environment")
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
