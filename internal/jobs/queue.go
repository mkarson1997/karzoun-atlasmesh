package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type State string

const (
	Queued State = "queued"
	Leased State = "leased"
	Completed State = "completed"
	DeadLetter State = "dead_letter"
)

var (
	ErrNotFound = errors.New("job not found")
	ErrNoWork = errors.New("no work available")
	ErrLeaseLost = errors.New("job lease is not owned or has expired")
	ErrDuplicateJob = errors.New("job already exists")
)

type Job struct {
	ID string `json:"id"`
	Type string `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	State State `json:"state"`
	Attempt int `json:"attempt"`
	MaxAttempts int `json:"max_attempts"`
	AvailableAt time.Time `json:"available_at"`
	LeaseOwner string `json:"lease_owner,omitempty"`
	LeaseUntil time.Time `json:"lease_until,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	LastError string `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Queue struct { mu sync.Mutex; jobs map[string]Job; order []string; now func() time.Time }

func New() *Queue { return NewWithClock(time.Now) }
func NewWithClock(now func() time.Time) *Queue { if now == nil { now = time.Now }; return &Queue{jobs: make(map[string]Job), now: now} }

func (q *Queue) Enqueue(job Job) (Job, error) {
	job.ID = strings.TrimSpace(job.ID); job.Type = strings.TrimSpace(job.Type)
	if job.ID == "" || job.Type == "" { return Job{}, errors.New("job id and type are required") }
	if job.MaxAttempts == 0 { job.MaxAttempts = 3 }
	if job.MaxAttempts < 1 || job.MaxAttempts > 100 { return Job{}, errors.New("max_attempts must be between 1 and 100") }
	if len(job.Payload) > 1<<20 { return Job{}, errors.New("payload exceeds 1 MiB") }
	if len(job.Payload) != 0 && !json.Valid(job.Payload) { return Job{}, errors.New("payload must contain valid JSON") }
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock()
	if _, exists := q.jobs[job.ID]; exists { return Job{}, ErrDuplicateJob }
	job.State = Queued; job.Attempt = 0; job.LeaseOwner = ""; job.LeaseUntil = time.Time{}; job.CreatedAt = now; job.UpdatedAt = now
	if job.AvailableAt.IsZero() { job.AvailableAt = now }
	q.jobs[job.ID] = cloneJob(job); q.order = append(q.order, job.ID)
	return cloneJob(job), nil
}

func (q *Queue) Claim(worker string, lease time.Duration) (Job, error) {
	worker = strings.TrimSpace(worker)
	if worker == "" { return Job{}, errors.New("worker is required") }
	if lease < time.Second || lease > time.Hour { return Job{}, errors.New("lease must be between 1s and 1h") }
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); q.reclaimExpiredLocked(now)
	for _, id := range q.order {
		job := q.jobs[id]; if job.State != Queued || job.AvailableAt.After(now) { continue }
		job.State = Leased; job.Attempt++; job.LeaseOwner = worker; job.LeaseUntil = now.Add(lease); job.UpdatedAt = now; q.jobs[id] = job
		return cloneJob(job), nil
	}
	return Job{}, ErrNoWork
}

func (q *Queue) Heartbeat(id, worker string, lease time.Duration) (Job, error) {
	if lease < time.Second || lease > time.Hour { return Job{}, errors.New("lease must be between 1s and 1h") }
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); job, err := q.assertLeaseLocked(id, worker, now); if err != nil { return Job{}, err }
	job.LeaseUntil = now.Add(lease); job.UpdatedAt = now; q.jobs[id] = job; return cloneJob(job), nil
}

func (q *Queue) Complete(id, worker string, result json.RawMessage) (Job, error) {
	if len(result) > 1<<20 { return Job{}, errors.New("result exceeds 1 MiB") }
	if len(result) != 0 && !json.Valid(result) { return Job{}, errors.New("result must contain valid JSON") }
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); job, err := q.assertLeaseLocked(id, worker, now); if err != nil { return Job{}, err }
	job.State = Completed; job.Result = append(json.RawMessage(nil), result...); job.LeaseOwner = ""; job.LeaseUntil = time.Time{}; job.UpdatedAt = now; q.jobs[id] = job
	return cloneJob(job), nil
}

func (q *Queue) Fail(id, worker, reason string, retryAfter time.Duration) (Job, error) {
	if retryAfter < 0 || retryAfter > 24*time.Hour { return Job{}, errors.New("retry_after must be between 0 and 24h") }
	reason = strings.TrimSpace(reason); if len(reason) > 2048 { return Job{}, errors.New("error reason exceeds 2048 bytes") }
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); job, err := q.assertLeaseLocked(id, worker, now); if err != nil { return Job{}, err }
	job.LastError = reason; job.LeaseOwner = ""; job.LeaseUntil = time.Time{}; job.UpdatedAt = now
	if job.Attempt >= job.MaxAttempts { job.State = DeadLetter } else { job.State = Queued; job.AvailableAt = now.Add(retryAfter) }
	q.jobs[id] = job; return cloneJob(job), nil
}

func (q *Queue) Get(id string) (Job, error) {
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); q.reclaimExpiredLocked(now); job, ok := q.jobs[id]; if !ok { return Job{}, ErrNotFound }; return cloneJob(job), nil
}

func (q *Queue) List(state State) []Job {
	now := q.now().UTC(); q.mu.Lock(); defer q.mu.Unlock(); q.reclaimExpiredLocked(now); result := make([]Job, 0, len(q.jobs))
	for _, job := range q.jobs { if state == "" || job.State == state { result = append(result, cloneJob(job)) } }
	sort.Slice(result, func(i, j int) bool { if result[i].CreatedAt.Equal(result[j].CreatedAt) { return result[i].ID < result[j].ID }; return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

func (q *Queue) assertLeaseLocked(id, worker string, now time.Time) (Job, error) {
	job, ok := q.jobs[id]; if !ok { return Job{}, ErrNotFound }
	if job.State != Leased || job.LeaseOwner != worker || !job.LeaseUntil.After(now) { return Job{}, ErrLeaseLost }
	return job, nil
}

func (q *Queue) reclaimExpiredLocked(now time.Time) {
	for id, job := range q.jobs {
		if job.State != Leased || job.LeaseUntil.After(now) { continue }
		job.LeaseOwner = ""; job.LeaseUntil = time.Time{}; job.UpdatedAt = now; job.LastError = "lease expired"
		if job.Attempt >= job.MaxAttempts { job.State = DeadLetter } else { job.State = Queued; job.AvailableAt = now }
		q.jobs[id] = job
	}
}

func cloneJob(job Job) Job { job.Payload = append(json.RawMessage(nil), job.Payload...); job.Result = append(json.RawMessage(nil), job.Result...); return job }
func (s State) Valid() bool { switch s { case "", Queued, Leased, Completed, DeadLetter: return true; default: return false } }
func (j Job) String() string { return fmt.Sprintf("job{id=%s type=%s state=%s attempt=%d/%d}", j.ID, j.Type, j.State, j.Attempt, j.MaxAttempts) }
