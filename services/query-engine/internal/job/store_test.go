package job

import "testing"

func TestStoreLifecycleAndPagination(t *testing.T) {
	store := NewStore()
	record := store.Create(Record{
		ID:           "job-1",
		ConnectionID: "conn-1",
		SQL:          "SELECT 1",
	})

	if record.Status != StatusPending {
		t.Fatalf("expected pending, got %s", record.Status)
	}

	if err := store.MarkRunning(record.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if err := store.MarkSucceeded(record.ID, []map[string]any{
		{"value": 1},
		{"value": 2},
	}, false); err != nil {
		t.Fatalf("mark success: %v", err)
	}

	got, rows, nextToken, err := store.GetResultPage(record.ID, 1, 1)
	if err != nil {
		t.Fatalf("get result page: %v", err)
	}
	if got.Status != StatusSuccess {
		t.Fatalf("expected success, got %s", got.Status)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if nextToken != "2" {
		t.Fatalf("expected next token 2, got %q", nextToken)
	}
}

func TestStoreMarkFailed(t *testing.T) {
	store := NewStore()
	store.Create(Record{ID: "job-fail"})

	if err := store.MarkFailed("job-fail", "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	got, err := store.Get("job-fail")
	if err != nil {
		t.Fatalf("get failed job: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("expected failed, got %s", got.Status)
	}
	if got.ErrorMessage != "boom" {
		t.Fatalf("expected error message boom, got %q", got.ErrorMessage)
	}
}
