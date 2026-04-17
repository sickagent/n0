package adapter

import (
	"context"
	"os"
	"testing"
)

func getClickHouseDSN() string {
	dsn := os.Getenv("CLICKHOUSE_DSN")
	if dsn == "" {
		dsn = "clickhouse://default:@localhost:9000?database=default"
	}
	return dsn
}

func TestClickHouseAdapter_TestConnection(t *testing.T) {
	dsn := getClickHouseDSN()
	a := NewClickHouseAdapter()
	ctx := context.Background()
	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("clickhouse not available: %v", err)
	}
}

func TestClickHouseAdapter_ExecuteQuery(t *testing.T) {
	dsn := getClickHouseDSN()
	a := NewClickHouseAdapter()
	ctx := context.Background()
	cid := "test-clickhouse"

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("clickhouse not available: %v", err)
	}

	if err := a.Prepare(cid, dsn); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer a.Release(cid)

	db := a.pools[cid]
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS test_users (id UInt32, name String) ENGINE = Memory"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE test_users"); err != nil {
		t.Logf("truncate may not be supported on Memory engine: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO test_users VALUES (1, 'Alice'), (2, 'Bob')"); err != nil {
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

func TestClickHouseAdapter_GetSchema(t *testing.T) {
	dsn := getClickHouseDSN()
	a := NewClickHouseAdapter()
	ctx := context.Background()

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("clickhouse not available: %v", err)
	}

	tables, err := a.GetSchema(ctx, dsn)
	if err != nil {
		t.Fatalf("get schema failed: %v", err)
	}
	if len(tables) == 0 {
		t.Log("no tables found in default database")
	}
}
