package migrations_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEveryTableHasUUIDPrimaryID(t *testing.T) {
	files, err := filepath.Glob("*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	combined := ""
	for _, name := range files {
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		combined += "\n" + string(data)
	}

	tablePattern := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+([a-z_]+)\s*\((.*?)\);`)
	idPattern := regexp.MustCompile(`(?i)\bid\s+UUID\s+PRIMARY\s+KEY(?:\s+DEFAULT\s+gen_random_uuid\(\))?`)
	for _, match := range tablePattern.FindAllStringSubmatch(combined, -1) {
		name, body := strings.ToLower(match[1]), match[2]
		if name == "tenant_plugins" {
			// The historical join table is upgraded in migration 000004.
			continue
		}
		if !idPattern.MatchString(body) {
			t.Errorf("table %s must declare id UUID PRIMARY KEY", name)
		}
	}

	if !strings.Contains(combined, "ALTER TABLE tenant_plugins ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid()") {
		t.Error("tenant_plugins must be upgraded to a UUID primary ID")
	}
}
