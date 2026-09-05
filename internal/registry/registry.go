package registry

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound      = errors.New("service instance not found")
	ErrNoHealthyNode = errors.New("no healthy service instance available")
)

const (
	minTTL = time.Second
	maxTTL = 24 * time.Hour
)

type RegisterInput struct {
	Service  string            `json:"service"`
	ID       string            `json:"id"`
	Address  string            `json:"address"`
	Weight   int               `json:"weight"`
	TTL      time.Duration     `json:"-"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Instance struct {
	Service        string            `json:"service"`
	ID             string            `json:"id"`
	Address        string            `json:"address"`
	Weight         int               `json:"weight"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	RegisteredAt   time.Time         `json:"registered_at"`
	LeaseExpiresAt time.Time         `json:"lease_expires_at"`
	Healthy        bool              `json:"healthy"`
}

type Registry struct {
	mu        sync.Mutex
	instances map[string]map[string]Instance
	cursors   map[string]uint64
	now       func() time.Time
}

func New() *Registry { return NewWithClock(time.Now) }

func NewWithClock(now func() time.Time) *Registry {
	if now == nil {
		now = time.Now
	}
	return &Registry{instances: make(map[string]map[string]Instance), cursors: make(map[string]uint64), now: now}
}

func (r *Registry) Register(input RegisterInput) (Instance, error) {
	input.Service = strings.TrimSpace(input.Service)
	input.ID = strings.TrimSpace(input.ID)
	input.Address = strings.TrimSpace(input.Address)
	if input.Service == "" || input.ID == "" || input.Address == "" {
		return Instance{}, errors.New("service, id and address are required")
	}
	if input.Weight == 0 {
		input.Weight = 1
	}
	if input.Weight < 1 || input.Weight > 1000 {
		return Instance{}, errors.New("weight must be between 1 and 1000")
	}
	if err := validateTTL(input.TTL); err != nil {
		return Instance{}, err
	}

	now := r.now().UTC()
	instance := Instance{Service: input.Service, ID: input.ID, Address: input.Address, Weight: input.Weight, Metadata: cloneMetadata(input.Metadata), RegisteredAt: now, LeaseExpiresAt: now.Add(input.TTL), Healthy: true}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reapLocked(now)
	if _, ok := r.instances[input.Service]; !ok {
		r.instances[input.Service] = make(map[string]Instance)
	}
	r.instances[input.Service][input.ID] = instance
	return cloneInstance(instance), nil
}

func (r *Registry) Heartbeat(service, id string, ttl time.Duration) (Instance, error) {
	if err := validateTTL(ttl); err != nil {
		return Instance{}, err
	}
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reapLocked(now)
	instance, ok := r.instances[service][id]
	if !ok {
		return Instance{}, ErrNotFound
	}
	instance.LeaseExpiresAt = now.Add(ttl)
	r.instances[service][id] = instance
	return cloneInstance(instance), nil
}

func (r *Registry) SetHealth(service, id string, healthy bool) error {
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reapLocked(now)
	instance, ok := r.instances[service][id]
	if !ok {
		return ErrNotFound
	}
	instance.Healthy = healthy
	r.instances[service][id] = instance
	return nil
}

func (r *Registry) Resolve(service string) (Instance, error) {
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reapLocked(now)
	candidates := make([]Instance, 0)
	totalWeight := 0
	for _, instance := range r.instances[service] {
		if instance.Healthy {
			candidates = append(candidates, instance)
			totalWeight += instance.Weight
		}
	}
	if len(candidates) == 0 || totalWeight == 0 {
		return Instance{}, ErrNoHealthyNode
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	cursor := r.cursors[service] % uint64(totalWeight)
	r.cursors[service]++
	cumulative := uint64(0)
	for _, instance := range candidates {
		cumulative += uint64(instance.Weight)
		if cursor < cumulative {
			return cloneInstance(instance), nil
		}
	}
	return Instance{}, fmt.Errorf("%w: weighted resolver invariant", ErrNoHealthyNode)
}

func (r *Registry) List(service string) []Instance {
	now := r.now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reapLocked(now)
	result := make([]Instance, 0)
	if service != "" {
		for _, instance := range r.instances[service] {
			result = append(result, cloneInstance(instance))
		}
	} else {
		for _, byID := range r.instances {
			for _, instance := range byID {
				result = append(result, cloneInstance(instance))
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Service == result[j].Service {
			return result[i].ID < result[j].ID
		}
		return result[i].Service < result[j].Service
	})
	return result
}

func (r *Registry) ReapExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reapLocked(r.now().UTC())
}

func (r *Registry) reapLocked(now time.Time) int {
	removed := 0
	for service, byID := range r.instances {
		for id, instance := range byID {
			if !instance.LeaseExpiresAt.After(now) {
				delete(byID, id)
				removed++
			}
		}
		if len(byID) == 0 {
			delete(r.instances, service)
			delete(r.cursors, service)
		}
	}
	return removed
}

func validateTTL(ttl time.Duration) error {
	if ttl < minTTL || ttl > maxTTL {
		return fmt.Errorf("ttl must be between %s and %s", minTTL, maxTTL)
	}
	return nil
}

func cloneMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for k, v := range input {
		cloned[k] = v
	}
	return cloned
}

func cloneInstance(instance Instance) Instance {
	instance.Metadata = cloneMetadata(instance.Metadata)
	return instance
}
