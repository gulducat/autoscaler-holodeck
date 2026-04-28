# Autoscaler Holodeck — Execution Plan

> Design brief lives in [`plan.md`](./plan.md). Read it first.

---

## Goal of This Document

This is the *execution* companion to `plan.md`. It describes how the work is phased, what depends on what, and how to pick up a work stream independently.

A fresh session should be able to read `plan.md` + this file, look at the `specs/` directory, and immediately know what is available to work on and what needs to happen first.

---

## Work Structure

Work is broken into **specs** — self-contained documents under `specs/`. Each spec describes one component: what to build, which interfaces to use, and how to verify it works.

```
specs/
  contracts/               ← Phase 1: shared interface definitions (must land first)
    holodeck-http-api.md
    observer-http-api.md
    nodesim-asg-api.md
    plugin-interfaces.md
  repo-bootstrap.md        ← Phase 0: foundational repo structure
  holodeck.md              ← Phase 2 (all parallel once Phase 1 is merged)
  observer.md
  holodeck-apm.md
  nodesim-target.md
  nodesim-asg.md
  nomad-jobs.md
```

### Rules for independent work

1. **One spec per branch.** Scope your branch to the component in the spec you picked up.
2. **Read `plan.md` for invariants.** Especially the non-goals and design invariants sections.
3. **Read the contract specs your component depends on** before writing any implementation code.
4. **Do not change contract specs during implementation.** Raise a PR discussion instead.
5. **Do not touch the autoscaler core.** Plugins are standalone binaries.

---

## Phases

### Phase 0 — Repo Bootstrap

**Spec:** [`specs/repo-bootstrap.md`](./specs/repo-bootstrap.md)
**Status:** ready to start
**Blocks:** everything

Sets up the Go workspace, directory layout, Makefile, and `copilot-setup-steps.yml`. Must land on `main` before other work begins, since it defines where things live.

---

### Phase 1 — Interface Contracts

**Specs:** [`specs/contracts/`](./specs/contracts/)
**Status:** ready to start (after Phase 0)
**Blocks:** all Phase 2 work

These specs define the HTTP APIs and Go interfaces that components will implement or consume. They produce committed Markdown and Go interface stubs — no working implementations yet. They must be reviewed and merged before parallel Phase 2 work begins.

| Contract | Author | Consumers |
|---|---|---|
| [`holodeck-http-api.md`](./specs/contracts/holodeck-http-api.md) | holodeck implementor | holodeck-apm plugin |
| [`observer-http-api.md`](./specs/contracts/observer-http-api.md) | observer implementor | holodeck, holodeck-apm, nodesim-target |
| [`nodesim-asg-api.md`](./specs/contracts/nodesim-asg-api.md) | nodesim-asg implementor | nodesim-target plugin |
| [`plugin-interfaces.md`](./specs/contracts/plugin-interfaces.md) | any | holodeck-apm, nodesim-target |

Phase 1 can be worked in parallel with Phase 0 since the contracts don't require code yet — but both must land before Phase 2 starts.

---

### Phase 2 — Parallel Implementation

**Status:** blocked on Phase 0 + Phase 1
**All six streams are independent once contracts are merged.**

| Spec | Repo | What it builds |
|---|---|---|
| [`specs/holodeck.md`](./specs/holodeck.md) | this repo | metric physics engine + HTTP API + minimal UI |
| [`specs/observer.md`](./specs/observer.md) | this repo | event sink, ordering, read-only UI |
| [`specs/holodeck-apm.md`](./specs/holodeck-apm.md) | this repo | autoscaler APM plugin |
| [`specs/nodesim-target.md`](./specs/nodesim-target.md) | this repo | autoscaler target plugin for node groups |
| [`specs/nodesim-asg.md`](./specs/nodesim-asg.md) | `hashicorp/nomad-nodesim` | ASG concept + HTTP API extension |
| [`specs/nomad-jobs.md`](./specs/nomad-jobs.md) | this repo | Nomad job files to run the full system |

---

### Phase 3 — Integration

**Status:** blocked on Phase 2

Write one end-to-end scenario: author a metric in Holodeck → autoscaler scales a Nomad job → Observer records the full event chain. Add CI to run it.

No spec exists yet; it will be written once Phase 2 components are available.

---

## Relevant Repos

All under `~/code` locally:

| Repo | Purpose |
|---|---|
| `gulducat/autoscaler-holodeck` | this repo — all new code |
| `hashicorp/nomad-nodesim` | extend with ASG support (`specs/nodesim-asg.md`) |
| `hashicorp/nomad-autoscaler` | reference only — do not modify |
| `hashicorp/nomad` | reference only — do not modify |
