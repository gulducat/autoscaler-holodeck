# Spec: Holodeck Service

## Context

The Holodeck owns **world state and metric physics**. It is the authoritative source of metric values that the autoscaler reads. It does not observe or control the autoscaler — it only authors the reality the autoscaler operates in.

See [`plan.md`](../plan.md) §Holodeck for the full description.

## Depends On

- Phase 0: [`specs/repo-bootstrap.md`](../repo-bootstrap.md) — directory layout and Go workspace must exist
- Phase 1: [`specs/contracts/holodeck-http-api.md`](../contracts/holodeck-http-api.md) — implement the API defined there
- Phase 1: [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md) — Holodeck reports world events to Observer

## Repositories and Packages

- Repo: `gulducat/autoscaler-holodeck`
- Package: `holodeck/` at repo root
- Module: `github.com/gulducat/autoscaler-holodeck/holodeck`
- Binary: `holodeck/cmd/holodeck/main.go`

## What to Build

### World state

A data model representing the current authored state:
- Named metrics with their current values
- Metric type: authored (static) or capacity-coupled (function of alloc/node count)
- Optional per-metric modifiers: lag, saturation, diminishing returns

World state is mutable via the authoring HTTP API.

### Metric evolution

A background loop that recalculates capacity-coupled metric values as Nomad state changes. Nomad state (alloc count, node count) is polled from the Nomad API.

Only the Nomad API is needed — do not embed a Nomad agent.

### HTTP API

Implement the endpoints defined in [`specs/contracts/holodeck-http-api.md`](../contracts/holodeck-http-api.md):
- Metric query endpoint (consumed by holodeck-apm plugin)
- World authoring endpoint (consumed by UI and test scripts)
- Reset endpoint (triggers a new run)
- Health endpoint

### Observer reporting

When world-authoring events occur (world reset, metric rule change), send an event to the Observer using the ingest API defined in [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md). Use a best-effort fire-and-forget approach — the Holodeck should not fail if the Observer is unavailable.

### Minimal JS UI

A single-page UI (plain HTML + JS, no build step required) served by the Go binary that allows:
- Viewing current world state
- Authoring metric rules (authored values, capacity-coupling parameters)
- Triggering a reset

This is a debugging/authoring tool, not a monitoring dashboard. Keep it minimal.

## Requirements

- SHALL serve metric queries synchronously on the query endpoint
- SHALL recalculate capacity-coupled metrics when Nomad alloc/node counts change
- SHALL emit world-authoring events to the Observer
- SHALL NOT store metric history — that is the Observer's job
- SHALL NOT implement Prometheus query language or compatible query interface
- SHALL support running without an Observer (degrade gracefully)
- SHALL support running without Nomad initially connected (start up cleanly, retry)

## Non-Goals

- Prometheus-compatible query engine
- Metric history or time-series storage
- Autoscaler policy logic of any kind

## Acceptance Criteria

- `GET /v1/health` returns 200
- An authored static metric returns the configured value on query
- A capacity-coupled metric returns a value that changes as Nomad alloc count changes
- A world reset emits an event visible in the Observer
- The minimal UI loads in a browser and allows authoring a metric rule
