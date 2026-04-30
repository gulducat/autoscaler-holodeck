package observer

import (
	"encoding/json"
	"sync"
	"time"
)

const kindWorldReset = "world_reset"

// EventInput is the caller-supplied fields for a new event.
// SentAt is optional; nil is acceptable for Nomad stream events.
type EventInput struct {
	Source  string
	Kind    string
	SentAt  *time.Time
	Payload json.RawMessage
	Summary json.RawMessage
}

// Event is a stored event with Observer-assigned ordering fields.
type Event struct {
	Seq        int64           `json:"seq"`
	Run        int64           `json:"run"`
	IngestedAt time.Time       `json:"ingested_at"`
	Source     string          `json:"source"`
	Kind       string          `json:"kind"`
	SentAt     *time.Time      `json:"sent_at,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	Summary    json.RawMessage `json:"summary,omitempty"`
}

// EventStore is the interface used by the API handlers and stream listener.
type EventStore interface {
	Add(input EventInput) Event
	Query(run, since int64, kind string) (currentRun int64, events []Event)
	CurrentRun() int64
}

// Store is an in-memory event store keyed by run ID.
// Run 0 holds events received before the first world_reset.
// Each world_reset event opens a new run whose ID equals the event's seq.
type Store struct {
	mu         sync.RWMutex
	runs       map[int64][]Event
	seq        int64
	currentRun int64
}

func NewStore() *Store {
	return &Store{runs: make(map[int64][]Event)}
}

// Add stores the event, assigns seq and ingested_at, and — if the kind is
// world_reset — advances currentRun to the new event's seq. The world_reset
// event itself is stored under the new run ID.
func (s *Store) Add(input EventInput) Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	e := Event{
		Seq:        s.seq,
		Run:        s.currentRun,
		IngestedAt: time.Now().UTC(),
		Source:     input.Source,
		Kind:       input.Kind,
		SentAt:     input.SentAt,
		Payload:    input.Payload,
		Summary:    input.Summary,
	}

	if input.Kind == kindWorldReset {
		s.currentRun = s.seq
		e.Run = s.currentRun
	}

	s.runs[e.Run] = append(s.runs[e.Run], e)
	return e
}

// Query returns events for run filtered by since (exclusive) and kind.
// currentRun is returned as a consistent snapshot alongside the events.
// The caller is responsible for resolving an omitted run param to CurrentRun()
// before calling Query — run 0 is a valid run ID (pre-reset events).
func (s *Store) Query(run, since int64, kind string) (int64, []Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	currentRun := s.currentRun
	src := s.runs[run]
	out := make([]Event, 0, len(src))
	for _, e := range src {
		if e.Seq <= since {
			continue
		}
		if kind != "" && e.Kind != kind {
			continue
		}
		out = append(out, e)
	}
	return currentRun, out
}

func (s *Store) CurrentRun() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRun
}
