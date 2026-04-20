package main

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"n0/pkg/shared/config"
	"n0/pkg/shared/discovery"
	"n0/pkg/shared/graceful"
	"n0/pkg/shared/logger"
	"n0/pkg/shared/natsclient"
	"n0/pkg/shared/observability"
	"n0/services/query-engine/internal/client"
	"n0/services/query-engine/internal/job"
	"n0/services/query-engine/internal/server"
	"n0/services/query-engine/internal/worker"
)

type Config struct {
	config.BaseConfig
	GRPCAddr              string `mapstructure:"grpc_addr"`
	GRPCAdvertiseAddr     string `mapstructure:"grpc_advertise_addr"`
	HTTPAddr              string `mapstructure:"http_addr"`
	RedisAddr             string `mapstructure:"redis_addr"`
	WorkerCount           int    `mapstructure:"worker_count"`
	MetaServiceAddr       string `mapstructure:"meta_service_addr"`
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

			metaCli, err := client.NewMetaClient(ctx, nc, cfg.MetaServiceAddr)
			if err != nil {
				log.Fatal("meta-service client init failed", zap.Error(err))
			}
			defer metaCli.Close()

			cmCli, err := client.NewConnectionManagerClient(ctx, nc, cfg.ConnectionManagerAddr)
			if err != nil {
				log.Fatal("connection manager client init failed", zap.Error(err))
			}
			defer cmCli.Close()

			store := job.NewStore()
			proc := worker.NewQueryProcessor(log, cmCli, metaCli, store)
			pool := worker.NewPool(cons, proc, log, cfg.WorkerCount)
			pool.Start(ctx)
			defer pool.Stop()

			grpcHandler := server.NewGRPCServer(log, store, nc.Conn)
			grpcSrv, err := server.StartGRPC(cfg.GRPCAddr, grpcHandler, log)
			if err != nil {
				log.Fatal("grpc start failed", zap.Error(err))
			}
			defer grpcSrv.GracefulStop()

			discoverySub, err := discovery.RegisterGRPCResponder(nc, "query-engine", cfg.GRPCAddr, cfg.GRPCAdvertiseAddr, log)
			if err != nil {
				log.Fatal("grpc discovery register failed", zap.Error(err))
			}
			defer discoverySub.Unsubscribe()

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
	cmd.Flags().String("grpc_advertise_addr", "", "advertised gRPC address for discovery")
	cmd.Flags().String("http_addr", ":8082", "HTTP listen address")
	cmd.Flags().String("redis_addr", "localhost:6379", "Redis address")
	cmd.Flags().Int("worker_count", 4, "number of query workers")
	cmd.Flags().String("meta_service_addr", "localhost:8080", "Meta Service gRPC address")
	cmd.Flags().String("connection_manager_addr", "localhost:8081", "Connection Manager gRPC address")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}
