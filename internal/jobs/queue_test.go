package jobs

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestLeaseFencingAndRetry(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	q := NewWithClock(func() time.Time { return now })
	if _, err := q.Enqueue(Job{ID: "j1", Type: "email", Payload: json.RawMessage(`{"to":"a"}`), MaxAttempts: 2}); err != nil {
		t.Fatal(err)
	}
	first, err := q.Claim("worker-a", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if first.Attempt != 1 {
		t.Fatalf("attempt=%d want 1", first.Attempt)
	}
	now = now.Add(3 * time.Second)
	second, err := q.Claim("worker-b", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if second.Attempt != 2 {
		t.Fatalf("attempt=%d want 2", second.Attempt)
	}
	if _, err := q.Complete("j1", "worker-a", json.RawMessage(`{"ok":true}`)); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale worker completion error=%v", err)
	}
	completed, err := q.Complete("j1", "worker-b", json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != Completed {
		t.Fatalf("state=%s want completed", completed.State)
	}
}

func TestFinalLeaseExpiryDeadLetters(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	q := NewWithClock(func() time.Time { return now })
	if _, err := q.Enqueue(Job{ID: "j1", Type: "task", MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Claim("w", time.Second); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Second)
	job, err := q.Get("j1")
	if err != nil {
		t.Fatal(err)
	}
	if job.State != DeadLetter {
		t.Fatalf("state=%s want dead_letter", job.State)
	}
}
