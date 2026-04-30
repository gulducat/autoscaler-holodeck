package holodeck

import (
	"sync"
	"time"
)

const DefaultWorldID = "default"

// MetricRule defines how a metric value is computed.
type MetricRule struct {
	Type string `json:"type"` // "authored" | "capacity_coupled"

	// Authored fields.
	Value float64 `json:"value,omitempty"`

	// Capacity-coupled fields.
	Base        float64  `json:"base,omitempty"`
	AllocFactor float64  `json:"alloc_factor,omitempty"`
	NodeFactor  float64  `json:"node_factor,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`

	// Optional modifiers (applied to both types).
	LagSeconds               float64 `json:"lag_seconds,omitempty"`
	Saturation               float64 `json:"saturation,omitempty"`
	DiminishingReturnsFactor float64 `json:"diminishing_returns_factor,omitempty"`
}

// Compute returns the current metric value given Nomad alloc and node counts.
func (r MetricRule) Compute(allocCount, nodeCount int64) float64 {
	var v float64
	switch r.Type {
	case "capacity_coupled":
		v = r.Base + float64(allocCount)*r.AllocFactor + float64(nodeCount)*r.NodeFactor
		if r.Min != nil && v < *r.Min {
			v = *r.Min
		}
		if r.Max != nil && v > *r.Max {
			v = *r.Max
		}
	default: // "authored"
		v = r.Value
	}

	if r.Saturation > 0 && v > r.Saturation {
		v = r.Saturation
	}
	if r.DiminishingReturnsFactor > 0 && v > 0.5 {
		excess := v - 0.5
		v = 0.5 + excess*r.DiminishingReturnsFactor
	}
	return v
}

// World holds the authored metric rules for one named world.
type World struct {
	mu      sync.RWMutex
	id      string
	metrics map[string]MetricRule
}

func newWorld(id string) *World {
	return &World{id: id, metrics: make(map[string]MetricRule)}
}

// ID returns the world's identifier.
func (w *World) ID() string { return w.id }

// Set replaces the full set of metric rules and returns the names of changed metrics.
func (w *World) Set(metrics map[string]MetricRule) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	changed := make([]string, 0, len(metrics))
	for name, rule := range metrics {
		if existing, ok := w.metrics[name]; !ok || existing != rule {
			changed = append(changed, name)
		}
	}

	w.metrics = make(map[string]MetricRule, len(metrics))
	for k, v := range metrics {
		w.metrics[k] = v
	}
	return changed
}

// SetOne updates a single metric rule without replacing the full set.
// Returns true if the rule was new or changed.
func (w *World) SetOne(name string, rule MetricRule) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing, ok := w.metrics[name]; ok && existing == rule {
		return false
	}
	w.metrics[name] = rule
	return true
}

// Reset clears all metric rules.
func (w *World) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.metrics = make(map[string]MetricRule)
}

// Query returns the computed value for the named metric, or false if not found.
func (w *World) Query(name string, allocCount, nodeCount int64) (float64, time.Time, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	rule, ok := w.metrics[name]
	if !ok {
		return 0, time.Time{}, false
	}
	return rule.Compute(allocCount, nodeCount), time.Now().UTC(), true
}

// State returns a snapshot of the world's metric rules.
func (w *World) State() map[string]MetricRule {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[string]MetricRule, len(w.metrics))
	for k, v := range w.metrics {
		out[k] = v
	}
	return out
}

// WorldManager manages multiple named worlds.
type WorldManager struct {
	mu             sync.RWMutex
	worlds         map[string]*World
	tracker        *NomadTracker
	observer       *ObserverClient
	sampledMu      sync.RWMutex
	sampledMetrics map[string]SampledMetricEntry
}

// NewWorldManager creates a WorldManager with a pre-created default world.
func NewWorldManager(tracker *NomadTracker, observer *ObserverClient) *WorldManager {
	m := &WorldManager{
		worlds:         make(map[string]*World),
		tracker:        tracker,
		observer:       observer,
		sampledMetrics: make(map[string]SampledMetricEntry),
	}
	m.worlds[DefaultWorldID] = newWorld(DefaultWorldID)
	return m
}

// Get returns the world with the given ID, or nil if it does not exist.
func (m *WorldManager) Get(id string) *World {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.worlds[id]
}

// GetOrCreate returns an existing world or creates a new one.
func (m *WorldManager) GetOrCreate(id string) *World {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.worlds[id]
	if !ok {
		w = newWorld(id)
		m.worlds[id] = w
	}
	return w
}

// List returns all world IDs.
func (m *WorldManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.worlds))
	for id := range m.worlds {
		ids = append(ids, id)
	}
	return ids
}

// Set replaces the metric rules for a world, creating it if needed, and
// emits a metric_rule_change event to the Observer for each changed metric.
func (m *WorldManager) Set(id string, metrics map[string]MetricRule) *World {
	w := m.GetOrCreate(id)
	changed := w.Set(metrics)
	for _, name := range changed {
		rule := metrics[name]
		m.observer.Send("metric_rule_change", map[string]any{
			"world":  id,
			"metric": name,
			"rule":   rule,
		})
	}
	return w
}

// SetOne upserts a single metric rule into a world, emitting a
// metric_rule_change event to the Observer if the rule is new or changed.
func (m *WorldManager) SetOne(id, name string, rule MetricRule) {
	w := m.GetOrCreate(id)
	if w.SetOne(name, rule) {
		m.observer.Send("metric_rule_change", map[string]any{
			"world":  id,
			"metric": name,
			"rule":   rule,
		})
	}
}

// Reset clears a world's metric rules and emits a world_reset event to the Observer.
func (m *WorldManager) Reset(id string) *World {
	w := m.GetOrCreate(id)
	w.Reset()
	m.observer.Send("world_reset", map[string]any{"world": id})
	return w
}

// AllocCount returns the current number of running allocations tracked by Nomad.
func (m *WorldManager) AllocCount() int64 { return m.tracker.AllocCount() }

// NodeCount returns the current number of ready nodes tracked by Nomad.
func (m *WorldManager) NodeCount() int64 { return m.tracker.NodeCount() }

// StoreSampledMetric stores a metric entry obtained from a sampled source.
func (m *WorldManager) StoreSampledMetric(name string, entry SampledMetricEntry) {
	m.sampledMu.Lock()
	defer m.sampledMu.Unlock()
	m.sampledMetrics[name] = entry
}

// GetSampledMetrics returns a snapshot of all stored sampled metrics.
func (m *WorldManager) GetSampledMetrics() map[string]SampledMetricEntry {
	m.sampledMu.RLock()
	defer m.sampledMu.RUnlock()
	out := make(map[string]SampledMetricEntry, len(m.sampledMetrics))
	for k, v := range m.sampledMetrics {
		out[k] = v
	}
	return out
}
