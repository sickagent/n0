package main

import (
	"context"
	"testing"

	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestDemoAdapter(t *testing.T) {
	t.Parallel()
	adapter := demoAdapter{}

	params, err := structpb.NewStruct(map[string]any{"api_key": "development-only"})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := adapter.TestConnection(context.Background(), &pb.AdapterTestConnectionRequest{Params: params})
	if err != nil || !connection.Ok {
		t.Fatalf("expected successful connection test, response=%v err=%v", connection, err)
	}

	schema, err := adapter.GetSchema(context.Background(), &pb.AdapterGetSchemaRequest{})
	if err != nil || len(schema.Tables) != 1 || schema.Tables[0].Name != "demo_metrics" {
		t.Fatalf("unexpected schema: response=%v err=%v", schema, err)
	}

	result, err := adapter.ExecuteQuery(context.Background(), &pb.AdapterExecuteQueryRequest{Query: "SELECT * FROM demo_metrics;"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 2 || len(result.Columns) != 2 {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestDemoAdapterRejectsUnsupportedQuery(t *testing.T) {
	t.Parallel()
	_, err := (demoAdapter{}).ExecuteQuery(context.Background(), &pb.AdapterExecuteQueryRequest{Query: "DELETE FROM demo_metrics"})
	if err == nil {
		t.Fatal("expected unsupported query to fail")
	}
}

func TestDemoAdapterRequiresAPIKey(t *testing.T) {
	t.Parallel()
	_, err := (demoAdapter{}).TestConnection(context.Background(), &pb.AdapterTestConnectionRequest{})
	if err == nil {
		t.Fatal("expected missing api_key to fail")
	}
}
