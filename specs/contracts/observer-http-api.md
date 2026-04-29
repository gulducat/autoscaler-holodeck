# Contract: Observer HTTP API

**Status:** defined ✅

This document defines the HTTP API that the Observer service exposes. It is the source of truth for:
- Event ingest endpoints (used by Holodeck, holodeck-apm plugin, nodesim-target plugin)
- Read/query endpoints (used by the Observer UI)

---

## Base URL

`http://observer:9090` (address is configurable via `observer_address` plugin config)

---

## Time and Ordering Model

- The Observer assigns a monotonically increasing **sequence number** and a **wall-clock ingest timestamp** to every event it receives.
- Sequence numbers are authoritative for ordering. Sender timestamps (`sent_at`) are informational only.
- A **run** begins when the Observer receives a `world_reset` event. The run's ID is the sequence number of that event. Events received before the first `world_reset` belong to run `0`.

---

## Event Kinds

| Kind | Sender | Description |
|---|---|---|
| `world_reset` | Holodeck | Marks the start of a new run |
| `metric_rule_change` | Holodeck | An authored metric rule was added or changed |
| `metric_observation` | holodeck-apm | The APM plugin observed a metric value |
| `scale_intent` | nodesim-target | The target plugin intends to scale a group |

Nomad job scaling, allocation, and node events are ingested directly by the Observer from the Nomad event stream — they are not pushed via this API.

---

## Endpoints

### Event Ingest

Receives world-authoring events from Holodeck and metric/intent observations from plugins.

```
POST /v1/events
```

**`world_reset`** — sent by Holodeck when a new run begins:

```sh
curl -s -X POST http://observer:9090/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "source":  "holodeck",
    "kind":    "world_reset",
    "sent_at": "2026-04-28T21:00:00Z",
    "payload": {}
  }'
```

Expected Response `202 Accepted`:

```json
{
  "seq":         1,
  "ingested_at": "2026-04-28T21:00:00.042Z",
  "run":         1
}
```

---

**`metric_rule_change`** — sent by Holodeck when a metric rule is added or changed:

```sh
curl -s -X POST http://observer:9090/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "source":  "holodeck",
    "kind":    "metric_rule_change",
    "sent_at": "2026-04-28T21:00:01Z",
    "payload": {
      "metric": "cpu_utilization",
      "value":  0.75
    }
  }'
```

Expected Response `202 Accepted`:

```json
{
  "seq":         2,
  "ingested_at": "2026-04-28T21:00:01.011Z",
  "run":         1
}
```

---

**`metric_observation`** — sent by the holodeck-apm plugin after each Query call:

```sh
curl -s -X POST http://observer:9090/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "source":  "holodeck-apm",
    "kind":    "metric_observation",
    "sent_at": "2026-04-28T21:00:05Z",
    "payload": {
      "query": "cpu_utilization",
      "value": 0.75
    }
  }'
```

Expected Response `202 Accepted`:

```json
{
  "seq":         3,
  "ingested_at": "2026-04-28T21:00:05.007Z",
  "run":         1
}
```

---

**`scale_intent`** — sent by the nodesim-target plugin before each Scale call:

```sh
curl -s -X POST http://observer:9090/v1/events \
  -H 'Content-Type: application/json' \
  -d '{
    "source":  "nodesim-target",
    "kind":    "scale_intent",
    "sent_at": "2026-04-28T21:00:06Z",
    "payload": {
      "group":         "my-group",
      "desired_count": 5,
      "current_count": 3
    }
  }'
```

Expected Response `202 Accepted`:

```json
{
  "seq":         4,
  "ingested_at": "2026-04-28T21:00:06.002Z",
  "run":         1
}
```

The Observer returns the assigned sequence number, ingest timestamp, and run ID so callers can confirm receipt.

---

### Event Query

Returns ordered events for display in the read-only UI or for use in tests.

```
GET /v1/events?run=<id>&since=<seq>&kind=<kind>
```

- `run` — run ID (sequence number of the opening `world_reset`); defaults to the current run
- `since` — return only events with `seq > since`; omit to return all events in the run
- `kind` — filter to a single event kind; omit to return all kinds

The UI calls this endpoint without a `run` parameter to get the current run and discovers the run ID from the `run` field of the response.

```sh
curl -s 'http://observer:9090/v1/events?since=2&kind=scale_intent'
```

Expected Response `200 OK`:

```json
{
  "run": 1,
  "events": [
    {
      "seq":         4,
      "run":         1,
      "ingested_at": "2026-04-28T21:00:06.002Z",
      "source":      "nodesim-target",
      "kind":        "scale_intent",
      "sent_at":     "2026-04-28T21:00:06Z",
      "payload":     {
        "group":         "my-group",
        "desired_count": 5,
        "current_count": 3
      }
    }
  ]
}
```

---

### Health

```
GET /v1/health
```

```sh
curl -s http://observer:9090/v1/health
```

Expected Response `200 OK`:

```json
{"status": "ok"}
```

---

## Notes

- The Observer assigns total ordering by ingest sequence — not wall-clock time.
- Run boundaries are defined by `world_reset` events from Holodeck.
- The Observer does not drive or control anything — it witnesses.
