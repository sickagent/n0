package job

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

var ErrJobNotFound = errors.New("job not found")

// Record stores the lifecycle and result of a query job.
type Record struct {
	ID           string
	TenantID     string
	ConnectionID string
	SQL          string
	Status       string
	ErrorMessage string
	Rows         []map[string]any
	Truncated    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store keeps query jobs in memory for the lifetime of the process.
type Store struct {
	mu   sync.RWMutex
	jobs map[string]*Record
}

// NewStore creates an empty job store.
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*Record),
	}
}

// Create registers a new job in pending state.
func (s *Store) Create(record Record) Record {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	record.Status = StatusPending
	record.ErrorMessage = ""
	record.Rows = cloneRows(record.Rows)
	record.CreatedAt = now
	record.UpdatedAt = now

	cp := record
	s.jobs[record.ID] = &cp
	return cloneRecord(cp)
}

// Get returns a copy of the current job state.
func (s *Store) Get(id string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.jobs[id]
	if !ok {
		return Record{}, ErrJobNotFound
	}
	return cloneRecord(*rec), nil
}

// MarkRunning moves a job into running state.
func (s *Store) MarkRunning(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	rec.Status = StatusRunning
	rec.ErrorMessage = ""
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkFailed stores a terminal failure for the job.
func (s *Store) MarkFailed(id, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	rec.Status = StatusFailed
	rec.ErrorMessage = errorMessage
	rec.Rows = nil
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkSucceeded stores successful rows for the job.
func (s *Store) MarkSucceeded(id string, rows []map[string]any, truncated bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.jobs[id]
	if !ok {
		return ErrJobNotFound
	}
	rec.Status = StatusSuccess
	rec.ErrorMessage = ""
	rec.Rows = cloneRows(rows)
	rec.Truncated = truncated
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// GetResultPage returns paginated rows and the next page token if more rows are available.
func (s *Store) GetResultPage(id string, page, pageSize int32) (Record, []map[string]any, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.jobs[id]
	if !ok {
		return Record{}, nil, "", ErrJobNotFound
	}

	record := cloneRecord(*rec)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	start := int((page - 1) * pageSize)
	if start >= len(record.Rows) {
		return record, []map[string]any{}, "", nil
	}

	end := start + int(pageSize)
	if end > len(record.Rows) {
		end = len(record.Rows)
	}

	var nextToken string
	if end < len(record.Rows) {
		nextToken = fmt.Sprintf("%d", page+1)
	}

	return record, cloneRows(record.Rows[start:end]), nextToken, nil
}

func cloneRecord(record Record) Record {
	record.Rows = cloneRows(record.Rows)
	return record
}

func cloneRows(rows []map[string]any) []map[string]any {
	if rows == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cp := make(map[string]any, len(row))
		for k, v := range row {
			cp[k] = v
		}
		out = append(out, cp)
	}
	return out
}
