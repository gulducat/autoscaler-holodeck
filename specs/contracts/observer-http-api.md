# Contract: Observer HTTP API

**Status:** needs definition (Phase 1)

This document defines the HTTP API that the Observer service exposes. It is the source of truth for:
- Event ingest endpoints (used by Holodeck, holodeck-apm plugin, nodesim-target plugin)
- Read/query endpoints (used by the Observer UI)

---

## Base

`http://observer:8081` (address is configurable)

---

## Endpoints to Define

### Event Ingest

Receives world-authoring events from Holodeck and metric/intent observations from plugins.

```
POST /v1/events
```

Request shape TBD. Should include at minimum:
- `source` — which component sent the event (holodeck, apm-plugin, target-plugin)
- `kind` — event type (e.g., `metric_observation`, `scale_intent`, `world_reset`)
- `payload` — event-specific data
- `sent_at` — sender's wall-clock timestamp (informational only)

The Observer assigns its own ingest timestamp and sequence number — sender timestamps are not authoritative.

### Event Query (for UI)

```
GET /v1/events?run=<id>&since=<seq>&kind=<kind>
```

Response shape TBD. Returns ordered events for display in the read-only UI.

### Health

```
GET /v1/health
```

---

## Notes

- The Observer assigns total ordering by ingest sequence — not wall-clock time.
- Run boundaries are defined by `world_reset` events from Holodeck.
- The Observer does not drive or control anything — it witnesses.

---

## Action Required

Before Phase 2 starts, replace the placeholder sections above with exact request/response shapes. Commit this file to `main`.
