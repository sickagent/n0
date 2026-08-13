// Package sandbox parses and rewrites the safe analytical SQL subset accepted
// by Query Engine. Unsupported syntax is rejected rather than guessed.
package sandbox

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const DefaultRowLimit = 10000

// Policy is trusted connection-side authorization metadata.
type Policy struct {
	AllowedTables []string
	TenantColumn  string
	TenantValue   string
}

// Result contains the parsed statement and safe SQL rewrite.
type Result struct {
	Allowed   bool
	Sanitized string
	Reason    string
	Tables    []string
}

type token struct {
	value string
	lower string
	start int
	end   int
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// Validate preserves the original API while enforcing a table allowlist.
func Validate(sql string, allowedTables []string) Result {
	return ValidatePolicy(sql, Policy{AllowedTables: allowedTables})
}

// ValidatePolicy parses a conservative SELECT AST, enforces referenced tables,
// injects a tenant predicate, and caps the result size.
func ValidatePolicy(sql string, policy Policy) Result {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return reject("empty query")
	}
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	tokens, err := lex(trimmed)
	if err != nil {
		return reject(err.Error())
	}
	if len(tokens) == 0 || tokens[0].lower != "select" {
		if len(tokens) > 0 && isForbidden(tokens[0].lower) {
			return reject("forbidden SQL construct: " + tokens[0].value)
		}
		return reject("query must start with SELECT")
	}
	forbidden := map[string]bool{
		"insert": true, "update": true, "delete": true, "merge": true,
		"create": true, "alter": true, "drop": true, "truncate": true,
		"grant": true, "revoke": true, "copy": true, "call": true,
		"execute": true, "into": true, "union": true, "intersect": true,
		"except":   true,
		"pg_sleep": true, "sleep": true, "benchmark": true, "load_file": true,
	}
	depth := 0
	for _, tok := range tokens {
		switch tok.value {
		case "(":
			depth++
		case ")":
			depth--
		case ";":
			return reject("multiple SQL statements are not allowed")
		}
		if depth < 0 {
			return reject("unbalanced parentheses")
		}
		if tok.lower == "select" && depth > 0 {
			return reject("subqueries are not allowed")
		}
		if forbidden[tok.lower] {
			return reject("forbidden SQL construct: " + tok.value)
		}
		if tok.lower == "for" {
			return reject("locking SELECT is not allowed")
		}
	}
	if depth != 0 {
		return reject("unbalanced parentheses")
	}

	tables, aliases, err := extractTables(tokens)
	if err != nil {
		return reject(err.Error())
	}
	if err := enforceAllowlist(tables, policy.AllowedTables); err != nil {
		return reject(err.Error())
	}

	sanitized := trimmed
	if policy.TenantColumn != "" {
		if !identifier.MatchString(policy.TenantColumn) {
			return reject("invalid tenant column policy")
		}
		if policy.TenantValue == "" {
			return reject("tenant value is required by policy")
		}
		if len(tables) != 1 {
			return reject("tenant predicate injection requires exactly one base table")
		}
		qualifier := aliases[0]
		if qualifier == "" {
			qualifier = tables[0]
			if idx := strings.LastIndex(qualifier, "."); idx >= 0 {
				qualifier = qualifier[idx+1:]
			}
		}
		predicate := quoteIdentifier(qualifier) + "." + quoteIdentifier(policy.TenantColumn) + " = " + quoteLiteral(policy.TenantValue)
		sanitized = injectPredicate(sanitized, tokens, predicate)
	}

	limit, hasLimit, err := parseLimit(tokens)
	if err != nil {
		return reject(err.Error())
	}
	if hasLimit && (limit < 0 || limit > DefaultRowLimit) {
		return reject(fmt.Sprintf("LIMIT must be between 0 and %d", DefaultRowLimit))
	}
	if !hasLimit {
		sanitized += fmt.Sprintf(" LIMIT %d", DefaultRowLimit)
	}
	return Result{Allowed: true, Sanitized: sanitized, Tables: tables}
}

