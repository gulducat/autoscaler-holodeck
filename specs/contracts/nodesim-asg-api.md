# Contract: Nodesim ASG HTTP API

**Status:** needs definition (Phase 1)

This document defines the HTTP API that the extended `nomad-nodesim` service will expose for the ASG (auto-scaling group) concept. It is the source of truth for:
- What the `nodesim-target` autoscaler plugin calls

---

## Base

`http://nodesim:8082` (address is configurable)

---

## Endpoints to Define

### Ensure Group Size

Instructs nodesim to ensure a logical node group contains exactly N nodes.

```
POST /v1/groups/<name>/scale
```

Request shape TBD. Should include at minimum:
- `count` — desired number of nodes in the group

Response should indicate the resulting state (accepted/current count).

### Get Group

```
GET /v1/groups/<name>
```

Returns current state of a node group (current count, pending changes).

### List Groups

```
GET /v1/groups
```

### Health

```
GET /v1/health
```

---

## Notes

- A "group" maps to a set of Nomad nodes identified by datacenter, pool, or node meta (TBD — nodesim-asg implementor decides).
- The API contains no policy logic — it is purely imperative ("make it N").
- The `nodesim-target` plugin translates autoscaler scaling intent into calls to this API.

---

## Action Required

Before Phase 2 starts, replace placeholder sections with exact request/response shapes, and document how groups map to Nomad constructs. Commit to `main`.
