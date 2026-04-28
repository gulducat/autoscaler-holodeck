# Spec: Observer Service

## Context

The Observer is the central **event sink and correlator**. It receives events from all other components, assigns authoritative ordering, and provides a read-only UI showing what actually happened.

The Observer **witnesses**. It does not drive time, control anything, or infer policy.

See [`plan.md`](../plan.md) §Observer for the full description.

## Depends On

- Phase 0: [`specs/repo-bootstrap.md`](../repo-bootstrap.md) — directory layout and Go workspace must exist
- Phase 1: [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md) — implement the API defined there

## Repositories and Packages

- Repo: `gulducat/autoscaler-holodeck`
- Package: `observer/` at repo root
- Module: `github.com/gulducat/autoscaler-holodeck/observer`
- Binary: `observer/cmd/observer/main.go`

## What to Build

### Event ingest

An HTTP endpoint (defined in [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md)) that receives events from:
- Holodeck (world-authoring events: resets, metric rule changes)
- holodeck-apm plugin (metric observations)
- nodesim-target plugin (scaling intent)

On receipt, the Observer:
1. Assigns a wall-clock ingest timestamp (authoritative for UI display)
2. Assigns a monotonically increasing sequence number (authoritative ordering)
3. Stores the event

### Nomad event stream listener

In addition to events pushed by plugins, the Observer independently listens to the Nomad event stream for:
- Job scaling events (actual scaling outcomes)
- Allocation state changes
- Node registration/deregistration

These are treated as events with the same ordering model as pushed events.

### Event storage

An in-memory store keyed by run. A "run" starts at a Holodeck reset event and ends at the next one. No persistence across restarts is required.

### Read-only UI

A single-page UI (plain HTML + JS, no build step) served by the Go binary:
- Timeline/graph showing events in ingest order
- Columns or lanes for: metric values, scaling intent, actual scaling outcomes
- Visual indication of run boundaries
- No configuration controls — read-only

### Event query API

Endpoints (defined in the contract) to query events for the UI or tests:
- Filter by run, sequence range, event kind

## Requirements

- SHALL assign ingest timestamps and sequence numbers — sender timestamps are informational only
- SHALL NOT control or influence any other component
- SHALL consume Nomad event stream directly (not via other components)
- SHALL handle missing or delayed events gracefully (sparse timelines are OK)
- SHALL support at least one active run in memory
- SHALL NOT persist data across restarts
- SHALL degrade gracefully if Holodeck or plugins are not yet running

## Non-Goals

- Unified dashboard for all components
- Prometheus-compatible metrics endpoint
- Multi-run concurrency or history beyond the current run
- Autoscaler activity sinks or tracing

## Acceptance Criteria

- `GET /v1/health` returns 200
- A Holodeck reset event appears in the Observer with an ingest timestamp
- A metric observation from the APM plugin appears in the Observer in sequence after the reset
- A Nomad scaling event from the event stream appears in the Observer
- The read-only UI loads in a browser and shows at least the above three events in order