func lex(sql string) ([]token, error) {
	var out []token
	for i := 0; i < len(sql); {
		if unicode.IsSpace(rune(sql[i])) {
			i++
			continue
		}
		start := i
		if sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			return nil, fmt.Errorf("SQL comments are not allowed")
		}
		if sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			return nil, fmt.Errorf("SQL comments are not allowed")
		}
		if sql[i] == '\'' {
			i++
			for i < len(sql) {
				if sql[i] == '\'' {
					if i+1 < len(sql) && sql[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			if i > len(sql) || sql[i-1] != '\'' {
				return nil, fmt.Errorf("unterminated string literal")
			}
			out = append(out, token{value: sql[start:i], lower: sql[start:i], start: start, end: i})
			continue
		}
		if sql[i] == '"' || sql[i] == '`' {
			quote := sql[i]
			i++
			for i < len(sql) && sql[i] != quote {
				i++
			}
			if i >= len(sql) {
				return nil, fmt.Errorf("unterminated identifier")
			}
			i++
			value := sql[start:i]
			out = append(out, token{value: value, lower: strings.ToLower(strings.Trim(value, "\"`")), start: start, end: i})
			continue
		}
		if strings.ContainsRune("(),.;", rune(sql[i])) {
			i++
			value := sql[start:i]
			out = append(out, token{value: value, lower: value, start: start, end: i})
			continue
		}
		for i < len(sql) && !unicode.IsSpace(rune(sql[i])) && !strings.ContainsRune("(),.;'\"`", rune(sql[i])) {
			i++
		}
		value := sql[start:i]
		out = append(out, token{value: value, lower: strings.ToLower(value), start: start, end: i})
	}
	return out, nil
}

func extractTables(tokens []token) ([]string, []string, error) {
	var tables, aliases []string
	reserved := map[string]bool{"where": true, "join": true, "left": true, "right": true, "inner": true, "outer": true, "full": true, "cross": true, "on": true, "group": true, "order": true, "having": true, "limit": true, "offset": true}
	for i := 0; i < len(tokens); i++ {
		if tokens[i].lower != "from" && tokens[i].lower != "join" {
			continue
		}
		i++
		if i >= len(tokens) || tokens[i].value == "(" {
			return nil, nil, fmt.Errorf("derived tables and subqueries are not allowed")
		}
		parts := []string{strings.Trim(tokens[i].value, "\"`")}
		for i+2 < len(tokens) && tokens[i+1].value == "." {
			parts = append(parts, strings.Trim(tokens[i+2].value, "\"`"))
			i += 2
		}
		table := strings.ToLower(strings.Join(parts, "."))
		alias := ""
		if i+1 < len(tokens) && tokens[i+1].value == "(" {
			return nil, nil, fmt.Errorf("table functions are not allowed")
		}
		if i+1 < len(tokens) && tokens[i+1].lower == "as" {
			if i+2 >= len(tokens) {
				return nil, nil, fmt.Errorf("missing table alias")
			}
			alias = strings.Trim(tokens[i+2].value, "\"`")
			i += 2
		} else if i+1 < len(tokens) && identifier.MatchString(tokens[i+1].value) && !reserved[tokens[i+1].lower] {
			alias = tokens[i+1].value
			i++
		}
		tables, aliases = append(tables, table), append(aliases, alias)
		if i+1 < len(tokens) && tokens[i+1].value == "," {
			return nil, nil, fmt.Errorf("comma-separated tables are not allowed; use explicit JOIN")
		}
	}
	if len(tables) == 0 {
		return nil, nil, fmt.Errorf("SELECT must reference a base table")
	}
	return tables, aliases, nil
}

func enforceAllowlist(tables, allowed []string) error {
	if len(allowed) == 0 {
		return fmt.Errorf("table allowlist is required")
	}
	set := make(map[string]bool, len(allowed))
	for _, table := range allowed {
		set[strings.ToLower(strings.TrimSpace(table))] = true
	}
	for _, table := range tables {
		short := table
		if idx := strings.LastIndex(table, "."); idx >= 0 {
			short = table[idx+1:]
		}
		if !set[table] && !set[short] {
			return fmt.Errorf("table %q is not allowed", table)
		}
	}
	return nil
}

func parseLimit(tokens []token) (int, bool, error) {
	for i, tok := range tokens {
		if tok.lower == "limit" {
			if i+1 >= len(tokens) {
				return 0, true, fmt.Errorf("LIMIT must be numeric")
			}
			n, err := strconv.Atoi(tokens[i+1].value)
			if err != nil {
				return 0, true, fmt.Errorf("LIMIT must be numeric")
			}
			return n, true, nil
		}
	}
	return 0, false, nil
}

func injectPredicate(sql string, tokens []token, predicate string) string {
	insertAt := len(sql)
	whereStart := -1
	whereEnd := -1
	depth := 0
	for _, tok := range tokens {
		switch tok.value {
		case "(":
			depth++
		case ")":
			depth--
		}
		if depth != 0 {
			continue
		}
		if tok.lower == "where" {
			whereStart, whereEnd = tok.start, tok.end
		}
		if tok.lower == "group" || tok.lower == "order" || tok.lower == "having" || tok.lower == "limit" || tok.lower == "offset" {
			insertAt = tok.start
			break
		}
	}
	if whereStart >= 0 {
		prefix := strings.TrimSpace(sql[:whereEnd])
		condition := strings.TrimSpace(sql[whereEnd:insertAt])
		suffix := strings.TrimSpace(sql[insertAt:])
		result := prefix + " (" + condition + ") AND " + predicate
		if suffix != "" {
			result += " " + suffix
		}
		return result
	}
	prefix := strings.TrimSpace(sql[:insertAt]) + " WHERE " + predicate
	suffix := strings.TrimSpace(sql[insertAt:])
	if suffix != "" {
		prefix += " " + suffix
	}
	return prefix
}

func quoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }
func quoteLiteral(value string) string    { return `'` + strings.ReplaceAll(value, `'`, `''`) + `'` }
func reject(reason string) Result         { return Result{Reason: reason} }

func isForbidden(keyword string) bool {
	switch keyword {
	case "insert", "update", "delete", "merge", "create", "alter", "drop", "truncate", "grant", "revoke", "copy", "call", "execute":
		return true
	default:
		return false
	}
}

func SortedTables(result Result) []string {
	out := append([]string(nil), result.Tables...)
	sort.Strings(out)
	return out
}
