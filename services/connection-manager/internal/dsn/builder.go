package dsn

import (
	"fmt"
	"net/url"

	"google.golang.org/protobuf/types/known/structpb"
)

// BuildDSN constructs a DSN from adapter type and params.
func BuildDSN(adapterType string, params *structpb.Struct) (string, error) {
	m := params.AsMap()
	switch adapterType {
	case "postgres":
		return buildPostgresDSN(m)
	case "clickhouse":
		return buildClickHouseDSN(m)
	case "mysql":
		return buildMySQLDSN(m)
	case "sqlite":
		return buildSQLiteDSN(m)
	case "mssql":
		return buildMSSQLDSN(m)
	case "bigquery":
		return buildBigQueryDSN(m)
	default:
		return "", fmt.Errorf("unsupported adapter type: %s", adapterType)
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func buildPostgresDSN(m map[string]any) (string, error) {
	host := getString(m, "host")
	if host == "" {
		host = "localhost"
	}
	port := getString(m, "port")
	if port == "" {
		port = "5432"
	}
	user := getString(m, "user")
	pass := getString(m, "password")
	db := getString(m, "database")
	sslmode := getString(m, "sslmode")
	if sslmode == "" {
		sslmode = "disable"
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   db,
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func buildClickHouseDSN(m map[string]any) (string, error) {
	host := getString(m, "host")
	if host == "" {
		host = "localhost"
	}
	port := getString(m, "port")
	if port == "" {
		port = "9000"
	}
	user := getString(m, "user")
	pass := getString(m, "password")
	db := getString(m, "database")

	u := url.URL{
		Scheme: "clickhouse",
		Host:   fmt.Sprintf("%s:%s", host, port),
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	q := u.Query()
	if db != "" {
		q.Set("database", db)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func buildMySQLDSN(m map[string]any) (string, error) {
	host := getString(m, "host")
	if host == "" {
		host = "localhost"
	}
	port := getString(m, "port")
	if port == "" {
		port = "3306"
	}
	user := getString(m, "user")
	pass := getString(m, "password")
	db := getString(m, "database")

	cfg := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		user, pass, host, port, db)
	return cfg, nil
}

func buildSQLiteDSN(m map[string]any) (string, error) {
	path := getString(m, "path")
	if path == "" {
		path = ":memory:"
	}
	return path, nil
}

func buildMSSQLDSN(m map[string]any) (string, error) {
	host := getString(m, "host")
	if host == "" {
		host = "localhost"
	}
	port := getString(m, "port")
	if port == "" {
		port = "1433"
	}
	user := getString(m, "user")
	pass := getString(m, "password")
	db := getString(m, "database")

	cfg := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
		user, pass, host, port, db)
	return cfg, nil
}

func buildBigQueryDSN(m map[string]any) (string, error) {
	projectID := getString(m, "project_id")
	if projectID == "" {
		return "", fmt.Errorf("project_id is required for bigquery")
	}
	location := getString(m, "location")
	if location == "" {
		location = "US"
	}
	return fmt.Sprintf("bigquery://%s?location=%s", projectID, location), nil
}
