# Spec: nodesim-target Plugin

## Context

The `nodesim-target` plugin is a custom target plugin for the Nomad Autoscaler. It translates the autoscaler's scaling intent for node groups into calls to the nodesim ASG HTTP API.

It also emits scaling intent events to the Observer so the event chain can be correlated.

See [`plan.md`](../plan.md) §Autoscaler for the full description.

## Depends On

- Phase 0: [`specs/repo-bootstrap.md`](../repo-bootstrap.md) — directory layout must exist
- Phase 1: [`specs/contracts/nodesim-asg-api.md`](../contracts/nodesim-asg-api.md) — call this API
- Phase 1: [`specs/contracts/observer-http-api.md`](../contracts/observer-http-api.md) — emit intent to this API
- Phase 1: [`specs/contracts/plugin-interfaces.md`](../contracts/plugin-interfaces.md) — implement the target plugin interface

The nodesim ASG extension ([`specs/nodesim-asg.md`](./nodesim-asg.md)) must be running for integration testing, but the plugin can be developed and unit-tested independently using mocks/stubs.

## Repositories and Packages

- Repo: `gulducat/autoscaler-holodeck`
- Package: `plugins/nodesim-target/`
- Module: `github.com/gulducat/autoscaler-holodeck/plugins/nodesim-target`
- Binary: `plugins/nodesim-target/main.go`

Reference an existing target plugin for the binary bootstrap pattern:
`hashicorp/nomad-autoscaler/plugins/builtin/target/nomad/`

## What to Build

### Target plugin binary

A standalone Go binary that implements the autoscaler target plugin interface (defined in [`specs/contracts/plugin-interfaces.md`](../contracts/plugin-interfaces.md)).

Configuration (passed by the autoscaler via plugin config):
- `nodesim_address` — URL of the nodesim service
- `observer_address` — URL of the Observer service (optional)

### Get scale

When the autoscaler calls the plugin to get current scale:
1. Call `GET /v1/groups/<name>` on nodesim
2. Return the current node count as the target count

### Set scale

When the autoscaler calls the plugin to set scale:
1. Emit a `scale_intent` event to the Observer (before calling nodesim)
2. Call `POST /v1/groups/<name>/scale` on nodesim with the desired count

### Observer emission

Emit to the Observer when scaling intent is received. Include:
- The group name
- The desired count
- The current count (from the get-scale call)

Use best-effort fire-and-forget.

## Requirements

- SHALL implement the autoscaler target plugin interface exactly
- SHALL translate scaling intent into nodesim ASG API calls
- SHALL emit scaling intent events to the Observer before acting
- SHALL NOT contain any policy logic
- SHALL degrade gracefully if the Observer is unavailable

## Acceptance Criteria

- Plugin binary starts and registers with the autoscaler without error
- Autoscaler can read current node count via the plugin
- Autoscaler scaling intent results in a nodesim group size change
- The scaling intent event appears in the Observer before the nodesim call completes
