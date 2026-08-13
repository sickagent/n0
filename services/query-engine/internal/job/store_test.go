package job

import (
	"reflect"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestStoreLifecycleAndPagination(t *testing.T) {
	store := NewStore()
	record := store.Create(Record{
		ID:           "job-1",
		ConnectionID: "conn-1",
		SQL:          "SELECT 1",
	})

	if record.Status != StatusPending {
		t.Fatalf("expected pending, got %s", record.Status)
	}

	if err := store.MarkRunning(record.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if err := store.MarkSucceeded(record.ID, []map[string]any{
		{"value": 1},
		{"value": 2},
	}, false); err != nil {
		t.Fatalf("mark success: %v", err)
	}

	got, rows, nextToken, err := store.GetResultPage(record.ID, 1, 1)
	if err != nil {
		t.Fatalf("get result page: %v", err)
	}
	if got.Status != StatusSuccess {
		t.Fatalf("expected success, got %s", got.Status)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if nextToken != "2" {
		t.Fatalf("expected next token 2, got %q", nextToken)
	}
}

func TestStoreMarkFailed(t *testing.T) {
	store := NewStore()
	store.Create(Record{ID: "job-fail"})

	if err := store.MarkFailed("job-fail", "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	got, err := store.Get("job-fail")
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}
	if got.ErrorMessage != "boom" {
		t.Fatalf("expected error message boom, got %q", got.ErrorMessage)
	}
}

func TestNewRedisClientStandalone(t *testing.T) {
	client, err := newRedisClient(DurableConfig{
		RedisMode: "standalone", RedisAddr: "redis:6379", RedisUsername: "app", RedisPassword: "secret", RedisDB: 2,
	})
	if err != nil {
		t.Fatalf("create standalone client: %v", err)
	}
	defer client.Close()

	standalone, ok := client.(*redis.Client)
	if !ok {
		t.Fatalf("expected standalone client, got %T", client)
	}
	opts := standalone.Options()
	if opts.Addr != "redis:6379" || opts.Username != "app" || opts.Password != "secret" || opts.DB != 2 {
		t.Fatalf("unexpected standalone options: %#v", opts)
	}
}

func TestNewRedisClientCluster(t *testing.T) {
	client, err := newRedisClient(DurableConfig{
		RedisMode: "cluster", RedisAddr: "ignored:6379",
		RedisAddrs: []string{" redis-0:6379 ", "", "redis-1:6379"},
	})
	if err != nil {
		t.Fatalf("create cluster client: %v", err)
	}
	defer client.Close()

	cluster, ok := client.(*redis.ClusterClient)
	if !ok {
		t.Fatalf("expected cluster client, got %T", client)
	}
	if want := []string{"redis-0:6379", "redis-1:6379"}; !reflect.DeepEqual(cluster.Options().Addrs, want) {
		t.Fatalf("cluster addresses = %v, want %v", cluster.Options().Addrs, want)
	}
}

func TestNewRedisClientRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  DurableConfig
	}{
		{name: "missing address", cfg: DurableConfig{RedisMode: "standalone"}},
		{name: "multiple standalone addresses", cfg: DurableConfig{RedisMode: "standalone", RedisAddrs: []string{"a:6379", "b:6379"}}},
		{name: "cluster database", cfg: DurableConfig{RedisMode: "cluster", RedisAddr: "a:6379", RedisDB: 1}},
		{name: "unknown mode", cfg: DurableConfig{RedisMode: "sentinel", RedisAddr: "a:6379"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if client, err := newRedisClient(tt.cfg); err == nil {
				client.Close()
				t.Fatal("expected configuration error")
			}
		})
	}
}
