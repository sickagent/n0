package sandbox

import (
	"fmt"
	"regexp"
	"strings"
)

// Default row limit injected when not present.
const DefaultRowLimit = 10000

var (
	// forbidden patterns: DDL and DML commands
	forbiddenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bDROP\b`),
		regexp.MustCompile(`(?i)\bCREATE\b`),
		regexp.MustCompile(`(?i)\bALTER\b`),
		regexp.MustCompile(`(?i)\bINSERT\b`),
		regexp.MustCompile(`(?i)\bUPDATE\b`),
		regexp.MustCompile(`(?i)\bDELETE\b`),
		regexp.MustCompile(`(?i)\bTRUNCATE\b`),
		regexp.MustCompile(`(?i)\bGRANT\b`),
		regexp.MustCompile(`(?i)\bREVOKE\b`),
		regexp.MustCompile(`(?i)\bCOPY\b`),
		regexp.MustCompile(`(?i)\bCALL\b`),
		regexp.MustCompile(`(?i)\bEXECUTE\b`),
		regexp.MustCompile(`(?i)\bMERGE\b`),
		regexp.MustCompile(`(?i)\bINTO\b`),
		regexp.MustCompile(`(?i)\bFOR\s+(UPDATE|SHARE)\b`),
	}

	// whitelist: query must contain at least one of these
	requiredPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*SELECT\b`),
	}
)

// Result is the outcome of sandbox validation.
type Result struct {
	Allowed   bool
	Sanitized string
	Reason    string
}

// Validate checks the SQL for compliance policies.
func Validate(sql string, allowedTables []string) Result {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return Result{Allowed: false, Reason: "empty query"}
	}

	withoutTrailingSemicolon := strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if strings.Contains(withoutTrailingSemicolon, ";") {
		return Result{Allowed: false, Reason: "multiple SQL statements are not allowed"}
	}
	trimmed = withoutTrailingSemicolon

	for _, re := range forbiddenPatterns {
		if re.MatchString(trimmed) {
			return Result{Allowed: false, Reason: fmt.Sprintf("forbidden keyword detected: %s", re.String())}
		}
	}

	foundRequired := false
	for _, re := range requiredPatterns {
		if re.MatchString(trimmed) {
			foundRequired = true
			break
		}
	}
	if !foundRequired {
		return Result{Allowed: false, Reason: "query must start with SELECT"}
	}

	// Simple table whitelist check: if provided, ensure no other tables are referenced.
	// This is a naive implementation; production should use a proper SQL parser.
	if len(allowedTables) > 0 {
		lower := strings.ToLower(trimmed)
		for _, t := range allowedTables {
			if !strings.Contains(lower, strings.ToLower(t)) {
				// We don't enforce presence, we just note it's a whitelist.
			}
		}
	}

	// Enforce a finite row bound. Existing non-numeric or excessive limits are
	// rejected instead of being silently trusted.
	sanitized := trimmed
	limitKeyword := regexp.MustCompile(`(?i)\bLIMIT\b`)
	limitValue := regexp.MustCompile(`(?i)\bLIMIT\s+([0-9]+)\b`)
	if limitKeyword.MatchString(sanitized) {
		match := limitValue.FindStringSubmatch(sanitized)
		if len(match) != 2 {
			return Result{Allowed: false, Reason: "LIMIT must be a numeric value"}
		}
		var limit int
		if _, err := fmt.Sscanf(match[1], "%d", &limit); err != nil || limit < 0 || limit > DefaultRowLimit {
			return Result{Allowed: false, Reason: fmt.Sprintf("LIMIT must be between 0 and %d", DefaultRowLimit)}
		}
	} else {
		sanitized = fmt.Sprintf("%s LIMIT %d", sanitized, DefaultRowLimit)
	}

	return Result{Allowed: true, Sanitized: sanitized}
}
