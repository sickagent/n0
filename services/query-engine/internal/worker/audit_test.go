package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sickagent/n0/pkg/shared/audit"
	"github.com/sickagent/n0/services/query-engine/internal/job"
	"go.uber.org/zap"
)

type auditRecorder struct {
	subject string
	event   audit.Event
}

func (r *auditRecorder) Publish(subject string, data []byte) error {
	r.subject = subject
	return json.Unmarshal(data, &r.event)
}

func TestQueryProcessorPublishesAuditEvent(t *testing.T) {
	store := job.NewStore()
	store.Create(job.Record{ID: "audit-job", ConnectionID: "conn-1", TenantID: "tenant-1", SQL: "SELECT value FROM users"})
	recorder := &auditRecorder{}
	proc := NewQueryProcessor(zap.NewNop(), &fakeExecutor{}, &fakeLookup{}, store, recorder)
	request := Job{ID: "audit-job", ConnectionID: "conn-1", TenantID: "tenant-1", SQL: "SELECT value FROM users"}
	if err := proc.Process(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if recorder.subject != "audit.events.tenant-1" || !recorder.event.Success || recorder.event.ResourceID != "audit-job" {
		t.Fatalf("unexpected audit event: %s %+v", recorder.subject, recorder.event)
	}
}
