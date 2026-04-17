package worker

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	pb "n0/proto/gen/go/lensagent/v1"
	"n0/services/query-engine/internal/client"
	"n0/services/query-engine/internal/sandbox"
)

// QueryProcessor executes jobs via Connection Manager after sandbox validation.
type QueryProcessor struct {
	log *zap.Logger
	cm  *client.ConnectionManagerClient
}

// NewQueryProcessor creates a new query processor.
func NewQueryProcessor(log *zap.Logger, cm *client.ConnectionManagerClient) *QueryProcessor {
	return &QueryProcessor{log: log, cm: cm}
}

// Process implements the Processor interface.
func (p *QueryProcessor) Process(ctx context.Context, job Job) error {
	p.log.Info("processing job",
		zap.String("job_id", job.ID),
		zap.String("connection_id", job.ConnectionID),
	)

	res := sandbox.Validate(job.SQL, nil)
	if !res.Allowed {
		p.log.Warn("sandbox rejected query",
			zap.String("job_id", job.ID),
			zap.String("reason", res.Reason),
		)
		return fmt.Errorf("sandbox rejection: %s", res.Reason)
	}

	resp, err := p.cm.ExecuteQuery(ctx, &pb.ExecuteQueryRequest{
		ConnectionId: job.ConnectionID,
		Sql:          res.Sanitized,
		Limit:        sandbox.DefaultRowLimit,
		TimeoutSeconds: 60,
	})
	if err != nil {
		return fmt.Errorf("execute query: %w", err)
	}

	p.log.Info("job executed",
		zap.String("job_id", job.ID),
		zap.Int64("rows", resp.RowCount),
	)
	return nil
}
