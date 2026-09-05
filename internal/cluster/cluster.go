package cluster

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNodeNotFound = errors.New("cluster node not found")

type Node struct { ID string `json:"id"`; Address string `json:"address"`; Zone string `json:"zone,omitempty"`; StartedAt time.Time `json:"started_at"`; LastHeartbeat time.Time `json:"last_heartbeat"`; ExpiresAt time.Time `json:"expires_at"` }
type Membership struct { mu sync.Mutex; nodes map[string]Node; now func() time.Time }

func New() *Membership { return NewWithClock(time.Now) }
func NewWithClock(now func() time.Time) *Membership { if now == nil { now = time.Now }; return &Membership{nodes: make(map[string]Node), now: now} }

func (m *Membership) Register(id, address, zone string, ttl time.Duration) (Node, error) {
	id, address, zone = strings.TrimSpace(id), strings.TrimSpace(address), strings.TrimSpace(zone)
	if id == "" || address == "" { return Node{}, errors.New("node id and address are required") }
	if ttl < time.Second || ttl > time.Hour { return Node{}, errors.New("ttl must be between 1s and 1h") }
	now := m.now().UTC(); node := Node{ID:id, Address:address, Zone:zone, StartedAt:now, LastHeartbeat:now, ExpiresAt:now.Add(ttl)}
	m.mu.Lock(); defer m.mu.Unlock(); m.reapLocked(now); if existing, ok := m.nodes[id]; ok { node.StartedAt = existing.StartedAt }; m.nodes[id] = node; return node, nil
}

func (m *Membership) Heartbeat(id string, ttl time.Duration) (Node, error) {
	if ttl < time.Second || ttl > time.Hour { return Node{}, errors.New("ttl must be between 1s and 1h") }
	now := m.now().UTC(); m.mu.Lock(); defer m.mu.Unlock(); m.reapLocked(now); node, ok := m.nodes[id]; if !ok { return Node{}, ErrNodeNotFound }; node.LastHeartbeat = now; node.ExpiresAt = now.Add(ttl); m.nodes[id] = node; return node, nil
}

func (m *Membership) List() []Node { now := m.now().UTC(); m.mu.Lock(); defer m.mu.Unlock(); m.reapLocked(now); result := make([]Node,0,len(m.nodes)); for _, node := range m.nodes { result = append(result,node) }; sort.Slice(result,func(i,j int)bool{return result[i].ID<result[j].ID}); return result }
func (m *Membership) ReapExpired() int { m.mu.Lock(); defer m.mu.Unlock(); return m.reapLocked(m.now().UTC()) }
func (m *Membership) reapLocked(now time.Time) int { removed:=0; for id,node:=range m.nodes { if !node.ExpiresAt.After(now) { delete(m.nodes,id); removed++ } }; return removed }
