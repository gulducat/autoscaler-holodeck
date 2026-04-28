# Spec: Nodesim ASG Extension

## Context

`nomad-nodesim` is an existing project that simulates Nomad nodes. This work stream extends it with an **ASG (auto-scaling group) concept** — a logical node group that can be scaled to a target count via an HTTP API.

This is the component the `nodesim-target` plugin calls. It has no policy logic — it only responds to imperative scale commands.

See [`plan.md`](../plan.md) §Capacity Simulation for the full description.

## Depends On

- Phase 1: [`specs/contracts/nodesim-asg-api.md`](../contracts/nodesim-asg-api.md) — implement the API defined there

## Repositories and Packages

- Repo: `hashicorp/nomad-nodesim` (local: `~/code/hashicorp/nomad-nodesim`)
- This is a **separate repo** from `autoscaler-holodeck`
- Work in a feature branch on `nomad-nodesim`; open a PR there

## What to Build

### Group model

A logical node group with:
- A name
- A desired count
- A mapping to Nomad constructs (datacenter, pool, or node meta — choose one approach and document it)
- The set of currently registered simulated nodes belonging to the group

### HTTP API

Implement the endpoints defined in [`specs/contracts/nodesim-asg-api.md`](../contracts/nodesim-asg-api.md):
- `POST /v1/groups/<name>/scale` — set desired count
- `GET /v1/groups/<name>` — get current state
- `GET /v1/groups` — list all groups
- `GET /v1/health`

### Scale reconciliation

A background loop that reconciles actual Nomad node registrations with the desired count:
- If desired > current: register additional simulated nodes in Nomad (using existing nodesim mechanisms)
- If desired < current: deregister simulated nodes from Nomad

Reconciliation is **deterministic** — same desired count always produces the same node set (stable node IDs by index within the group).

### Configuration

Groups should be definable in the existing nodesim configuration format (or an extension of it). An operator should be able to pre-define groups at startup.

## Requirements

- SHALL implement the ASG HTTP API defined in the contract spec
- SHALL add/remove Nomad nodes deterministically when desired count changes
- SHALL map groups to Nomad constructs consistently (document the chosen approach)
- SHALL NOT contain any scaling policy logic
- SHALL work with the existing nodesim node simulation mechanisms (do not replace them)

## Non-Goals

- Cloud-accurate ASG behavior
- Multi-region or multi-datacenter groups
- Health-check-based replacement

## Acceptance Criteria

- `GET /v1/health` returns 200
- Creating a group and scaling it to N results in N nodes registered in Nomad
- Scaling down removes nodes from Nomad
- The same desired count always produces the same node IDs (deterministic)
- A nodesim-target plugin call to scale results in the correct node count in Nomad
