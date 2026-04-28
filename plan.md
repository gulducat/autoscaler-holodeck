# Autoscaler Holodeck — Agent Brief

## Goal

Build a small, controlled system to exercise **core Nomad Autoscaler behavior** (policy math, convergence, stability), for hands‑on exploration and e2e / CI tests.

This is **not** a production simulator, Prometheus clone, or performance benchmark.

**Core idea:**

> Author a simple, explicit “universe,” let the autoscaler react, and observe what actually happens.

***

## Core Loop

**Authored reality → autoscaler belief → scaling intent → Nomad execution → observed outcomes → feedback**

Correctness depends on **event ordering**, not precise time durations.

***

## Components

### 1. Holodeck

*   Go service
*   Owns **world state and metric physics**
*   Supports:
    *   Authored metrics (static values)
    *   Capacity‑coupled metrics (functions of alloc/node count)
    *   Optional lag, saturation, diminishing returns
*   Evolves metrics over time and incorporates feedback from Nomad state
*   Exposes:
    *   Metric query interface (used by autoscaler APM plugin)
    *   HTTP API + minimal JS UI for configuration
*   Reports **world‑authoring events** (model/parameter changes, resets) **directly to the Observer**

The Holodeck authors *rules*, not dashboards.

***

### 2. Autoscaler

*   Existing autoscaler core remains **unchanged**
*   **Custom APM plugin**
    *   Queries Holodeck
    *   Emits metric observations to Observer
*   **Target plugins**
    *   Use existing Nomad **job scaling target** as‑is
    *   Custom target plugin only for **node groups** (nodesim)
*   Autoscaler is treated as a **black box**:
    *   Observed via inputs, intent, logs, and outcomes

***

### 3. Capacity Simulation (nodesim)

*   Implements **logical node groups** (cloud ASG analogue)
*   Exposes small HTTP API: “ensure group size = N”
*   Projects groups onto Nomad constructs (dc / pool / meta)
*   Adds/removes nodes deterministically
*   Contains **no policy logic**

***

### 4. Nomad Control Plane

*   Real Nomad server + client
*   Executes job scaling via standard Nomad mechanisms
*   Registers/deregisters nodes provided by nodesim
*   Emits authoritative job / allocation / node events

***

### 5. Observer

*   Central **event sink and correlator**
*   Receives:
    *   Holodeck world‑authoring events
    *   APM metric observations
    *   Target scaling intent
*   Listens to Nomad’s event stream for actual outcomes
*   Assigns:
    *   Ingest timestamp (wall clock, authoritative for UI)
    *   Total ordering / sequence
*   Provides a **read‑only UI**:
    *   Timelines/graphs of metrics, intent, outcomes
    *   Shows *what actually happened*, not configuration

The Observer **witnesses**, it does not drive time or infer policy.

## Repositories

The below repos are located under ~/code. If you can't find them, ask the user for where to look instead.

### gulducat/autoscaler-holodeck

This repo, all new code.

*   nomad-autoscaler plugins
    *   holodeck-apm
    *   nodesim-target
*   holodeck
*   observer
*   nomad jobs to run all this
    * could be put together in a nomad-pack

### hashicorp/nomad-nodesim

Existing nodesim program to extend with ASG concept and associated HTTP API.

### hashicorp/nomad/api

Go SDK for Nomad API.

### hashicorp/nomad

Full Nomad repo for reference only.

### hashicorp/nomad-autoscaler

Full Autoscaler repo for reference only.

***

## Time Semantics

*   Multiple clocks exist; no global synchronization
*   Wall‑clock timestamps are for **visualization only**
*   Tests rely on **ordering**, not duration
*   Observer time is authoritative for UI graphs

***

## Runs / Experiments

*   Assume **one active run at a time**
*   Run boundaries are defined by the Observer (e.g., Holodeck reset/start events)
*   No requirement to propagate run IDs through autoscaler plugins

***

## Non‑Goals (Do Not Build)

*   Prometheus‑compatible query engine
*   Unified UI across components
*   Autoscaler core tracing or activity sinks
*   Multi‑run concurrency
*   Deterministic step‑driven simulation or fast‑forward time
*   Full realism or cloud‑accurate behavior

***

## Design Invariants

*   Autoscaler core stays untouched
*   Reality is **authored**, not emergent
*   Observation ≠ control
*   Ordering > duration
*   Add complexity only when it clearly improves insight

***

## Execution

See [`copilot-plan.md`](./copilot-plan.md) for the phased work breakdown, dependency order, and the index of work streams available to pick up.

