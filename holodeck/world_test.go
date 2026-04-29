package holodeck

import (
	"testing"
)

func TestMetricRule_Compute_Authored(t *testing.T) {
	r := MetricRule{Type: "authored", Value: 0.75}
	if got := r.Compute(0, 0); got != 0.75 {
		t.Errorf("authored: got %v, want 0.75", got)
	}
}

func TestMetricRule_Compute_CapacityCoupled(t *testing.T) {
	r := MetricRule{
		Type:        "capacity_coupled",
		Base:        10,
		AllocFactor: 2,
		NodeFactor:  5,
	}
	// 10 + 3*2 + 1*5 = 21
	if got := r.Compute(3, 1); got != 21 {
		t.Errorf("capacity_coupled: got %v, want 21", got)
	}
}

func TestMetricRule_Compute_Clamped(t *testing.T) {
	min, max := 5.0, 15.0
	r := MetricRule{
		Type:        "capacity_coupled",
		Base:        0,
		AllocFactor: 100,
		Min:         &min,
		Max:         &max,
	}
	// 100*1 = 100, clamped to max 15
	if got := r.Compute(1, 0); got != 15 {
		t.Errorf("max clamp: got %v, want 15", got)
	}
	// 100*0 = 0, clamped to min 5
	if got := r.Compute(0, 0); got != 5 {
		t.Errorf("min clamp: got %v, want 5", got)
	}
}

func TestMetricRule_Compute_Saturation(t *testing.T) {
	r := MetricRule{Type: "authored", Value: 0.9, Saturation: 0.8}
	if got := r.Compute(0, 0); got != 0.8 {
		t.Errorf("saturation: got %v, want 0.8", got)
	}
}

func TestMetricRule_Compute_DiminishingReturns(t *testing.T) {
	r := MetricRule{Type: "authored", Value: 0.7, DiminishingReturnsFactor: 0.5}
	// excess above 0.5 = 0.2; 0.5 + 0.2*0.5 = 0.6
	want := 0.5 + 0.2*0.5
	if got := r.Compute(0, 0); got != want {
		t.Errorf("diminishing returns: got %v, want %v", got, want)
	}
}

func TestWorld_SetAndQuery(t *testing.T) {
	w := newWorld("test")
	rules := map[string]MetricRule{
		"cpu": {Type: "authored", Value: 0.5},
	}
	w.Set(rules)

	val, _, ok := w.Query("cpu", 0, 0)
	if !ok {
		t.Fatal("expected metric to be found")
	}
	if val != 0.5 {
		t.Errorf("got %v, want 0.5", val)
	}
}

func TestWorld_QueryMissing(t *testing.T) {
	w := newWorld("test")
	_, _, ok := w.Query("missing", 0, 0)
	if ok {
		t.Error("expected not found")
	}
}

func TestWorld_Reset(t *testing.T) {
	w := newWorld("test")
	w.Set(map[string]MetricRule{"cpu": {Type: "authored", Value: 1}})
	w.Reset()
	if st := w.State(); len(st) != 0 {
		t.Errorf("expected empty state after reset, got %v", st)
	}
}

func TestWorld_SetReturnsChanged(t *testing.T) {
	w := newWorld("test")
	// First set: both are new, so both changed.
	changed := w.Set(map[string]MetricRule{
		"a": {Type: "authored", Value: 1},
		"b": {Type: "authored", Value: 2},
	})
	if len(changed) != 2 {
		t.Errorf("expected 2 changed, got %d", len(changed))
	}
	// Second set with same rules: nothing changed.
	changed = w.Set(map[string]MetricRule{
		"a": {Type: "authored", Value: 1},
		"b": {Type: "authored", Value: 2},
	})
	if len(changed) != 0 {
		t.Errorf("expected 0 changed, got %d: %v", len(changed), changed)
	}
}

func TestWorldManager_DefaultWorldExists(t *testing.T) {
	m := NewWorldManager(NewNomadTracker(), NewObserverClient("", noopLogger()))
	if w := m.Get(DefaultWorldID); w == nil {
		t.Error("default world should exist")
	}
}

func TestWorldManager_MultiWorld(t *testing.T) {
	m := NewWorldManager(NewNomadTracker(), NewObserverClient("", noopLogger()))
	m.Set("alpha", map[string]MetricRule{"x": {Type: "authored", Value: 1}})
	m.Set("beta", map[string]MetricRule{"x": {Type: "authored", Value: 2}})

	alpha := m.Get("alpha")
	beta := m.Get("beta")

	va, _, _ := alpha.Query("x", 0, 0)
	vb, _, _ := beta.Query("x", 0, 0)

	if va == vb {
		t.Error("worlds should be independent")
	}
}

func TestWorldManager_Reset(t *testing.T) {
	m := NewWorldManager(NewNomadTracker(), NewObserverClient("", noopLogger()))
	m.Set(DefaultWorldID, map[string]MetricRule{"x": {Type: "authored", Value: 1}})
	m.Reset(DefaultWorldID)

	w := m.Get(DefaultWorldID)
	if w == nil {
		t.Fatal("world should still exist after reset")
	}
	if st := w.State(); len(st) != 0 {
		t.Error("world metrics should be empty after reset")
	}
}
