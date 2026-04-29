# Contract: Holodeck HTTP API

**Status:** defined ✅

This document defines the HTTP API that the Holodeck service exposes. It is the source of truth for:
- What the `holodeck-apm` plugin queries
- What the authoring UI and any test scripts write

Implementors of both the Holodeck service and the `holodeck-apm` plugin must not change this spec during implementation — raise a PR discussion instead.

---

## Base URL

`http://holodeck:8080` (address is configurable)

---

## World Model

The Holodeck supports multiple named **worlds**, each with independent metric rules and authored state. Worlds are identified by a string ID.

A **default world** (`id=default`) is created automatically on startup. Shorthand routes without a world ID operate on the default world.

---

## Metric Types

### Authored (static)

A fixed numeric value set directly by the caller.

```json
{ "type": "authored", "value": 0.75 }
```

### Capacity-coupled

A value computed from current Nomad alloc and/or node counts using a simple linear function:

```
value = base + (alloc_count * alloc_factor) + (node_count * node_factor)
```

Optionally clamped to a `[min, max]` range.

```json
{
  "type": "capacity_coupled",
  "base": 0.0,
  "alloc_factor": 0.05,
  "node_factor": 0.0,
  "min": 0.0,
  "max": 1.0
}
```

All fields except `type` are optional and default to `0` / unclamped.

### Modifiers (optional, applies to both types)

```json
{
  "lag_seconds": 5,
  "saturation": 0.95,
  "diminishing_returns_factor": 0.8
}
```

- `lag_seconds` — delay before a new value takes effect
- `saturation` — clamp output to this ceiling (applied after capacity function)
- `diminishing_returns_factor` — scale factor applied above 0.5 to simulate diminishing returns

All modifier fields are optional.

---

## Endpoints

### Health

```
GET /v1/health
```

```sh
curl -s http://holodeck:8080/v1/health
```

Response `200 OK`:

```json
{ "status": "ok" }
```

---

### Metric Query

Used by the `holodeck-apm` plugin to retrieve the current value of a named metric.

**World-scoped:**
```
GET /v1/worlds/{id}/metrics?metric=<name>
```

**Shorthand (defaults to world `default`):**
```
GET /v1/metrics?metric=<name>
```

Optional query parameters:
- `metric` — metric name (required)
- `job` — Nomad job name (informational; passed through to Observer event)
- `group` — Nomad task group name (informational; passed through to Observer event)

```sh
curl -s 'http://holodeck:8080/v1/metrics?metric=cpu_utilization&job=my-job&group=web'
```

Response `200 OK`:

```json
{
  "metric":    "cpu_utilization",
  "value":     0.75,
  "queried_at": "2026-04-29T14:00:00.000Z"
}
```

Response `404 Not Found` (metric not defined in the world):

```json
{ "error": "metric not found: cpu_utilization" }
```

---

### Get World State

Returns the current metric rules for a world.

**World-scoped:**
```
GET /v1/worlds/{id}
```

**Shorthand:**
```
GET /v1/world
```

```sh
curl -s http://holodeck:8080/v1/world
```

Response `200 OK`:

```json
{
  "id": "default",
  "metrics": {
    "cpu_utilization": {
      "type":  "authored",
      "value": 0.75
    },
    "queue_depth": {
      "type":         "capacity_coupled",
      "base":         10.0,
      "alloc_factor": 2.5,
      "node_factor":  0.0,
      "max":          1000.0
    }
  }
}
```

Response `404 Not Found` (world does not exist):

```json
{ "error": "world not found: my-world" }
```

---

### Set World State (Authoring)

Replaces the full set of metric rules for a world. Creates the world if it does not exist.

**World-scoped:**
```
PUT /v1/worlds/{id}
```

**Shorthand:**
```
PUT /v1/world
```

```sh
curl -s -X PUT http://holodeck:8080/v1/world \
  -H 'Content-Type: application/json' \
  -d '{
    "metrics": {
      "cpu_utilization": {
        "type":  "authored",
        "value": 0.75
      },
      "queue_depth": {
        "type":         "capacity_coupled",
        "base":         10.0,
        "alloc_factor": 2.5,
        "node_factor":  0.0,
        "max":          1000.0
      }
    }
  }'
```

Response `200 OK` — echoes back the full world state (same shape as `GET /v1/world`).

Each metric change emits a `metric_rule_change` event to the Observer (one event per changed metric).

---

### Reset World

Clears all metric rules for the world and emits a `world_reset` event to the Observer, signalling a new run.

**World-scoped:**
```
POST /v1/worlds/{id}/reset
```

**Shorthand:**
```
POST /v1/world/reset
```

```sh
curl -s -X POST http://holodeck:8080/v1/world/reset
```

Response `200 OK`:

```json
{ "id": "default", "reset": true }
```

---

### List Worlds

```
GET /v1/worlds
```

```sh
curl -s http://holodeck:8080/v1/worlds
```

Response `200 OK`:

```json
{
  "worlds": ["default", "load-test"]
}
```

---

## Notes

- The `default` world is created automatically at startup and is never deleted by reset — only its metrics are cleared.
- The Holodeck does not store metric history. History is the Observer's job.
- Metric query semantics are intentionally simple — the APM plugin does not implement query parsing.
- All Observer reporting is best-effort fire-and-forget. The Holodeck does not fail if the Observer is unreachable.
