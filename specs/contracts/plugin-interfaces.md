# Contract: Autoscaler Plugin Interfaces

**Status:** defined ✅

This document records the Go interfaces from `hashicorp/nomad-autoscaler` that the `holodeck-apm` and `nodesim-target` plugins must implement. Source of truth is the autoscaler repo; this file is a stable reference for plugin implementors.

---

## Source Paths (for reference)

- [`plugins/base/base.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/base/base.go) — shared base interface
- [`plugins/apm/apm.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/apm/apm.go) — APM plugin interface
- [`plugins/target/target.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/target/target.go) — target plugin interface
- [`sdk/apm.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/apm.go) — `TimeRange`, `TimestampedMetrics`
- [`sdk/strategy.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/strategy.go) — `ScalingAction`
- [`sdk/target.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/target.go) — `TargetStatus`, `TargetScalingNoOpError`
- [`sdk/plugin.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/plugin.go) — plugin type constants
- [`plugins/plugin.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/plugin.go) — `Serve`, `Handshake`

---

## Base Interface

All plugins must implement `base.Base` ([`plugins/base/base.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/base/base.go)):

```go
type Base interface {
    // PluginInfo returns name and type. Used during setup and lifecycle.
    PluginInfo() (*PluginInfo, error)

    // SetConfig delivers plugin-specific config from the autoscaler.
    // Failure is terminal for the plugin.
    SetConfig(config map[string]string) error
}

type PluginInfo struct {
    Name       string
    PluginType string // use sdk.PluginTypeAPM or sdk.PluginTypeTarget
}
```

---

## APM Plugin Interface

The `holodeck-apm` plugin must implement `apm.APM` ([`plugins/apm/apm.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/apm/apm.go)):

```go
type APM interface {
    base.Base

    // Query returns timestamped metrics for the given query and time range.
    Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error)

    // QueryMultiple returns multiple metric series. Used by Dynamic Application Sizing.
    QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error)
}
```

### Relevant SDK types

From [`sdk/apm.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/apm.go):

```go
type TimeRange struct {
    From time.Time
    To   time.Time
}

type TimestampedMetric struct {
    Timestamp time.Time
    Value     float64
}

type TimestampedMetrics []TimestampedMetric
```

---

## Target Plugin Interface

The `nodesim-target` plugin must implement `target.Target` ([`plugins/target/target.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/target/target.go)):

```go
type Target interface {
    base.Base

    // Scale enacts a scaling action on the target.
    Scale(action sdk.ScalingAction, config map[string]string) error

    // Status returns current state of the target, including whether it is
    // ready to be scaled and its current count.
    Status(config map[string]string) (*sdk.TargetStatus, error)
}
```

### Relevant SDK types

From [`sdk/strategy.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/strategy.go):

```go
type ScalingAction struct {
    Count     int64          // desired count; see dry-run note below
    Reason    string
    Error     bool
    Direction ScaleDirection // ScaleDirectionDown(-1), ScaleDirectionNone(0), ScaleDirectionUp(1)
    Meta      map[string]any
}
```

From [`sdk/target.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/target.go):

```go
type TargetStatus struct {
    Ready bool              // false = autoscaler will not scale
    Count int64             // current target count
    Meta  map[string]string // optional; use TargetStatusMetaKeyLastEvent for last scaling event timestamp
}
```

### Dry-run requirement

**Every `Scale()` implementation must handle dry-run explicitly.**

The autoscaler sets `Count` to `-1` (`sdk.StrategyActionMetaValueDryRunCount`) during dry-run evaluations. A plugin that does not check this will make real scaling calls during dry-runs.

```go
func (p *Plugin) Scale(action sdk.ScalingAction, config map[string]string) error {
    if action.Count == sdk.StrategyActionMetaValueDryRunCount {
        return nil // no-op: do not call nodesim
    }
    // ...
}
```

### No-op scaling

