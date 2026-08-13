package pluginlifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sickagent/n0/services/meta-service/internal/app"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Repository interface {
	ListPluginsForHealth(context.Context) ([]app.PluginDefinition, error)
	RecordPluginHealth(context.Context, uuid.UUID, bool, time.Duration, string) (string, bool, error)
}

type Publisher interface{ Publish(string, []byte) error }

type Route struct {
	PluginID    string `json:"plugin_id"`
	AdapterType string `json:"adapter_type"`
	Endpoint    string `json:"endpoint"`
	Status      string `json:"status"`
}

type Manager struct {
	repo      Repository
	publisher Publisher
	interval  time.Duration
	log       *zap.Logger
}

func New(repo Repository, publisher Publisher, interval time.Duration, log *zap.Logger) *Manager {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Manager{repo: repo, publisher: publisher, interval: interval, log: log}
}

func (m *Manager) Run(ctx context.Context) {
	m.check(ctx)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *Manager) check(ctx context.Context) {
	plugins, err := m.repo.ListPluginsForHealth(ctx)
	if err != nil {
		m.log.Error("list plugins for health failed", zap.Error(err))
		return
	}
	for _, plugin := range plugins {
		started := time.Now()
		err := probe(ctx, plugin.Endpoint)
		status, changed, saveErr := m.repo.RecordPluginHealth(ctx, plugin.ID, err == nil, time.Since(started), errorString(err))
		if saveErr != nil {
			m.log.Error("record plugin health failed", zap.String("plugin_id", plugin.ID.String()), zap.Error(saveErr))
			continue
		}
		if changed || status == "active" {
			m.publishRoute(plugin, status)
		}
	}
}

func probe(parent context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("health status %s", resp.Status)
	}
	return nil
}

func (m *Manager) publishRoute(plugin app.PluginDefinition, status string) {
	if !strings.EqualFold(plugin.PluginType, "DB_ADAPTER") {
		return
	}
	payload, _ := json.Marshal(Route{PluginID: plugin.ID.String(), AdapterType: plugin.Name, Endpoint: plugin.Endpoint, Status: status})
	if err := m.publisher.Publish("events.plugin.route", payload); err != nil {
		m.log.Error("publish plugin route failed", zap.Error(err))
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
