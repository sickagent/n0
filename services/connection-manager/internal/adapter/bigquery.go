package adapter

import (
	"context"
	"errors"

	"n0/pkg/shared/adapter"
)

// BigQueryAdapter is a stub for Google BigQuery.
type BigQueryAdapter struct{}

// NewBigQueryAdapter creates a new BigQuery adapter stub.
func NewBigQueryAdapter() *BigQueryAdapter {
	return &BigQueryAdapter{}
}

// TestConnection validates connectivity (stub).
func (a *BigQueryAdapter) TestConnection(ctx context.Context, dsn string) error {
	return errors.New("bigquery adapter not yet implemented")
}

// Prepare is a no-op for the stub.
func (a *BigQueryAdapter) Prepare(connectionID string, dsn string) error {
	return nil
}

// Release is a no-op for the stub.
func (a *BigQueryAdapter) Release(connectionID string) error {
	return nil
}

// ExecuteQuery runs a query (stub).
func (a *BigQueryAdapter) ExecuteQuery(ctx context.Context, connectionID string, sql string, limit int32) ([]map[string]any, error) {
	return nil, errors.New("bigquery adapter not yet implemented")
}

// GetSchema returns schema metadata (stub).
func (a *BigQueryAdapter) GetSchema(ctx context.Context, dsn string) ([]adapter.TableInfo, error) {
	return nil, errors.New("bigquery adapter not yet implemented")
}