If a scale request results in no action (e.g. already at desired count), return a `TargetScalingNoOpError` instead of `nil`. This signals the autoscaler to skip cooldown for this cycle. See [`sdk/target.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/sdk/target.go).

```go
return sdk.NewTargetScalingNoOpError("already at desired count %d", count)
```

---

## Plugin Binary Convention

Autoscaler plugins are **standalone Go binaries** communicating with the autoscaler over gRPC via [go-plugin](https://github.com/hashicorp/go-plugin).

### go-plugin handshake

From [`plugins/plugin.go`](https://github.com/hashicorp/nomad-autoscaler/blob/main/plugins/plugin.go) — values must match exactly:

```go
Handshake = plugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "NOMAD_AUTOSCALER_PLUGIN_MAGIC_COOKIE",
    MagicCookieValue: "e082fa04d587a6525d683666fa253d6afda00f20c122c54a80a3ed57fec99ff3",
}
```

### main.go bootstrap pattern

```go
package main

import (
    hclog "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/nomad-autoscaler/plugins"
    myplugin "github.com/gulducat/autoscaler-holodeck/plugins/<name>/plugin"
)

func main() {
    plugins.Serve(factory)
}

func factory(log hclog.Logger) interface{} {
    return myplugin.New(log)
}
```

`plugins.Serve` handles gRPC server setup and plugin type dispatch automatically.

Reference implementations in the autoscaler repo:
- Target: [`plugins/builtin/target/aws-asg/`](https://github.com/hashicorp/nomad-autoscaler/tree/main/plugins/builtin/target/aws-asg)
- APM: [`plugins/builtin/apm/prometheus/`](https://github.com/hashicorp/nomad-autoscaler/tree/main/plugins/builtin/apm/prometheus)

---

## Phase 1 Deliverables

In addition to this document, Phase 1 must commit Go stub files for each plugin. These are empty struct implementations that satisfy the interfaces and compile cleanly — no business logic.

### `plugins/nodesim-target/plugin/plugin.go`

```go
package plugin

import (
    hclog "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/nomad-autoscaler/plugins/base"
    "github.com/hashicorp/nomad-autoscaler/plugins/target"
    "github.com/hashicorp/nomad-autoscaler/sdk"
)

// Ensure Plugin implements target.Target at compile time.
var _ target.Target = (*Plugin)(nil)

type Plugin struct {
    logger hclog.Logger
    config map[string]string
}

func New(log hclog.Logger) *Plugin {
    return &Plugin{logger: log}
}

func (p *Plugin) PluginInfo() (*base.PluginInfo, error)                          { panic("not implemented") }
func (p *Plugin) SetConfig(config map[string]string) error                       { panic("not implemented") }
func (p *Plugin) Scale(action sdk.ScalingAction, config map[string]string) error { panic("not implemented") }
func (p *Plugin) Status(config map[string]string) (*sdk.TargetStatus, error)     { panic("not implemented") }
```

### `plugins/holodeck-apm/plugin/plugin.go`

```go
package plugin

import (
    hclog "github.com/hashicorp/go-hclog"
    "github.com/hashicorp/nomad-autoscaler/plugins/apm"
    "github.com/hashicorp/nomad-autoscaler/plugins/base"
    "github.com/hashicorp/nomad-autoscaler/sdk"
)

// Ensure Plugin implements apm.APM at compile time.
var _ apm.APM = (*Plugin)(nil)

type Plugin struct {
    logger hclog.Logger
    config map[string]string
}

func New(log hclog.Logger) *Plugin {
    return &Plugin{logger: log}
}

func (p *Plugin) PluginInfo() (*base.PluginInfo, error)                                                 { panic("not implemented") }
func (p *Plugin) SetConfig(config map[string]string) error                                              { panic("not implemented") }
func (p *Plugin) Query(query string, timeRange sdk.TimeRange) (sdk.TimestampedMetrics, error)           { panic("not implemented") }
func (p *Plugin) QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error) { panic("not implemented") }
```
