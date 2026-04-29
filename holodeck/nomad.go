package holodeck

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	nomadapi "github.com/hashicorp/nomad/api"
)

// NomadTracker maintains current alloc and node counts. All methods are safe
// to call concurrently.
type NomadTracker struct {
	allocCount atomic.Int64
	nodeCount  atomic.Int64
}

func NewNomadTracker() *NomadTracker { return &NomadTracker{} }

// AllocCount returns the number of running allocations.
func (t *NomadTracker) AllocCount() int64 { return t.allocCount.Load() }

// NodeCount returns the number of ready nodes.
func (t *NomadTracker) NodeCount() int64 { return t.nodeCount.Load() }

// NomadPoller uses Nomad blocking queries to maintain up-to-date alloc and
// node counts in a NomadTracker. It uses the Nomad SDK client so that all
// SDK connectivity options (TLS, ACL tokens, namespaces) are handled
// transparently. Reconnects automatically with exponential backoff on error.
type NomadPoller struct {
	client  *nomadapi.Client
	tracker *NomadTracker
	logger  hclog.Logger
}

func NewNomadPoller(client *nomadapi.Client, tracker *NomadTracker, logger hclog.Logger) *NomadPoller {
	return &NomadPoller{client: client, tracker: tracker, logger: logger}
}

// Run starts alloc and node watchers and blocks until ctx is cancelled.
func (p *NomadPoller) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p.watchAllocs(ctx) }()
	go func() { defer wg.Done(); p.watchNodes(ctx) }()
	wg.Wait()
}

func (p *NomadPoller) watchAllocs(ctx context.Context) {
	var waitIndex uint64
	backoff := time.Second

	for ctx.Err() == nil {
		q := &nomadapi.QueryOptions{
			WaitIndex: waitIndex,
			WaitTime:  5 * time.Minute,
		}
		q = q.WithContext(ctx)

		stubs, meta, err := p.client.Allocations().List(q)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.logger.Warn("alloc list error, retrying", "error", err, "retry_in", backoff)
			sleep(ctx, backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		backoff = time.Second
		waitIndex = meta.LastIndex

		var running int64
		for _, s := range stubs {
			if s.ClientStatus == "running" {
				running++
			}
		}
		p.tracker.allocCount.Store(running)
		p.logger.Debug("alloc count updated", "running", running)
	}
}

func (p *NomadPoller) watchNodes(ctx context.Context) {
	var waitIndex uint64
	backoff := time.Second

	for ctx.Err() == nil {
		q := &nomadapi.QueryOptions{
			WaitIndex: waitIndex,
			WaitTime:  5 * time.Minute,
		}
		q = q.WithContext(ctx)

		stubs, meta, err := p.client.Nodes().List(q)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.logger.Warn("node list error, retrying", "error", err, "retry_in", backoff)
			sleep(ctx, backoff)
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		backoff = time.Second
		waitIndex = meta.LastIndex

		var ready int64
		for _, s := range stubs {
			if s.Status == "ready" {
				ready++
			}
		}
		p.tracker.nodeCount.Store(ready)
		p.logger.Debug("node count updated", "ready", ready)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
