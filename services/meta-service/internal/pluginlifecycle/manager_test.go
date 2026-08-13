package pluginlifecycle

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sickagent/n0/services/meta-service/internal/app"
	"go.uber.org/zap"
)

type fakeRepo struct {
	plugin  app.PluginDefinition
	healthy bool
	calls   int
}

func (r *fakeRepo) ListPluginsForHealth(context.Context) ([]app.PluginDefinition, error) {
	return []app.PluginDefinition{r.plugin}, nil
}
func (r *fakeRepo) RecordPluginHealth(context.Context, uuid.UUID, bool, time.Duration, string) (string, bool, error) {
	r.calls++
	return "active", true, nil
}

type fakePublisher struct {
	subject string
	data    []byte
}

func (p *fakePublisher) Publish(subject string, data []byte) error {
	p.subject, p.data = subject, data
	return nil
}

func TestPublishRouteContract(t *testing.T) {
	p := &fakePublisher{}
	m := New(&fakeRepo{}, p, time.Second, zap.NewNop())
	plugin := app.PluginDefinition{ID: uuid.New(), PluginType: "DB_ADAPTER", Name: "warehouse", Endpoint: "adapter:8080"}
	m.publishRoute(plugin, "active")
	if p.subject != "events.plugin.route" {
		t.Fatalf("unexpected subject %q", p.subject)
	}
	var route Route
	if err := json.Unmarshal(p.data, &route); err != nil {
		t.Fatal(err)
	}
	if route.AdapterType != "warehouse" || route.Endpoint != "adapter:8080" || route.Status != "active" {
		t.Fatalf("unexpected route %+v", route)
	}
}
