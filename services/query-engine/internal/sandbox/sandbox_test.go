package sandbox

import (
	"strings"
	"testing"
)

func TestValidate_Allowed(t *testing.T) {
	res := Validate("SELECT * FROM users", []string{"users"})
	if !res.Allowed {
		t.Fatalf("expected allowed, got: %s", res.Reason)
	}
	if !strings.Contains(res.Sanitized, "LIMIT") {
		t.Errorf("expected LIMIT injection, got: %s", res.Sanitized)
	}
}

func TestValidate_Empty(t *testing.T) {
	res := Validate("", nil)
	if res.Allowed {
		t.Fatal("expected not allowed for empty query")
	}
	if res.Reason != "empty query" {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestValidate_ForbiddenKeyword(t *testing.T) {
	for _, kw := range []string{"DROP", "CREATE", "ALTER", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "GRANT", "REVOKE"} {
		res := Validate(kw+" TABLE users", []string{"users"})
		if res.Allowed {
			t.Errorf("expected not allowed for %s", kw)
		}
		if !strings.Contains(res.Reason, "forbidden") {
			t.Errorf("unexpected reason: %s", res.Reason)
		}
	}
}

func TestValidate_MustStartWithSelect(t *testing.T) {
	res := Validate("SHOW TABLES", nil)
	if res.Allowed {
		t.Fatal("expected not allowed for non-SELECT")
	}
	if !strings.Contains(res.Reason, "SELECT") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestValidate_LimitAlreadyPresent(t *testing.T) {
	res := Validate("SELECT * FROM users LIMIT 5", []string{"users"})
	if !res.Allowed {
		t.Fatalf("expected allowed, got: %s", res.Reason)
	}
	if strings.Count(res.Sanitized, "LIMIT") != 1 {
		t.Errorf("expected exactly one LIMIT, got: %s", res.Sanitized)
	}
}

func TestValidateRejectsStackedStatements(t *testing.T) {
	res := Validate("SELECT 1; DROP TABLE users", nil)
	if res.Allowed || !strings.Contains(res.Reason, "multiple") {
		t.Fatalf("expected stacked statement rejection, got %+v", res)
	}
}

func TestValidateAllowsSingleTrailingSemicolon(t *testing.T) {
	res := Validate("SELECT * FROM users;", []string{"users"})
	if !res.Allowed || res.Sanitized != "SELECT * FROM users LIMIT 10000" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestValidateRejectsUnsafeOrUnboundedLimit(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM users LIMIT 10001",
		"SELECT * FROM users LIMIT ALL",
		"SELECT * INTO backup FROM users",
		"SELECT * FROM users FOR UPDATE",
	} {
		if res := Validate(query, []string{"users"}); res.Allowed {
			t.Errorf("expected query to be rejected: %s", query)
		}
	}
}

func TestValidatePolicyEnforcesAllowlistAndTenantPredicate(t *testing.T) {
	policy := Policy{AllowedTables: []string{"public.orders"}, TenantColumn: "tenant_id", TenantValue: "tenant-1"}
	res := ValidatePolicy("SELECT id FROM public.orders WHERE total > 0 ORDER BY id", policy)
	if !res.Allowed {
		t.Fatalf("expected allowed query: %s", res.Reason)
	}
	want := `SELECT id FROM public.orders WHERE (total > 0) AND "orders"."tenant_id" = 'tenant-1' ORDER BY id LIMIT 10000`
	if res.Sanitized != want {
		t.Fatalf("unexpected rewrite:\nwant %s\ngot  %s", want, res.Sanitized)
	}
	if res := ValidatePolicy("SELECT * FROM secrets", policy); res.Allowed {
		t.Fatal("expected table outside allowlist to be rejected")
	}
}

func TestValidatePolicyPreservesBooleanPrecedence(t *testing.T) {
	policy := Policy{AllowedTables: []string{"orders"}, TenantColumn: "tenant_id", TenantValue: "tenant-1"}
	res := ValidatePolicy("SELECT id FROM orders WHERE public = true OR owner = 'x'", policy)
	want := `SELECT id FROM orders WHERE (public = true OR owner = 'x') AND "orders"."tenant_id" = 'tenant-1' LIMIT 10000`
	if !res.Allowed || res.Sanitized != want {
		t.Fatalf("unsafe boolean rewrite: %+v", res)
	}
}

func TestValidateRejectsUnparsedTableSources(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM users, secrets",
		"SELECT * FROM read_csv('/tmp/data')",
		"SELECT * FROM users WHERE id IN (SELECT user_id FROM secrets)",
		"SELECT * FROM users -- LIMIT is commented out",
		"SELECT pg_sleep(60) FROM users",
	} {
		res := Validate(query, []string{"users", "secrets", "read_csv"})
		if res.Allowed {
			t.Fatalf("expected ambiguous table source to be rejected: %s", query)
		}
	}
}
