# Spec: Repo Bootstrap

## Context

This is Phase 0 — foundational scaffolding. It must land on `main` before any other work starts.

See [`plan.md`](../plan.md) for the full design brief.

## What to Build

### Directory layout

```
plugins/
  holodeck-apm/
    go.mod    # module github.com/gulducat/autoscaler-holodeck/plugins/holodeck-apm
  nodesim-target/
    go.mod    # module github.com/gulducat/autoscaler-holodeck/plugins/nodesim-target
holodeck/
  go.mod      # module github.com/gulducat/autoscaler-holodeck/holodeck
observer/
  go.mod      # module github.com/gulducat/autoscaler-holodeck/observer
jobs/         # Nomad job HCL files (no go.mod)
specs/        # already exists
```

### Go workspace

Create a `go.work` at the repo root covering all four Go modules above.

`nomad-nodesim` is a separate project modified under its own repo (`hashicorp/nomad-nodesim`). The `nodesim-target` plugin communicates with it over HTTP — no Go import relationship.

### Makefile

Top-level `Makefile` with at minimum:

- `make build` — build all binaries
- `make test` — run all tests
- `make lint` — run `golangci-lint` across all modules
- `make tidy` — run `go mod tidy` in each module

Each module directory may also have its own `Makefile` that the top-level delegates to.

### copilot-setup-steps.yml

Create `.github/copilot-setup-steps.yml` so GitHub Copilot coding agents have a working environment. Include:

- Install Go (match version in go.work)
- Install `golangci-lint`
- Install Nomad (latest stable) — needed for integration tests
- Run `go work sync` to pre-populate the module cache

### Directory stubs

Create empty `doc.go` or `.gitkeep` files in each package directory so the structure is visible in the repo. Do not write any business logic — leave that to the component specs.

## Acceptance Criteria

- `make build` runs without error (even if binaries are empty `main` packages)
- `make test` runs without error
- `go work sync` runs cleanly
- The directory layout matches the structure documented above
- `.github/copilot-setup-steps.yml` is present and valid YAML
