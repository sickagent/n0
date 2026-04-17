package adapter

import (
	"context"
	"os"
	"testing"
)

func getMySQLDSN() string {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(localhost:3306)/meta?parseTime=true"
	}
	return dsn
}

func TestMySQLAdapter_TestConnection(t *testing.T) {
	dsn := getMySQLDSN()
	a := NewMySQLAdapter()
	ctx := context.Background()
	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("mysql not available: %v", err)
	}
}

func TestMySQLAdapter_ExecuteQuery(t *testing.T) {
	dsn := getMySQLDSN()
	a := NewMySQLAdapter()
	ctx := context.Background()
	cid := "test-mysql"

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("mysql not available: %v", err)
	}

	if err := a.Prepare(cid, dsn); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer a.Release(cid)

	db := a.pools[cid]
	if _, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS test_users (id INT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(255))"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE test_users"); err != nil {
		t.Fatalf("truncate failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO test_users (name) VALUES ('Alice'), ('Bob')"); err != nil {
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

func TestMySQLAdapter_GetSchema(t *testing.T) {
	dsn := getMySQLDSN()
	a := NewMySQLAdapter()
	ctx := context.Background()

	if err := a.TestConnection(ctx, dsn); err != nil {
		t.Skipf("mysql not available: %v", err)
	}

	tables, err := a.GetSchema(ctx, dsn)
	if err != nil {
		t.Fatalf("get schema failed: %v", err)
	}
	if len(tables) == 0 {
		t.Log("no tables found in database")
	}
}
