# Contract: Nodesim Node Group HTTP API

**Status:** defined ✅

This document defines the HTTP API that the extended `nomad-nodesim` service will expose for the node group concept. It is the source of truth for:
- What the `nodesim-target` autoscaler plugin calls
- What the `nodesim-asg` implementor must build

---

## Base URL

`http://nodesim:8082` (address is configurable via `nodesim_address` plugin config)

---

## Group → Nomad Construct Mapping

A node group maps to a set of Nomad nodes identified by **node pool**. The pool name is set via the `node_pool` field in the group's `node {}` config block — the group name and pool name are independent.

Node pool was chosen as the discriminator because:
- Membership is visible natively in the Nomad UI and API
- The autoscaler's existing `node_pool` target config key works without modification
- It is the closest Nomad analogue to a cloud ASG, which maps to a single instance type/pool

---

## Endpoints

### Scale Group

Instructs nodesim to ensure a node group contains exactly N nodes. Reconciliation is synchronous — the response reflects the state after reconciliation completes.

```
POST /v1/groups/{name}/scale
```

Request:

```json
{"count": 5}
```

Response `200`:

```json
{
  "name":          "my-group",
  "desired_count": 5,
  "current_count": 5
}
```

Response `400` if `count` is negative.
Response `404` if the group does not exist.

```sh
curl -s -X POST http://nodesim:8082/v1/groups/my-group/scale \
  -H 'Content-Type: application/json' \
  -d '{"count": 5}'
```

---

### Get Group

```
GET /v1/groups/{name}
```

Response `200`:

```json
{
  "name":          "my-group",
  "node_pool":     "web-nodes",
  "desired_count": 3,
  "current_count": 3,
  "ready":         true
}
```

`ready` is `true` when `current_count == desired_count`.
The `nodesim-target` plugin uses `current_count` for `Status().Count` and `ready` for `Status().Ready`.

Response `404` if the group does not exist.

```sh
curl -s http://nodesim:8082/v1/groups/my-group
```

---

### List Groups

```
GET /v1/groups
```

Response `200`: array of group objects (same shape as Get Group).

```sh
curl -s http://nodesim:8082/v1/groups
```

---

### Health

```
GET /v1/health
```

Response `200`:

```json
{"status": "ok"}
```

```sh
curl -s http://nodesim:8082/v1/health
```

---

## Notes

- The API contains no policy logic — it is purely imperative ("make it N").
- The `nodesim-target` plugin translates autoscaler scaling intent into calls to this API.
- Groups must be pre-declared in nodesim configuration; they are not created on demand via the API.
- Node naming is deterministic: `<group_name>-<index>` (e.g. `web-0`, `web-1`).
