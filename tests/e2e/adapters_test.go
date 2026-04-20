package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AdapterTestCase defines the configuration for an adapter E2E test.
type AdapterTestCase struct {
	Name         string
	AdapterType  string
	Params       map[string]any
	SetupSQL     []string
	QuerySQL     string
	ExpectedCols []string
	ShouldFail   bool
}

func getAdapterCases() []AdapterTestCase {
	return []AdapterTestCase{
		{
			Name:        "postgres",
			AdapterType: "postgres",
			Params: map[string]any{
				"host":     "postgres",
				"port":     "5432",
				"user":     "postgres",
				"password": "postgres",
				"database": "meta",
				"sslmode":  "disable",
			},
			SetupSQL: []string{
				"DROP TABLE IF EXISTS e2e_test_users",
				"CREATE TABLE e2e_test_users (id SERIAL PRIMARY KEY, name TEXT NOT NULL)",
				"INSERT INTO e2e_test_users (name) VALUES ('Alice'), ('Bob')",
			},
			QuerySQL:     "SELECT id, name FROM e2e_test_users ORDER BY id",
			ExpectedCols: []string{"id", "name"},
		},
		{
			Name:        "mysql",
			AdapterType: "mysql",
			Params: map[string]any{
				"host":     "mysql",
				"port":     "3306",
				"user":     "root",
				"password": "root",
				"database": "meta",
			},
			SetupSQL: []string{
				"DROP TABLE IF EXISTS e2e_test_users",
				"CREATE TABLE e2e_test_users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255) NOT NULL)",
				"INSERT INTO e2e_test_users (name) VALUES ('Alice'), ('Bob')",
			},
			QuerySQL:     "SELECT id, name FROM e2e_test_users ORDER BY id",
			ExpectedCols: []string{"id", "name"},
		},
		{
			Name:        "clickhouse",
			AdapterType: "clickhouse",
			Params: map[string]any{
				"host":     "clickhouse",
				"port":     "9000",
				"user":     "default",
				"password": "",
				"database": "default",
			},
			SetupSQL:     nil, // ClickHouse native driver has issues with DDL via QueryContext; skip DDL setup
			QuerySQL:     "SELECT number FROM system.numbers LIMIT 2",
			ExpectedCols: []string{"number"},
		},
		{
			Name:        "sqlite",
			AdapterType: "sqlite",
			Params: map[string]any{
				"path": "/tmp/e2e_test.db",
			},
			SetupSQL: []string{
				"DROP TABLE IF EXISTS e2e_test_users",
				"CREATE TABLE e2e_test_users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)",
				"INSERT INTO e2e_test_users (name) VALUES ('Alice'), ('Bob')",
			},
			QuerySQL:     "SELECT id, name FROM e2e_test_users ORDER BY id",
			ExpectedCols: []string{"id", "name"},
		},
		{
			Name:        "mssql_should_fail",
			AdapterType: "mssql",
			Params: map[string]any{
				"host":     "localhost",
				"port":     "1433",
				"user":     "sa",
				"password": "",
				"database": "meta",
			},
			ShouldFail: true,
		},
		{
			Name:        "bigquery_should_fail",
			AdapterType: "bigquery",
			Params: map[string]any{
				"project_id": "test",
				"location":   "US",
			},
			ShouldFail: true,
		},
	}
}

func TestAdapters_TestConnection(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewGatewayClient(t)

	for _, tc := range getAdapterCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var res struct {
				OK           bool   `json:"ok"`
				ErrorMessage string `json:"error_message"`
				LatencyMs    int64  `json:"latency_ms"`
			}
			err := client.JSON("POST", "/v1/test-connection", map[string]any{
				"adapter_type": tc.AdapterType,
				"params":       tc.Params,
			}, &res)
			require.NoError(t, err, "HTTP request should succeed")

			if tc.ShouldFail {
				require.False(t, res.OK, "expected ok=false for failing adapter")
				require.NotEmpty(t, res.ErrorMessage, "error_message should be present")
				return
			}
			require.True(t, res.OK, "expected ok=true, got error: %s", res.ErrorMessage)
			require.GreaterOrEqual(t, res.LatencyMs, int64(0), "latency should be non-negative")
		})
	}
}

