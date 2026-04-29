# Spec: Nodesim Node Group Extension

## Context

`nomad-nodesim` is an existing project that simulates Nomad nodes. This work stream extends it with a **node group concept** — a named, dynamically-scalable set of simulated Nomad nodes managed via an HTTP API.

This is the component the `nodesim-target` plugin calls. It has no policy logic — it only responds to imperative scale commands.

See [`plan.md`](../plan.md) §Capacity Simulation for the full description.

## Depends On

- Phase 1: [`specs/contracts/nodesim-nodegroup-api.md`](../contracts/nodesim-nodegroup-api.md) — implement the API defined there

## Repositories and Packages

- Repo: `hashicorp/nomad-nodesim` (local: `~/code/hashicorp/nomad-nodesim`)
- This is a **separate repo** from `autoscaler-holodeck`
- Work in a feature branch on `nomad-nodesim`; open a PR there
- New packages: `nodegroup/` (manager + HTTP API), `internal/nodefactory/` (extracted node builder)

## Decisions

### NodeGroup, not ASG

The type and concept is called **NodeGroup** (Go: `NodeGroup`, `Manager`). It is not an "auto-scaling group" because nodes do not scale themselves — they respond to explicit commands from the autoscaler plugin.

### Nomad construct mapping

A node group maps to a **Nomad node pool**: the `node_pool` field on the group's `node {}` config block determines which pool the group's nodes join. Node pool membership is visible natively in the Nomad UI and API, and the autoscaler's `node_pool` target config key works without modification.

### Deterministic node identity

Nodes in a group are named `<group_name>-<index>` (e.g. `web-0`, `web-1`). Since Nomad derives node ID from the client state directory path, and the state directory is derived from the node name, the same index always produces the same node ID across restarts.

### Node factory extraction

`startClient()` is currently a private function in `main.go`. It must be extracted to `internal/nodefactory/` so both `main.go` (for flat `node_num` nodes) and the `nodegroup` manager can call it.

### `node_num` default fixed to 0

The existing default of `node_num = 1` prevents a groups-only config. The default is changed to `0`. The `Merge()` logic is fixed to allow explicitly setting `node_num = 0` (currently guarded by `if z.NodeNum > 0`).

## What to Build

### Config extension

New `group` labeled HCL block in `internal/config/config.go`:

```hcl
group "web" {
  count = 3

  node {
    node_pool = "web-nodes"
    resources {
      cpu_compute = 4000
      memory_mb   = 8000
    }
  }
}
```

- `count` — initial node count at startup (can be 0)
- `node {}` — optional; merged over the top-level `node {}` block. Supports all existing Node fields: `region`, `datacenter`, `node_pool`, `node_class`, `options`, `resources`
- Group name is decoupled from `node_pool` — set `node_pool` inside `node {}` explicitly

### Node factory

`internal/nodefactory/nodefactory.go`: extract and export `startClient()` as `Build(cfg *config.Config, buildInfo *simnode.BuildInfo, logger hclog.Logger, nodeName string) (*simnode.Node, error)`. `main.go` and `nodegroup.Manager` both use it.

### NodeGroup manager

`nodegroup/manager.go`:
- `NodeGroup` — name, desired count, `map[int]*simnode.Node`, mutex
- `Manager` — map of groups, base config, build info, logger
- `Manager.InitFromConfig` — pre-create groups from config, start `count` nodes each
- `Manager.Scale(name, count)` — reconcile: start nodes for new indices, shut down nodes for removed indices
- `Manager.Get(name)` — state snapshot
- `Manager.List()` — all groups

### HTTP API

`nodegroup/api.go` — serves on `NODESIM_ASG_ADDR` (default `:8082`):

```
GET  /v1/health
GET  /v1/groups
GET  /v1/groups/{name}
POST /v1/groups/{name}/scale  — body: {"count": N}
```

Response shapes match [`specs/contracts/nodesim-nodegroup-api.md`](../contracts/nodesim-nodegroup-api.md) exactly.
`nodes` reflects state at time of request; reconciliation is synchronous (Scale blocks until done).

### main.go changes

- Wire `Manager.InitFromConfig` after existing flat-node startup
- Start ASG HTTP server
- Graceful shutdown: stop HTTP server, call `Shutdown()` on all group nodes

## Requirements

- SHALL implement the node group HTTP API defined in the contract spec
- SHALL add/remove Nomad nodes deterministically when desired count changes
- SHALL map node group nodes to Nomad node pools via the `node {}` config block
- SHALL NOT contain any scaling policy logic
- SHALL work with the existing nodesim node simulation mechanisms (do not replace them)
- SHALL allow `node_num = 0` (or omitting it) for a groups-only config

## Non-Goals

- Cloud-accurate scaling behavior
- Multi-region or multi-datacenter groups
- Health-check-based node replacement
- On-demand group creation via the API (groups may be pre-declared in config OR created at runtime)

## Acceptance Criteria

- `GET /v1/health` returns 200
- A group with `count = 3` has 3 nodes registered in Nomad at startup
- `POST /v1/groups/{name}/scale` with `{"count": 5}` results in 5 nodes in Nomad
- Scaling down removes the highest-indexed nodes
- The same desired count always produces the same node IDs (deterministic)
- A config with no `node_num` (or `node_num = 0`) starts cleanly with only group nodes
