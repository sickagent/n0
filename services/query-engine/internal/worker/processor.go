package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/query-engine/internal/job"
	"n0/services/query-engine/internal/sandbox"
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
	store *job.Store
}

// NewQueryProcessor creates a new query processor.
func NewQueryProcessor(log *zap.Logger, cm queryExecutor, meta connectionLookup, store *job.Store) *QueryProcessor {
	return &QueryProcessor{
		log:   log,
		cm:    cm,
		meta:  meta,
		store: store,
	}
}

// Process implements the Processor interface.
func (p *QueryProcessor) Process(ctx context.Context, job Job) error {
	p.log.Info("processing job",
		zap.String("job_id", job.ID),
		zap.String("connection_id", job.ConnectionID),
	)

	if err := p.store.MarkRunning(job.ID); err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}

	res := sandbox.Validate(job.SQL, nil)
	if !res.Allowed {
		return p.failJob(job.ID, "sandbox rejection: "+res.Reason)
	}

	connResp, err := p.meta.GetConnection(ctx, &pb.GetConnectionRequest{
		ConnectionId: job.ConnectionID,
		TenantId:     job.TenantID,
	})
	if err != nil {
		return p.failJob(job.ID, fmt.Sprintf("get connection: %v", err))
	}
	if connResp.Connection == nil {
		return p.failJob(job.ID, "connection not found")
	}

	resp, err := p.cm.ExecuteQuery(ctx, &pb.ExecuteQueryRequest{
		ConnectionId:   job.ConnectionID,
		Sql:            res.Sanitized,
		Limit:          0,
		TimeoutSeconds: 60,
		AdapterType:    connResp.Connection.AdapterType,
		Params:         connResp.Connection.Params,
	})
	if err != nil {
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
		return fmt.Errorf("mark job success: %w", err)
	}

	p.log.Info("job executed",
		zap.String("job_id", job.ID),
		zap.Int64("rows", resp.RowCount),
	)
	return nil
}

func (p *QueryProcessor) failJob(jobID, message string) error {
	p.log.Warn("query job failed", zap.String("job_id", jobID), zap.String("reason", message))
	if err := p.store.MarkFailed(jobID, message); err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}
	return nil
}