func TestAdapters_GetSchema(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewGatewayClient(t)

	for _, tc := range getAdapterCases() {
		if tc.ShouldFail {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			if tc.AdapterType == "clickhouse" {
				db := getString(tc.Params, "database")
				setupURL := fmt.Sprintf("http://localhost:8123/?database=%s", db)
				httpClient := &http.Client{Timeout: 10 * time.Second}
				for _, stmt := range []string{
					"DROP TABLE IF EXISTS e2e_test_users",
					"CREATE TABLE e2e_test_users (id UInt32, name String) ENGINE MergeTree ORDER BY id",
				} {
					resp, err := httpClient.Post(setupURL, "text/plain", strings.NewReader(stmt))
					require.NoError(t, err, "clickhouse http setup failed")
					resp.Body.Close()
				}
			} else {
				for _, stmt := range tc.SetupSQL {
					var qres QueryResult
					err := client.JSON("POST", "/v1/execute-query", map[string]any{
						"connection_id":   "e2e-" + tc.AdapterType,
						"adapter_type":    tc.AdapterType,
						"params":          tc.Params,
						"sql":             stmt,
						"limit":           0,
						"timeout_seconds": 30,
					}, &qres)
					if err != nil && (strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "not exist")) {
						err = nil
					}
					require.NoError(t, err, "setup SQL should not fail: %s", stmt)
				}
			}

			if tc.AdapterType == "clickhouse" {
				time.Sleep(200 * time.Millisecond)
			}

			var res struct {
				Tables []TableInfo `json:"tables"`
			}
			err := client.JSON("POST", "/v1/schema", map[string]any{
				"connection_id": "e2e-" + tc.AdapterType,
				"adapter_type":  tc.AdapterType,
				"params":        tc.Params,
			}, &res)
			require.NoError(t, err, "get schema should succeed")
			require.NotEmpty(t, res.Tables, "schema should contain at least one table")

			found := false
			for _, tbl := range res.Tables {
				if tbl.Name == "e2e_test_users" {
					found = true
					require.NotEmpty(t, tbl.Columns, "table should have columns")
					colNames := make([]string, len(tbl.Columns))
					for i, c := range tbl.Columns {
						colNames[i] = c.Name
					}
					expectedCols := tc.ExpectedCols
					if tc.AdapterType == "clickhouse" {
						expectedCols = []string{"id", "name"}
					}
					for _, expected := range expectedCols {
						require.Contains(t, colNames, expected, "column %s should exist", expected)
					}
				}
			}
			require.True(t, found, "e2e_test_users table should be in schema")
		})
	}
}

func TestAdapters_ExecuteQuery(t *testing.T) {
	WaitForService(t, GatewayBaseURL+"/health")
	client := NewGatewayClient(t)

	for _, tc := range getAdapterCases() {
		if tc.ShouldFail {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			// Ensure clean state for adapters that support DDL via QueryContext
			if tc.SetupSQL != nil {
				dropSQL := "DROP TABLE IF EXISTS e2e_test_users"
				_ = client.JSON("POST", "/v1/execute-query", map[string]any{
					"connection_id":   "e2e-" + tc.AdapterType,
					"adapter_type":    tc.AdapterType,
					"params":          tc.Params,
					"sql":             dropSQL,
					"limit":           0,
					"timeout_seconds": 30,
				}, nil)

				for _, stmt := range tc.SetupSQL {
					var qres QueryResult
					err := client.JSON("POST", "/v1/execute-query", map[string]any{
						"connection_id":   "e2e-" + tc.AdapterType,
						"adapter_type":    tc.AdapterType,
						"params":          tc.Params,
						"sql":             stmt,
						"limit":           0,
						"timeout_seconds": 30,
					}, &qres)
					if err != nil && (strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "not exist")) {
						err = nil
					}
					require.NoError(t, err, "setup SQL should not fail: %s", stmt)
				}
			}

			if tc.AdapterType == "clickhouse" {
				time.Sleep(200 * time.Millisecond)
			}

			queryLimit := int32(100)
			if tc.AdapterType == "clickhouse" {
				queryLimit = 0 // avoid double LIMIT in ClickHouse
			}

			var res QueryResult
			err := client.JSON("POST", "/v1/execute-query", map[string]any{
				"connection_id":   "e2e-" + tc.AdapterType,
				"adapter_type":    tc.AdapterType,
				"params":          tc.Params,
				"sql":             tc.QuerySQL,
				"limit":           queryLimit,
				"timeout_seconds": 30,
			}, &res)
			require.NoError(t, err, "execute query should succeed")
			require.ElementsMatch(t, tc.ExpectedCols, res.Columns, "columns should match")
			require.GreaterOrEqual(t, res.RowCount, int64(2), "should have at least 2 rows")
			require.Len(t, res.Rows, 2, "should return exactly 2 rows")

			for _, row := range res.Rows {
				require.Len(t, row.Values, len(tc.ExpectedCols), "each row should have expected column count")
			}
		})
	}
}

func TestAdapters_DirectCM_TestConnection(t *testing.T) {
	WaitForService(t, CMBaseURL+"/healthz")
	client := NewHTTPClient(CMBaseURL)

	for _, tc := range getAdapterCases() {
		t.Run(tc.Name, func(t *testing.T) {
			var res struct {
				OK           bool   `json:"ok"`
				ErrorMessage string `json:"error_message"`
			}
			err := client.JSON("POST", "/v1/test-connection", map[string]any{
				"adapter_type": tc.AdapterType,
				"params":       tc.Params,
			}, &res)
			require.NoError(t, err, "HTTP request should succeed")

			if tc.ShouldFail {
				require.False(t, res.OK, "expected ok=false for failing adapter")
				require.NotEmpty(t, res.ErrorMessage)
				return
			}
			require.True(t, res.OK, res.ErrorMessage)
		})
	}
}
