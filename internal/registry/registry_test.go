package registry

import (
	"errors"
	"testing"
	"time"
)

func TestWeightedResolveAndHealth(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	r := NewWithClock(func() time.Time { return now })
	if _, err := r.Register(RegisterInput{Service: "api", ID: "a", Address: "http://a", Weight: 1, TTL: time.Minute}); err != nil { t.Fatal(err) }
	if _, err := r.Register(RegisterInput{Service: "api", ID: "b", Address: "http://b", Weight: 2, TTL: time.Minute}); err != nil { t.Fatal(err) }
	got := make([]string, 3)
	for i := range got { instance, err := r.Resolve("api"); if err != nil { t.Fatal(err) }; got[i] = instance.ID }
	want := []string{"a", "b", "b"}
	for i := range want { if got[i] != want[i] { t.Fatalf("resolve[%d]=%q want %q", i, got[i], want[i]) } }
	if err := r.SetHealth("api", "b", false); err != nil { t.Fatal(err) }
	instance, err := r.Resolve("api"); if err != nil { t.Fatal(err) }
	if instance.ID != "a" { t.Fatalf("got %q want a", instance.ID) }
}

func TestLeaseExpiryAndHeartbeat(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	r := NewWithClock(func() time.Time { return now })
	if _, err := r.Register(RegisterInput{Service: "api", ID: "a", Address: "http://a", TTL: 2 * time.Second}); err != nil { t.Fatal(err) }
	now = now.Add(time.Second)
	if _, err := r.Heartbeat("api", "a", 3*time.Second); err != nil { t.Fatal(err) }
	now = now.Add(2 * time.Second)
	if len(r.List("api")) != 1 { t.Fatal("instance expired too early") }
	now = now.Add(2 * time.Second)
	if _, err := r.Resolve("api"); !errors.Is(err, ErrNoHealthyNode) { t.Fatalf("expected ErrNoHealthyNode, got %v", err) }
}
