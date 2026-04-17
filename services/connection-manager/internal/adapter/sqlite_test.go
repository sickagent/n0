package adapter

import (
	"context"
	"os"
	"testing"
)

func TestSQLiteAdapter_TestConnection(t *testing.T) {
	a := NewSQLiteAdapter()
	ctx := context.Background()
	if err := a.TestConnection(ctx, ":memory:"); err != nil {
		t.Fatalf("test connection failed: %v", err)
	}
}

func TestSQLiteAdapter_ExecuteQuery(t *testing.T) {
	a := NewSQLiteAdapter()
	ctx := context.Background()
	cid := "test-sqlite"

	if err := a.Prepare(cid, ":memory:"); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer a.Release(cid)

	db := a.pools[cid]
	if _, err := db.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO users (name) VALUES ('Alice'), ('Bob')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	rows, err := a.ExecuteQuery(ctx, cid, "SELECT * FROM users ORDER BY id", 10)
	if err != nil {
		t.Fatalf("execute query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", rows[0]["name"])
	}
}

func TestSQLiteAdapter_GetSchema(t *testing.T) {
	a := NewSQLiteAdapter()
	ctx := context.Background()
	path := "/tmp/test_sqlite_schema.db"
	cid := "test-schema"

	defer func() { _ = os.Remove(path) }()

	if err := a.Prepare(cid, path); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}
	defer a.Release(cid)

	db := a.pools[cid]
	if _, err := db.ExecContext(ctx, "CREATE TABLE products (id INTEGER PRIMARY KEY, price REAL NOT NULL)"); err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	tables, err := a.GetSchema(ctx, path)
	if err != nil {
		t.Fatalf("get schema failed: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("expected at least one table")
	}

	var found bool
	for _, tbl := range tables {
		if tbl.Name == "products" {
			found = true
			if len(tbl.Columns) != 2 {
				t.Errorf("expected 2 columns, got %d", len(tbl.Columns))
			}
		}
	}
	if !found {
		t.Errorf("products table not found in schema")
	}
}
