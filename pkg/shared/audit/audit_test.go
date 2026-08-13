package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

type recorder struct {
	subject string
	payload []byte
}

func (r *recorder) Publish(subject string, payload []byte) error {
	r.subject = subject
	r.payload = payload
	return nil
}

func TestPublishBuildsTenantSubjectAndIdentity(t *testing.T) {
	r := &recorder{}
	if err := Publish(r, Event{TenantID: "tenant.a", Action: "query.execute", ResourceType: "query_job", Success: true}); err != nil {
		t.Fatal(err)
	}
	if r.subject != "audit.events.tenant_a" {
		t.Fatalf("unexpected subject %q", r.subject)
	}
	var event Event
	if err := json.Unmarshal(r.payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.ID == "" || event.OccurredAt.IsZero() {
		t.Fatalf("identity/timestamp missing: %+v", event)
	}
	if strings.Contains(r.subject, ".a.") {
		t.Fatal("tenant escaped subject partition")
	}
}

func TestPublishRequiresTenant(t *testing.T) {
	if err := Publish(&recorder{}, Event{}); err == nil {
		t.Fatal("expected tenant validation")
	}
}
