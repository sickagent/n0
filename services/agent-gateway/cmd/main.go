package main

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/sickagent/n0/pkg/shared/config"
	"github.com/sickagent/n0/pkg/shared/graceful"
	"github.com/sickagent/n0/pkg/shared/jwt"
	"github.com/sickagent/n0/pkg/shared/logger"
	"github.com/sickagent/n0/pkg/shared/natsclient"
	"github.com/sickagent/n0/pkg/shared/observability"
	"github.com/sickagent/n0/services/agent-gateway/internal/client"
	"github.com/sickagent/n0/services/agent-gateway/internal/gateway"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type Config struct {
	config.BaseConfig
	GRPCAddr              string `mapstructure:"grpc_addr"`
	HTTPAddr              string `mapstructure:"http_addr"`
	MetaServiceAddr       string `mapstructure:"meta_service_addr"`
	MetaServiceHTTPURL    string `mapstructure:"meta_service_http_url"`
	QueryEngineAddr       string `mapstructure:"query_engine_addr"`
	ConnectionManagerAddr string `mapstructure:"connection_manager_addr"`
	JWTSecret             string `mapstructure:"jwt_secret"`
	JWTExpiryHours        int    `mapstructure:"jwt_expiry_hours"`
	CORSAllowedOrigins    string `mapstructure:"cors_allowed_origins"`
}

func main() {
	var cfg Config
	cmd := &cobra.Command{
		Use:   "agent-gateway",
		Short: "n0 Agent Gateway",
		Run: func(_ *cobra.Command, _ []string) {
			cobra.CheckErr(config.Load(&cfg))
			log := logger.New(cfg.LogLevel)
			log.Info("starting agent-gateway",
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

			metaCli, err := client.NewMetaClient(ctx, nc, cfg.MetaServiceAddr)
			if err != nil {
				log.Fatal("meta client init failed", zap.Error(err))
			}
			defer metaCli.Close()

			queryCli, err := client.NewQueryEngineClient(ctx, nc, cfg.QueryEngineAddr)
			if err != nil {
				log.Fatal("query engine client init failed", zap.Error(err))
			}
			defer queryCli.Close()

			cmCli, err := client.NewConnectionManagerClient(ctx, nc, cfg.ConnectionManagerAddr)
			if err != nil {
				log.Fatal("connection manager client init failed", zap.Error(err))
			}
			defer cmCli.Close()

			if strings.EqualFold(cfg.Environment, "production") && cfg.JWTSecret == "" {
				log.Fatal("jwt_secret is required in production")
			}
			var jwtManager *jwt.Manager
			if cfg.JWTSecret != "" {
				secret, err := base64.StdEncoding.DecodeString(cfg.JWTSecret)
				if err != nil {
					log.Fatal("invalid jwt_secret: must be base64", zap.Error(err))
				}
				if len(secret) < 32 {
					log.Fatal("invalid jwt_secret: decoded key must be at least 32 bytes")
				}
				expiry := time.Duration(cfg.JWTExpiryHours) * time.Hour
				if expiry <= 0 {
					expiry = 24 * time.Hour
				}
				jwtManager = jwt.NewManager(secret, "n0-gateway", expiry)
			}

			server := gateway.NewServer(cfg.GRPCAddr, cfg.HTTPAddr, log, metaCli, queryCli, cmCli, jwtManager, strings.Split(cfg.CORSAllowedOrigins, ",")...)
			server.SetMetaHTTPBase(cfg.MetaServiceHTTPURL)
			go func() {
				if err := server.Start(ctx); err != nil {
					log.Error("gateway server error", zap.Error(err))
				}
			}()

			metrics := observability.StartMetricsServer(":9090", log)
			defer observability.Shutdown(metrics, log)

			<-ctx.Done()
			log.Info("shutting down agent-gateway")
		},
	}

	cmd.Flags().String("app_name", "agent-gateway", "application name")
	cmd.Flags().String("environment", "development", "runtime environment")
	cmd.Flags().String("log_level", "info", "log level")
	cmd.Flags().String("nats_url", "nats://localhost:4222", "NATS URL")
	cmd.Flags().String("grpc_addr", ":8080", "gRPC listen address")
	cmd.Flags().String("http_addr", ":8081", "HTTP listen address")
	cmd.Flags().String("meta_service_addr", "localhost:8080", "Meta Service gRPC address")
	cmd.Flags().String("meta_service_http_url", "http://localhost:8081", "Meta Service internal HTTP base URL")
	cmd.Flags().String("query_engine_addr", "localhost:8082", "Query Engine gRPC address")
	cmd.Flags().String("connection_manager_addr", "localhost:8081", "Connection Manager gRPC address")
	cmd.Flags().String("jwt_secret", "", "Base64-encoded JWT secret")
	cmd.Flags().Int("jwt_expiry_hours", 24, "JWT expiry in hours")
	cmd.Flags().String("cors_allowed_origins", "http://localhost:3000", "comma-separated allowed browser origins")

	cobra.CheckErr(config.InitCobra(cmd, "N0"))
	cobra.CheckErr(cmd.Execute())
}
