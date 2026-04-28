# Contract: Holodeck HTTP API

**Status:** needs definition (Phase 1)

This document defines the HTTP API that the Holodeck service exposes. It is the source of truth for:
- What the `holodeck-apm` plugin queries
- What the authoring UI and any test scripts write

Implementors of both the Holodeck service and the `holodeck-apm` plugin must agree on this spec before writing code.

---

## Base

`http://holodeck:8080` (address is configurable)

---

## Endpoints to Define

The following are placeholders. The Holodeck and holodeck-apm implementors should flesh these out before Phase 2 begins.

### Metric Query

Used by the APM plugin to retrieve current metric values.

```
GET /v1/metrics?job=<job>&group=<group>&metric=<name>
```

Response shape TBD. Should return a numeric value and a timestamp.

### World Authoring

Used by the UI and test scripts to configure metric rules.

```
PUT /v1/world
```

Request/response shape TBD. Should set metric rules (authored values, capacity-coupled functions, lag, saturation).

### Reset

Used to reset world state and signal a new run to the Observer.

```
POST /v1/reset
```

### Status / Health

```
GET /v1/health
```

---

## Notes

- Authored metrics are static values. Capacity-coupled metrics are functions of alloc/node count.
- The Holodeck does not store history — the Observer does.
- Metric query semantics should be simple enough that the APM plugin does not need to implement query parsing.

---

## Action Required

Before Phase 2 starts, replace the placeholder sections above with exact request/response shapes. Commit this file to `main` so all implementors share the same definition.
