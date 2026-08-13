package main

import (
	"context"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sickagent/n0/pkg/shared/audit"
	"github.com/sickagent/n0/pkg/shared/config"
	"github.com/sickagent/n0/pkg/shared/discovery"
	"github.com/sickagent/n0/pkg/shared/graceful"
	"github.com/sickagent/n0/pkg/shared/logger"
	"github.com/sickagent/n0/pkg/shared/natsclient"
	"github.com/sickagent/n0/pkg/shared/observability"
	"github.com/sickagent/n0/services/query-engine/internal/client"
	"github.com/sickagent/n0/services/query-engine/internal/job"
	"github.com/sickagent/n0/services/query-engine/internal/server"
	"github.com/sickagent/n0/services/query-engine/internal/worker"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type Config struct {
	config.BaseConfig
	GRPCAddr              string `mapstructure:"grpc_addr"`
	GRPCAdvertiseAddr     string `mapstructure:"grpc_advertise_addr"`
	HTTPAddr              string `mapstructure:"http_addr"`
	RedisMode             string `mapstructure:"redis_mode"`
	RedisAddr             string `mapstructure:"redis_addr"`
	RedisAddrs            string `mapstructure:"redis_addrs"`
	RedisUsername         string `mapstructure:"redis_username"`
	RedisPassword         string `mapstructure:"redis_password"`
	RedisDB               int    `mapstructure:"redis_db"`
	JobTTLHours           int    `mapstructure:"job_ttl_hours"`
	S3Endpoint            string `mapstructure:"s3_endpoint"`
	S3AccessKey           string `mapstructure:"s3_access_key"`
	S3SecretKey           string `mapstructure:"s3_secret_key"`
	S3Bucket              string `mapstructure:"s3_bucket"`
	S3UseSSL              bool   `mapstructure:"s3_use_ssl"`
	ResultInlineMaxBytes  int    `mapstructure:"result_inline_max_bytes"`
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

			queryStream, err := nc.EnsureStream(ctx, jetstream.StreamConfig{
				Name:     "QUERIES",
				Subjects: []string{"QUERIES.*"},
				Replicas: 1,
			})
			if err != nil {
				log.Fatal("stream ensure failed", zap.Error(err))
			}
			if _, err := nc.EnsureStream(ctx, jetstream.StreamConfig{
				Name: "AUDIT", Subjects: []string{"audit.events.>"}, Replicas: 1,
				Retention: jetstream.LimitsPolicy, MaxAge: 30 * 24 * time.Hour,
			}); err != nil {
				log.Fatal("audit stream ensure failed", zap.Error(err))
			}

			cons, err := queryStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
				Durable:    "query-workers",
				Name:       "query-workers",
				Replicas:   1,
				AckPolicy:  jetstream.AckExplicitPolicy,
				AckWait:    90 * time.Second,
				MaxDeliver: 3,
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

			var objectClient *minio.Client
			if cfg.S3Endpoint != "" {
				objectClient, err = minio.New(cfg.S3Endpoint, &minio.Options{
					Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
					Secure: cfg.S3UseSSL,
				})
				if err != nil {
					log.Fatal("object storage client init failed", zap.Error(err))
				}
				if err := ensureBucket(ctx, objectClient, cfg.S3Bucket, cfg.JobTTLHours); err != nil {
					log.Fatal("object storage bucket init failed", zap.Error(err))
				}
			}
			store, err := job.NewDurableStore(ctx, job.DurableConfig{
				RedisMode: cfg.RedisMode, RedisAddr: cfg.RedisAddr, RedisAddrs: strings.Split(cfg.RedisAddrs, ","),
				RedisUsername: cfg.RedisUsername, RedisPassword: cfg.RedisPassword, RedisDB: cfg.RedisDB,
				TTL: time.Duration(cfg.JobTTLHours) * time.Hour, ObjectClient: objectClient,
				ObjectBucket: cfg.S3Bucket, InlineMaxBytes: cfg.ResultInlineMaxBytes,
			})
			if err != nil {
				if strings.EqualFold(cfg.Environment, "production") {
					log.Fatal("durable job store init failed", zap.Error(err))
				}
				log.Warn("durable job store unavailable; using development memory store", zap.Error(err))
				store = job.NewStore()
			}
			defer func() { _ = store.Close() }()
			proc := worker.NewQueryProcessor(log, cmCli, metaCli, store, audit.NewJetStreamPublisher(nc.JS, 5*time.Second))
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
			defer observability.Shutdown(metrics, log)

			<-ctx.Done()
			log.Info("shutting down query-engine")
		},
	}

	cmd.Flags().String("app_name", "query-engine", "application name")
	cmd.Flags().String("environment", "development", "runtime environment")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("grpc_advertise_addr", "", "advertised gRPC address for discovery")
	cmd.Flags().String("http_addr", ":8082", "HTTP listen address")
	cmd.Flags().String("redis_mode", "standalone", "Redis mode: standalone or cluster")
	cmd.Flags().String("redis_addr", "localhost:6379", "Redis address")
	cmd.Flags().String("redis_addrs", "", "comma-separated Redis Cluster seed addresses (overrides redis_addr)")
	cmd.Flags().String("redis_username", "", "Redis ACL username")
	cmd.Flags().String("redis_password", "", "Redis password")
	cmd.Flags().Int("redis_db", 0, "Redis database")
	cmd.Flags().Int("job_ttl_hours", 24, "job metadata and result TTL")
	cmd.Flags().String("s3_endpoint", "", "S3-compatible endpoint without scheme")
	cmd.Flags().String("s3_access_key", "", "S3 access key")
	cmd.Flags().String("s3_secret_key", "", "S3 secret key")
	cmd.Flags().String("s3_bucket", "n0-results", "S3 result bucket")
	cmd.Flags().Bool("s3_use_ssl", false, "use TLS for S3 endpoint")
	cmd.Flags().Int("result_inline_max_bytes", 1048576, "maximum result size stored inline in Redis")
	cmd.Flags().Int("worker_count", 4, "number of query workers")
	cmd.Flags().String("meta_service_addr", "localhost:8080", "Meta Service gRPC address")
	cmd.Flags().String("connection_manager_addr", "localhost:8081", "Connection Manager gRPC address")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}

func ensureBucket(ctx context.Context, client *minio.Client, bucket string, ttlHours int) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if exists {
		return ensureBucketLifecycle(ctx, client, bucket, ttlHours)
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		return err
	}
	return ensureBucketLifecycle(ctx, client, bucket, ttlHours)
}

func ensureBucketLifecycle(ctx context.Context, client *minio.Client, bucket string, ttlHours int) error {
	days := (ttlHours + 23) / 24
	if days < 1 {
		days = 1
	}
	return client.SetBucketLifecycle(ctx, bucket, &lifecycle.Configuration{Rules: []lifecycle.Rule{{
		ID: "expire-query-results", Status: "Enabled", RuleFilter: lifecycle.Filter{Prefix: "results/"},
		Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(days)},
	}}})
}
