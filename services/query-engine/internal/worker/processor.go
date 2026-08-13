package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sickagent/n0/pkg/shared/audit"
	pb "github.com/sickagent/n0/proto/gen/go/n0/platform/v1"
	jobpkg "github.com/sickagent/n0/services/query-engine/internal/job"
	"github.com/sickagent/n0/services/query-engine/internal/sandbox"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

type queryExecutor interface {
	ExecuteQuery(ctx context.Context, req *pb.ExecuteQueryRequest) (*pb.ExecuteQueryResponse, error)
}

type connectionLookup interface {
	GetConnection(ctx context.Context, req *pb.GetConnectionRequest) (*pb.GetConnectionResponse, error)
}

// QueryProcessor executes jobs via Connection Manager after sandbox validation.
type QueryProcessor struct {
	log   *zap.Logger
	cm    queryExecutor
	meta  connectionLookup
	store *jobpkg.Store
	audit audit.Publisher
}

// NewQueryProcessor creates a new query processor.
func NewQueryProcessor(log *zap.Logger, cm queryExecutor, meta connectionLookup, store *jobpkg.Store, publisher ...audit.Publisher) *QueryProcessor {
	var auditPublisher audit.Publisher
	if len(publisher) > 0 {
		auditPublisher = publisher[0]
	}
	return &QueryProcessor{
		log:   log,
		cm:    cm,
		meta:  meta,
		store: store,
		audit: auditPublisher,
	}
}

// Process implements the Processor interface.
func (p *QueryProcessor) Process(ctx context.Context, job Job) (processErr error) {
	startedAt := time.Now()
	auditSuccess := false
	auditError := ""
	eventID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("n0/query-job/"+job.ID)).String()
	defer func() {
		if p.audit == nil {
			return
		}
		if err := audit.Publish(p.audit, audit.Event{
			ID: eventID, TenantID: job.TenantID, ActorID: job.TenantID, Action: "query.execute",
			ResourceType: "query_job", ResourceID: job.ID, Success: auditSuccess,
			ErrorMessage: auditError, Metadata: map[string]any{
				"connection_id": job.ConnectionID,
				"duration_ms":   time.Since(startedAt).Milliseconds(),
			},
		}); err != nil {
			p.log.Error("audit publish failed", zap.String("job_id", job.ID), zap.Error(err))
			if processErr == nil {
				processErr = fmt.Errorf("publish durable audit event: %w", err)
			}
		}
	}()

	// A JetStream redelivery caused only by a transient audit publish failure
	// must not execute a terminal query twice. Re-publish the same deterministic
	// audit event; the PostgreSQL sink treats the event ID idempotently.
	if existing, err := p.store.Get(job.ID); err == nil {
		switch existing.Status {
		case jobpkg.StatusSuccess:
			auditSuccess = true
			return nil
		case jobpkg.StatusFailed:
			auditError = existing.ErrorMessage
			return nil
		}
	}
	p.log.Info("processing job",
		zap.String("job_id", job.ID),
		zap.String("connection_id", job.ConnectionID),
	)

	if err := p.store.MarkRunning(job.ID); err != nil {
		auditError = err.Error()
		return fmt.Errorf("mark job running: %w", err)
	}

	connResp, err := p.meta.GetConnection(ctx, &pb.GetConnectionRequest{
		ConnectionId: job.ConnectionID,
		TenantId:     job.TenantID,
	})
	if err != nil {
		auditError = "get connection: " + err.Error()
		return p.failJob(job.ID, fmt.Sprintf("get connection: %v", err))
	}
	if connResp.Connection == nil {
		auditError = "connection not found"
		return p.failJob(job.ID, "connection not found")
	}
	params := connResp.Connection.Params.AsMap()
	policy, err := policyFromConnection(params, job.TenantID)
	if err != nil {
		auditError = "invalid query policy: " + err.Error()
		return p.failJob(job.ID, "invalid query policy: "+err.Error())
	}
	res := sandbox.ValidatePolicy(job.SQL, policy)
	if !res.Allowed {
		auditError = "sandbox rejection: " + res.Reason
		return p.failJob(job.ID, "sandbox rejection: "+res.Reason)
	}
	delete(params, "query_policy")
	cleanParams, err := structpb.NewStruct(params)
	if err != nil {
		auditError = "sanitize connection params: " + err.Error()
		return p.failJob(job.ID, "sanitize connection params: "+err.Error())
	}

	resp, err := p.cm.ExecuteQuery(ctx, &pb.ExecuteQueryRequest{
		ConnectionId:   job.ConnectionID,
		Sql:            res.Sanitized,
		Limit:          0,
		TimeoutSeconds: 60,
		AdapterType:    connResp.Connection.AdapterType,
		Params:         cleanParams,
	})
	if err != nil {
		auditError = "execute query: " + err.Error()
		return p.failJob(job.ID, fmt.Sprintf("execute query: %v", err))
	}

	rows := make([]map[string]any, 0, len(resp.Rows))
	for _, r := range resp.Rows {
		row := make(map[string]any, len(resp.Columns))
		for i, col := range resp.Columns {
			if i < len(r.Values) {
				row[col] = r.Values[i].AsInterface()
			} else {
				row[col] = nil
			}
		}
		rows = append(rows, row)
	}

	if err := p.store.MarkSucceeded(job.ID, rows, resp.Truncated); err != nil {
		auditError = err.Error()
		if failErr := p.store.MarkFailed(job.ID, "result persistence: "+err.Error()); failErr != nil {
			return fmt.Errorf("persist result: %v; mark job failed: %w", err, failErr)
		}
		return fmt.Errorf("mark job success: %w", err)
	}

	p.log.Info("job executed",
		zap.String("job_id", job.ID),
		zap.Int64("rows", resp.RowCount),
	)
	auditSuccess = true
	return nil
}

func policyFromConnection(params map[string]any, tenantID string) (sandbox.Policy, error) {
	raw, ok := params["query_policy"].(map[string]any)
	if !ok {
		return sandbox.Policy{}, fmt.Errorf("query_policy is required")
	}
	values, ok := raw["allowed_tables"].([]any)
	if !ok || len(values) == 0 {
		return sandbox.Policy{}, fmt.Errorf("allowed_tables is required")
	}
	allowed := make([]string, 0, len(values))
	for _, value := range values {
		table, ok := value.(string)
		if !ok || table == "" {
			return sandbox.Policy{}, fmt.Errorf("allowed_tables must contain strings")
		}
		allowed = append(allowed, table)
	}
	tenantColumn, _ := raw["tenant_column"].(string)
	return sandbox.Policy{AllowedTables: allowed, TenantColumn: tenantColumn, TenantValue: tenantID}, nil
}

func (p *QueryProcessor) failJob(jobID, message string) error {
	p.log.Warn("query job failed", zap.String("job_id", jobID), zap.String("reason", message))
	if err := p.store.MarkFailed(jobID, message); err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	return nil
}
