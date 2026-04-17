package main

import (
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"n0/pkg/shared/config"
	"n0/pkg/shared/graceful"
	"n0/pkg/shared/logger"
	"n0/pkg/shared/natsclient"
	"n0/pkg/shared/observability"
	"n0/services/query-engine/internal/client"
	"n0/services/query-engine/internal/server"
	"n0/services/query-engine/internal/worker"
	"github.com/nats-io/nats.go/jetstream"
)

type Config struct {
	config.BaseConfig
	GRPCAddr              string `mapstructure:"grpc_addr"`
	HTTPAddr              string `mapstructure:"http_addr"`
	RedisAddr             string `mapstructure:"redis_addr"`
	WorkerCount           int    `mapstructure:"worker_count"`
	ConnectionManagerAddr string `mapstructure:"connection_manager_addr"`
}

func main() {
	var cfg Config
	cmd := &cobra.Command{
		Use:   "query-engine",
		Short: "n0 Query Engine",
		Run: func(_ *cobra.Command, _ []string) {
			cobra.CheckErr(config.Load(&cfg))
			log := logger.New(cfg.LogLevel)
			log.Info("starting query-engine", zap.Int("workers", cfg.WorkerCount))

			ctx, cancel := graceful.ContextWithShutdown(30 * time.Second)
			defer cancel()

			nc, err := natsclient.New(cfg.NATSURL, 5*time.Second, log)
			if err != nil {
				log.Fatal("nats connect failed", zap.Error(err))
			}
			defer nc.Close()

			_, err = nc.EnsureStream(ctx, jetstream.StreamConfig{
				Name:     "QUERIES",
				Subjects: []string{"QUERIES.*"},
				Replicas: 1,
			})
			if err != nil {
				log.Fatal("stream ensure failed", zap.Error(err))
			}

			cons, err := nc.JS.CreateConsumer(ctx, "QUERIES", jetstream.ConsumerConfig{
				Durable:   "query-workers",
				Name:      "query-workers",
				Replicas:  1,
				AckPolicy: jetstream.AckExplicitPolicy,
			})
			if err != nil {
				log.Fatal("consumer create failed", zap.Error(err))
			}

			cmCli, err := client.NewConnectionManagerClient(cfg.ConnectionManagerAddr)
			if err != nil {
				log.Fatal("connection manager client init failed", zap.Error(err))
			}
			defer cmCli.Close()

			proc := worker.NewQueryProcessor(log, cmCli)
			pool := worker.NewPool(cons, proc, log, cfg.WorkerCount)
			pool.Start(ctx)
			defer pool.Stop()

			grpcSrv, err := server.StartGRPC(cfg.GRPCAddr, log)
			if err != nil {
				log.Fatal("grpc start failed", zap.Error(err))
			}
			defer grpcSrv.GracefulStop()

			grpcHandler := server.NewGRPCServer(log)
			httpSrv := server.NewHTTPServer(cfg.HTTPAddr, log, grpcHandler)
			go func() {
				if err := httpSrv.Start(ctx); err != nil {
					log.Error("http server error", zap.Error(err))
				}
			}()

			metrics := observability.StartMetricsServer(":9090", log)
			defer func() { _ = metrics.Close() }()

			<-ctx.Done()
			log.Info("shutting down query-engine")
		},
	}

	cmd.Flags().String("app_name", "query-engine", "application name")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("http_addr", ":8082", "HTTP listen address")
	cmd.Flags().String("redis_addr", "localhost:6379", "Redis address")
	cmd.Flags().Int("worker_count", 4, "number of query workers")
	cmd.Flags().String("connection_manager_addr", "localhost:8081", "Connection Manager gRPC address")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}
