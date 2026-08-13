package config

import (
	"strings"
	"testing"
)

func TestLoadRejectsInvalidIntegerEnvironmentValue(t *testing.T) {
	t.Setenv("N0_WORKER_COUNT", "many")
	cfg := struct {
		WorkerCount int `mapstructure:"worker_count"`
	}{}

	err := Load(&cfg)
	if err == nil || !strings.Contains(err.Error(), "invalid integer") {
		t.Fatalf("expected invalid integer error, got %v", err)
	}
}

func TestLoadPrefersNamespacedEnvironmentValue(t *testing.T) {
	t.Setenv("WORKER_COUNT", "2")
	t.Setenv("N0_WORKER_COUNT", "8")
	cfg := struct {
		WorkerCount int `mapstructure:"worker_count"`
	}{}

	if err := Load(&cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.WorkerCount != 8 {
		t.Fatalf("expected namespaced value 8, got %d", cfg.WorkerCount)
	}
}

func TestLoadRejectsNonPointer(t *testing.T) {
	if err := Load(struct{}{}); err == nil {
		t.Fatal("expected non-pointer config to fail")
	}
}
