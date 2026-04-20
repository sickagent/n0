package server

import (
	"context"
	"encoding/json"
	"testing"

	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/query-engine/internal/job"
	"n0/services/query-engine/internal/worker"

	"go.uber.org/zap"
)

type fakePublisher struct {
	subjects []string
	payloads [][]byte
}

func (p *fakePublisher) Publish(subject string, data []byte) error {
	p.subjects = append(p.subjects, subject)
	p.payloads = append(p.payloads, append([]byte(nil), data...))
	return nil
}

func TestGRPCServer_SubmitQueryAndReadResult(t *testing.T) {
	store := job.NewStore()
	pub := &fakePublisher{}
	svc := NewGRPCServer(zap.NewNop(), store, pub)

	submitResp, err := svc.SubmitQuery(context.Background(), &pb.SubmitQueryRequest{
		TenantId:     "tenant-1",
		ConnectionId: "conn-1",
		Sql:          "SELECT 1",
	})
	if err != nil {
		t.Fatalf("submit query: %v", err)
	}
	if submitResp.Status != job.StatusPending {
		t.Fatalf("expected pending status, got %s", submitResp.Status)
	}
	if len(pub.subjects) != 1 || pub.subjects[0] != querySubject {
		t.Fatalf("expected publish to %s, got %+v", querySubject, pub.subjects)
	}

	var payload worker.Job
	if err := json.Unmarshal(pub.payloads[0], &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ID != submitResp.JobId {
		t.Fatalf("expected job id %s in payload, got %s", submitResp.JobId, payload.ID)
	}

	statusResp, err := svc.GetJobStatus(context.Background(), &pb.GetJobStatusRequest{JobId: submitResp.JobId})
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if statusResp.Status != job.StatusPending {
		t.Fatalf("expected pending status, got %s", statusResp.Status)
	}

	if err := store.MarkSucceeded(submitResp.JobId, []map[string]any{
		{"value": "1"},
		{"value": "2"},
	}, false); err != nil {
		t.Fatalf("mark success: %v", err)
	}

	resultResp, err := svc.GetJobResult(context.Background(), &pb.GetJobResultRequest{
		JobId:    submitResp.JobId,
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	if len(resultResp.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(resultResp.Rows))
	}
	if resultResp.NextPageToken != "2" {
		t.Fatalf("expected next page token 2, got %q", resultResp.NextPageToken)
	}
}
