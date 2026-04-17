package dsn

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestBuildDSN_Postgres(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"host":     "localhost",
		"port":     "5432",
		"user":     "postgres",
		"password": "secret",
		"database": "meta",
	})
	dsn, err := BuildDSN("postgres", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "postgres://postgres:secret@localhost:5432/meta?sslmode=disable"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_ClickHouse(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"host":     "ch",
		"port":     "9000",
		"user":     "default",
		"password": "",
		"database": "default",
	})
	dsn, err := BuildDSN("clickhouse", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "clickhouse://default:@ch:9000?database=default"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_MySQL(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"host":     "mysql",
		"port":     "3306",
		"user":     "root",
		"password": "root",
		"database": "meta",
	})
	dsn, err := BuildDSN("mysql", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "root:root@tcp(mysql:3306)/meta?parseTime=true"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_SQLite(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"path": ":memory:",
	})
	dsn, err := BuildDSN("sqlite", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dsn != ":memory:" {
		t.Errorf("expected :memory:, got %q", dsn)
	}
}

func TestBuildDSN_MSSQL(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"host":     "mssql",
		"port":     "1433",
		"user":     "sa",
		"password": "Password123",
		"database": "meta",
	})
	dsn, err := BuildDSN("mssql", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "sqlserver://sa:Password123@mssql:1433?database=meta"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_BigQuery(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{
		"project_id": "my-project",
		"location":   "EU",
	})
	dsn, err := BuildDSN("bigquery", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "bigquery://my-project?location=EU"
	if dsn != expected {
		t.Errorf("expected %q, got %q", expected, dsn)
	}
}

func TestBuildDSN_BigQuery_MissingProject(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{})
	_, err := BuildDSN("bigquery", params)
	if err == nil {
		t.Fatal("expected error for missing project_id")
	}
}

func TestBuildDSN_Unknown(t *testing.T) {
	params, _ := structpb.NewStruct(map[string]any{})
	_, err := BuildDSN("oracle", params)
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
}
