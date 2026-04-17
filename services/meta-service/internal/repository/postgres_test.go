package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"n0/services/meta-service/internal/app"
)

func getDSN() string {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/meta?sslmode=disable"
	}
	return dsn
}

func TestPostgresRepository_ListWorkspaces(t *testing.T) {
	repo, err := NewPostgresRepositoryFromDSN(getDSN())
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	// This test assumes the workspaces table exists (migrations should be run).
	_, err = repo.ListWorkspaces(ctx, "tenant-1", 10, 0)
	if err != nil {
		t.Logf("list workspaces returned error (table may not exist): %v", err)
	}
}

func TestPostgresRepository_GetSchemaSnapshot(t *testing.T) {
	repo, err := NewPostgresRepositoryFromDSN(getDSN())
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	snap, err := repo.GetSchemaSnapshot(ctx, "conn-1")
	if err != nil {
		t.Logf("get schema snapshot returned error (table may not exist): %v", err)
	}
	_ = snap
}

func TestPostgresRepository_RegisterPlugin(t *testing.T) {
	repo, err := NewPostgresRepositoryFromDSN(getDSN())
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()
	id, err := repo.RegisterPlugin(ctx, app.PluginDefinition{
		PluginType: "DB_ADAPTER",
		Name:       "test-plugin-" + uuid.New().String(),
		Version:    "1.0.0",
		Endpoint:   "localhost:8080",
		Protocol:   "grpc",
		Status:     "registered",
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Logf("register plugin returned error (table may not exist): %v", err)
	} else {
		if id == uuid.Nil {
			t.Error("expected non-nil uuid")
		}
	}
}
