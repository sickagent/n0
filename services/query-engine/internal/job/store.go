package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
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
	ResultRef    string
}

// DurableConfig configures Redis metadata and S3-compatible large results.
type DurableConfig struct {
	RedisMode      string
	RedisAddr      string
	RedisAddrs     []string
	RedisUsername  string
	RedisPassword  string
	RedisDB        int
	TTL            time.Duration
	ObjectClient   *minio.Client
	ObjectBucket   string
	InlineMaxBytes int
}

// Store keeps query jobs in memory for the lifetime of the process.
type Store struct {
	mu        sync.RWMutex
	jobs      map[string]*Record
	redis     redis.UniversalClient
	ttl       time.Duration
	objects   *minio.Client
	bucket    string
	inlineMax int
}

// NewStore creates an empty job store.
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*Record),
	}
}

// NewDurableStore creates a Redis-backed store. Large results are moved to
// S3-compatible object storage when an object client is configured.
func NewDurableStore(ctx context.Context, cfg DurableConfig) (*Store, error) {
	client, err := newRedisClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	if cfg.InlineMaxBytes <= 0 {
		cfg.InlineMaxBytes = 1 << 20
	}
	if cfg.ObjectClient != nil && cfg.ObjectBucket == "" {
		_ = client.Close()
		return nil, fmt.Errorf("object bucket is required")
	}
	return &Store{redis: client, ttl: cfg.TTL, objects: cfg.ObjectClient, bucket: cfg.ObjectBucket, inlineMax: cfg.InlineMaxBytes}, nil
}

func newRedisClient(cfg DurableConfig) (redis.UniversalClient, error) {
	addrs := make([]string, 0, len(cfg.RedisAddrs)+1)
	for _, addr := range cfg.RedisAddrs {
		if addr = strings.TrimSpace(addr); addr != "" {
			addrs = append(addrs, addr)
		}
	}
	if len(addrs) == 0 && strings.TrimSpace(cfg.RedisAddr) != "" {
		addrs = append(addrs, strings.TrimSpace(cfg.RedisAddr))
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("redis address is required")
	}

	switch strings.ToLower(strings.TrimSpace(cfg.RedisMode)) {
	case "", "standalone", "single":
		if len(addrs) != 1 {
			return nil, fmt.Errorf("standalone redis requires exactly one address")
		}
		return redis.NewClient(&redis.Options{
			Addr: addrs[0], Username: cfg.RedisUsername, Password: cfg.RedisPassword, DB: cfg.RedisDB,
		}), nil
	case "cluster":
		if cfg.RedisDB != 0 {
			return nil, fmt.Errorf("redis cluster does not support redis_db other than 0")
		}
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: addrs, Username: cfg.RedisUsername, Password: cfg.RedisPassword,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported redis mode %q (expected standalone or cluster)", cfg.RedisMode)
	}
}

