package adapter

import (
	"context"
	"os"
	"testing"
)

func getPostgresDSN() string {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/meta?sslmode=disable"
	}
	return dsn
}

func TestPostgresAdapter_TestConnection(t *testing.T) {
	dsn := getPostgresDSN()
	a := NewPostgresAdapter()
	ctx := context.Background()
	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("postgres not available: %v", err)
	}
}

func TestPostgresAdapter_ExecuteQuery(t *testing.T) {
	dsn := getPostgresDSN()
	a := NewPostgresAdapter()
	ctx := context.Background()
	cid := "test-postgres"

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	if err := a.Prepare(cid, dsn); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer a.Release(cid)

	pool := a.pools[cid]
	if _, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS test_users (id SERIAL PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE test_users RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO test_users (name) VALUES ('Alice'), ('Bob')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	rows, err := a.ExecuteQuery(ctx, cid, "SELECT * FROM test_users ORDER BY id", 10)
	if err != nil {
		t.Fatalf("execute query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestPostgresAdapter_GetSchema(t *testing.T) {
	dsn := getPostgresDSN()
	a := NewPostgresAdapter()
	ctx := context.Background()

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	tables, err := a.GetSchema(ctx, dsn)
	if err != nil {
		t.Fatalf("get schema failed: %v", err)
	}
	if len(tables) == 0 {
		t.Log("no tables found in public schema")
	}
}
