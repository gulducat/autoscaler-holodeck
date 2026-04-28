# Spec: Nomad Jobs

## Context

This work stream produces the Nomad job definitions (and optionally a nomad-pack) needed to run the full system locally for development and testing.

The jobs run all the custom components — Holodeck, Observer, the autoscaler with custom plugins, and nodesim with the ASG extension. A real Nomad server/client is assumed to already be running.

See [`plan.md`](../plan.md) §Nomad Control Plane and §Repositories for the full description.

## Depends On

- Phase 0: [`specs/repo-bootstrap.md`](../repo-bootstrap.md) — `jobs/` directory must exist
- Phase 2: all component specs — jobs reference binaries produced by the other work streams

The job files can be written before the binaries exist (reference the expected binary paths), but they cannot be tested until the binaries are built.

## Repositories and Packages

- Repo: `gulducat/autoscaler-holodeck`
- Directory: `jobs/` at repo root (no Go module)

## What to Build

### Job files

One Nomad job file per service:

| File | Service |
|---|---|
| `jobs/holodeck.nomad.hcl` | Holodeck service |
| `jobs/observer.nomad.hcl` | Observer service |
| `jobs/autoscaler.nomad.hcl` | Nomad Autoscaler + plugin config |
| `jobs/nodesim.nomad.hcl` | nodesim with ASG extension |

Each job should:
- Use `type = "service"` 
- Define the binary path and any required config (addresses for other services)
- Expose the service's HTTP port via Nomad service registration
- Use Nomad service discovery for inter-service addresses where practical

### Autoscaler configuration

The autoscaler job needs:
- Plugin config for `holodeck-apm` (pointing at Holodeck)
- Plugin config for `nodesim-target` (pointing at nodesim)
- A sample policy that exercises the full loop (job scaling via holodeck-apm)

### Makefile targets

Add to the top-level `Makefile`:
- `make run` — submit all jobs to a local Nomad instance
- `make stop` — stop all jobs

### Optional: nomad-pack

If the job files share significant common config, consider wrapping them in a [nomad-pack](https://github.com/hashicorp/nomad-pack) so variables (addresses, binary paths) are in one place. This is optional — plain job files are acceptable.

## Requirements

- SHALL define jobs for all four services
- SHALL include a sample autoscaler policy that exercises the full loop
- SHALL work against a local Nomad dev agent (`nomad agent -dev`)
- SHALL NOT require Consul or Vault

## Acceptance Criteria

- All four jobs can be submitted to a local Nomad instance without errors
- The autoscaler successfully loads both custom plugins
- The autoscaler evaluates the sample policy using the holodeck-apm plugin
- The full system can be started and stopped with `make run` / `make stop`
