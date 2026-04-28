# Contract: Autoscaler Plugin Interfaces

**Status:** needs definition (Phase 1)

This document records the Go interfaces from `hashicorp/nomad-autoscaler` that the holodeck-apm and nodesim-target plugins must implement. It serves as a quick reference so plugin implementors don't need to dig through the autoscaler source.

---

## Source

Plugin interfaces live in `hashicorp/nomad-autoscaler` under `plugins/`. Read that repo for the authoritative definitions; copy the relevant interfaces here once confirmed.

Key paths to check:
- `plugins/apm/` — APM plugin interface
- `plugins/target/` — target plugin interface
- `plugins/base/` — shared base interface
- `plugins/shared/` — shared plugin infrastructure

---

## APM Plugin Interface

*(To be filled in from `hashicorp/nomad-autoscaler/plugins/apm`)*

The `holodeck-apm` plugin must implement this interface.

Key methods expected:
- Query metrics by name/selector
- Return time-series or latest-value data

---

## Target Plugin Interface

*(To be filled in from `hashicorp/nomad-autoscaler/plugins/target`)*

The `nodesim-target` plugin must implement this interface.

Key methods expected:
- Get current scale (current node group size)
- Set scale (call nodesim ASG API)

---

## Plugin Binary Convention

Autoscaler plugins are standalone Go binaries. They communicate with the autoscaler via go-plugin (HashiCorp). The bootstrap pattern for a plugin binary should be copied from an existing builtin plugin.

Reference: `hashicorp/nomad-autoscaler/plugins/builtin/apm/prometheus/`

---

## Action Required

Before Phase 2 starts, copy the exact interface definitions from the autoscaler source into this file and note the go-plugin protocol version. Commit to `main`.
