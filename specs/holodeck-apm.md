# Spec: holodeck-apm Plugin

## Context

The `holodeck-apm` plugin is a custom APM (Application Performance Monitoring) plugin for the Nomad Autoscaler. It connects the autoscaler to the Holodeck service, allowing the autoscaler to read the authored metrics as if they were real APM data.

It also emits metric observations to the Observer so the event chain can be correlated.

The autoscaler core is treated as a **black box** — this plugin is its only interface to the Holodeck.

See [`plan.md`](../plan.md) §Autoscaler for the full description.

## Depends On

- Phase 0: [`specs/repo-bootstrap.md`](../repo-bootstrap.md) — directory layout must exist
- Phase 1: [`specs/contracts/holodeck-http-api.md`](../contracts/holodeck-http-api.md) — query this API
- Phase 1: [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md) — emit observations to this API
- Phase 1: [`specs/contracts/plugin-interfaces.md`](../contracts/plugin-interfaces.md) — implement the APM plugin interface

## Repositories and Packages

- Repo: `gulducat/autoscaler-holodeck`
- Package: `plugins/holodeck-apm/`
- Module: `github.com/gulducat/autoscaler-holodeck/plugins/holodeck-apm`
- Binary: `plugins/holodeck-apm/main.go`

Reference an existing APM plugin for the binary bootstrap pattern:
`hashicorp/nomad-autoscaler/plugins/builtin/apm/prometheus/`

## What to Build

### APM plugin binary

A standalone Go binary that implements the autoscaler APM plugin interface (defined in [`specs/contracts/plugin-interfaces.md`](../contracts/plugin-interfaces.md)).

Configuration (passed by the autoscaler via plugin config):
- `holodeck_address` — URL of the Holodeck service
- `observer_address` — URL of the Observer service (optional; omit to disable observation)

### Query handler

When the autoscaler calls the plugin to query a metric:
1. Call the Holodeck metric query endpoint
2. Return the value in the format expected by the autoscaler APM interface
3. Emit a `metric_observation` event to the Observer (best-effort, non-blocking)

### Observer emission

Emit to the Observer on each metric query. Include:
- The metric name/selector
- The returned value
- The query timestamp

Use best-effort fire-and-forget — do not fail the metric query if the Observer is unavailable.

## Requirements

- SHALL implement the autoscaler APM plugin interface exactly
- SHALL query Holodeck for metric values
- SHALL emit metric observations to the Observer
- SHALL NOT implement any scaling policy logic
- SHALL NOT store metric history
- SHALL degrade gracefully if the Observer is unavailable

## Acceptance Criteria

- Plugin binary starts and registers with the autoscaler without error
- Autoscaler successfully queries a metric via the plugin
- The queried value matches what Holodeck reports
- The observation appears in the Observer
