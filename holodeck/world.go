package holodeck

import (
	"sync"
	"time"
)

const DefaultWorldID = "default"

// MetricRule defines how a metric value is computed.
type MetricRule struct {
	Type string `json:"type"` // "authored" | "capacity_coupled" | "scheduled"

	// Authored / scheduled fields.
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
	Count                    int     `json:"count,omitempty"`
	Delta                    string  `json:"delta,omitempty"`
}

// HistoryEntry is a snapshot of a metric rule captured before it was replaced.
type HistoryEntry struct {
	Rule  MetricRule `json:"rule"`
	SetAt time.Time  `json:"set_at"`
}

// MetricEntry holds the current rule for a metric and its full change history
// since the last world reset.
type MetricEntry struct {
	Rule    MetricRule     `json:"rule"`
	History []HistoryEntry `json:"history,omitempty"`
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

// World holds the authored metric entries for one named world.
type World struct {
	mu      sync.RWMutex
	id      string
	metrics map[string]MetricEntry
}

func newWorld(id string) *World {
	return &World{id: id, metrics: make(map[string]MetricEntry)}
}

// ID returns the world's identifier.
func (w *World) ID() string { return w.id }

// Set replaces the full set of metric rules. For each metric whose rule
// changes, the previous rule is pushed to its history. Returns the names
// of changed metrics.
func (w *World) Set(metrics map[string]MetricRule) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now().UTC()
	changed := make([]string, 0, len(metrics))
	newEntries := make(map[string]MetricEntry, len(metrics))
	for name, rule := range metrics {
		existing, ok := w.metrics[name]
		if !ok || existing.Rule != rule {
			changed = append(changed, name)
			var history []HistoryEntry
			if ok {
				history = make([]HistoryEntry, len(existing.History)+1)
				copy(history, existing.History)
				history[len(existing.History)] = HistoryEntry{Rule: existing.Rule, SetAt: now}
			}
			newEntries[name] = MetricEntry{Rule: rule, History: history}
		} else {
			newEntries[name] = existing
		}
	}
	w.metrics = newEntries
	return changed
}

// SetOne upserts a single metric rule, pushing the previous rule to history
// if it changed. Returns true if the rule was new or changed.
func (w *World) SetOne(name string, rule MetricRule) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	existing, ok := w.metrics[name]
	if ok && existing.Rule == rule {
		return false
	}

	now := time.Now().UTC()
	var history []HistoryEntry
	if ok {
		history = make([]HistoryEntry, len(existing.History)+1)
		copy(history, existing.History)
		history[len(existing.History)] = HistoryEntry{Rule: existing.Rule, SetAt: now}
	}
	w.metrics[name] = MetricEntry{Rule: rule, History: history}
	return true
}

// Reset clears all metric entries and their histories.
func (w *World) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.metrics = make(map[string]MetricEntry)
}

// Query returns the computed value for the named metric, or false if not found.
func (w *World) Query(name string, allocCount, nodeCount int64) (float64, time.Time, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	entry, ok := w.metrics[name]
	if !ok {
		return 0, time.Time{}, false
	}
	return entry.Rule.Compute(allocCount, nodeCount), time.Now().UTC(), true
}

// State returns a snapshot of all metric entries (current rules + histories).
func (w *World) State() map[string]MetricEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[string]MetricEntry, len(w.metrics))
	for k, v := range w.metrics {
		hist := make([]HistoryEntry, len(v.History))
		copy(hist, v.History)
		out[k] = MetricEntry{Rule: v.Rule, History: hist}
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
