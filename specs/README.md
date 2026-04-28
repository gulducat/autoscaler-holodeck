# Specs

Each file in this directory is a **self-contained work stream** for the Autoscaler Holodeck project.

Before reading any spec here, read [`plan.md`](../plan.md) (design brief) and [`copilot-plan.md`](../copilot-plan.md) (execution plan and phase ordering).

## Phase 0

| Spec | Status |
|---|---|
| [`repo-bootstrap.md`](./repo-bootstrap.md) | ready to start |

## Phase 1 — Interface Contracts (must land before Phase 2)

| Spec | Status |
|---|---|
| [`contracts/holodeck-http-api.md`](./contracts/holodeck-http-api.md) | needs definition |
| [`contracts/observer-http-api.md`](./contracts/observer-http-api.md) | needs definition |
| [`contracts/nodesim-asg-api.md`](./contracts/nodesim-asg-api.md) | needs definition |
| [`contracts/plugin-interfaces.md`](./contracts/plugin-interfaces.md) | needs definition |

## Phase 2 — Parallel Implementation

| Spec | Repo | Status |
|---|---|---|
| [`holodeck.md`](./holodeck.md) | this repo | blocked on Phase 1 |
| [`observer.md`](./observer.md) | this repo | blocked on Phase 1 |
| [`holodeck-apm.md`](./holodeck-apm.md) | this repo | blocked on Phase 1 |
| [`nodesim-target.md`](./nodesim-target.md) | this repo | blocked on Phase 1 |
| [`nodesim-asg.md`](./nodesim-asg.md) | `hashicorp/nomad-nodesim` | blocked on Phase 1 |
| [`nomad-jobs.md`](./nomad-jobs.md) | this repo | blocked on Phase 2 components |