// Close releases external clients.
func (s *Store) Close() error {
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

// Create registers a new job in pending state.
func (s *Store) Create(record Record) Record {
	created, _ := s.CreateContext(context.Background(), record)
	return created
}

// CreateContext registers a job and reports durable persistence errors.
func (s *Store) CreateContext(ctx context.Context, record Record) (Record, error) {
	now := time.Now().UTC()
	record.Status = StatusPending
	record.ErrorMessage = ""
	record.Rows = cloneRows(record.Rows)
	record.CreatedAt = now
	record.UpdatedAt = now

	cp := record
	if s.redis != nil {
		if err := s.save(ctx, cp); err != nil {
			return Record{}, err
		}
	} else {
		s.mu.Lock()
		s.jobs[record.ID] = &cp
		s.mu.Unlock()
	}
	return cloneRecord(cp), nil
}

// Get returns a copy of the current job state.
func (s *Store) Get(id string) (Record, error) {
	if s.redis != nil {
		return s.load(context.Background(), id)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.jobs[id]
	if !ok {
		return Record{}, ErrJobNotFound
	}
	return cloneRecord(*rec), nil
}

// GetForTenant returns a job only when it belongs to the requested tenant.
// A mismatch is intentionally indistinguishable from a missing job.
func (s *Store) GetForTenant(id, tenantID string) (Record, error) {
	record, err := s.Get(id)
	if err != nil || record.TenantID != tenantID {
		return Record{}, ErrJobNotFound
	}
	return record, nil
}

// MarkRunning moves a job into running state.
func (s *Store) MarkRunning(id string) error {
	return s.update(id, func(rec *Record) { rec.Status = StatusRunning; rec.ErrorMessage = "" })
}

// MarkFailed stores a terminal failure for the job.
func (s *Store) MarkFailed(id, errorMessage string) error {
	return s.update(id, func(rec *Record) {
		rec.Status = StatusFailed
		rec.ErrorMessage = errorMessage
		rec.Rows = nil
		rec.ResultRef = ""
	})
}

// MarkSucceeded stores successful rows for the job.
func (s *Store) MarkSucceeded(id string, rows []map[string]any, truncated bool) error {
	rec, err := s.Get(id)
	if err != nil {
		return err
	}
	rec.Status, rec.ErrorMessage, rec.Truncated, rec.UpdatedAt = StatusSuccess, "", truncated, time.Now().UTC()
	rec.Rows = cloneRows(rows)
	if s.redis != nil {
		payload, err := json.Marshal(rows)
		if err != nil {
			return fmt.Errorf("marshal result: %w", err)
		}
		if len(payload) > s.inlineMax {
			if s.objects == nil {
				return fmt.Errorf("result exceeds inline limit and object storage is unavailable")
			}
			key := "results/" + id + ".json"
			_, err = s.objects.PutObject(context.Background(), s.bucket, key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "application/json"})
			if err != nil {
				return fmt.Errorf("store object result: %w", err)
			}
			rec.Rows, rec.ResultRef = nil, key
		}
	}
	if s.redis != nil {
		return s.save(context.Background(), rec)
	}
	s.mu.Lock()
	cp := rec
	s.jobs[id] = &cp
	s.mu.Unlock()
	return nil
}

// GetResultPage returns paginated rows and the next page token if more rows are available.
func (s *Store) GetResultPage(id string, page, pageSize int32) (Record, []map[string]any, string, error) {
	record, err := s.Get(id)
	if err != nil {
		return Record{}, nil, "", err
	}
	if record.ResultRef != "" {
		if s.objects == nil {
			return Record{}, nil, "", fmt.Errorf("object result backend unavailable")
		}
		obj, err := s.objects.GetObject(context.Background(), s.bucket, record.ResultRef, minio.GetObjectOptions{})
		if err != nil {
			return Record{}, nil, "", fmt.Errorf("get object result: %w", err)
		}
		defer obj.Close()
		payload, err := io.ReadAll(obj)
		if err != nil {
			return Record{}, nil, "", fmt.Errorf("read object result: %w", err)
		}
		if err := json.Unmarshal(payload, &record.Rows); err != nil {
			return Record{}, nil, "", fmt.Errorf("decode object result: %w", err)
		}
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
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

func (s *Store) update(id string, mutate func(*Record)) error {
	rec, err := s.Get(id)
	if err != nil {
		return err
	}
	mutate(&rec)
	rec.UpdatedAt = time.Now().UTC()
	if s.redis != nil {
		return s.save(context.Background(), rec)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := rec
	s.jobs[id] = &cp
	return nil
}

func (s *Store) save(ctx context.Context, record Record) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := s.redis.Set(ctx, "n0:job:"+record.ID, payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("save job: %w", err)
	}
	return nil
}

func (s *Store) load(ctx context.Context, id string) (Record, error) {
	payload, err := s.redis.Get(ctx, "n0:job:"+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return Record{}, ErrJobNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("load job: %w", err)
	}
	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return Record{}, fmt.Errorf("decode job: %w", err)
	}
	return record, nil
}

// GetResultPageForTenant returns a result page only to the owning tenant.
func (s *Store) GetResultPageForTenant(id, tenantID string, page, pageSize int32) (Record, []map[string]any, string, error) {
	if _, err := s.GetForTenant(id, tenantID); err != nil {
		return Record{}, nil, "", err
	}
	return s.GetResultPage(id, page, pageSize)
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
