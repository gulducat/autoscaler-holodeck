package observer

import (
	"encoding/json"
	"strings"
	"time"
)

// NomadNodeEvent is the payload shape for nomad_node_* event kinds.
// These are emitted by the Observer's Nomad event stream listener when it
// receives Node topic events (e.g. nomad_node_noderegistration,
// nomad_node_nodederegistration, nomad_node_nodeeligibility).
type NomadNodeEvent struct {
	SentAt     *time.Time        `json:"sent_at,omitempty"`
	NodePool   string            `json:"NodePool"`
	Attributes map[string]string `json:"Attributes"`
	Events     json.RawMessage   `json:"Events"`
}

// NomadJobEvent is the payload shape for nomad_job_* event kinds.
// Emitted for Job topic events (e.g. nomad_job_jobregistered,
// nomad_job_jobderegistered, nomad_job_scalingevent).
type NomadJobEvent struct {
	SentAt      *time.Time `json:"sent_at,omitempty"`
	Name        string     `json:"Name"`
	Status      string     `json:"Status"`
	Count       int64      `json:"Count"`
	Priority    int        `json:"Priority"`
	Region      string     `json:"Region"`
	Datacenters []string   `json:"Datacenters"`
}

// NomadAllocationEvent is the payload shape for nomad_allocation_* event kinds.
// Emitted for Allocation topic events (e.g. nomad_allocation_allocationupdated,
// nomad_allocation_allocationcreated).
type NomadAllocationEvent struct {
	SentAt             *time.Time      `json:"sent_at,omitempty"`
	TaskGroup          string          `json:"TaskGroup"`
	DesiredStatus      string          `json:"DesiredStatus"`
	DeploymentStatus   json.RawMessage `json:"DeploymentStatus"`
	ClientStatus       string          `json:"ClientStatus"`
	AllocatedResources json.RawMessage `json:"AllocatedResources"`
}

// ScaleIntentEvent is the payload shape for scale_intent events.
// Sent by the nodesim-target plugin before each Scale call.
type ScaleIntentEvent struct {
	SentAt       *time.Time `json:"sent_at,omitempty"`
	Group        string     `json:"group"`
	DesiredCount int64      `json:"desired_count"`
	CurrentCount int64      `json:"current_count"`
}

// RuleChangeEvent is the payload shape for metric_rule_change events.
// Sent by Holodeck when a metric rule is added or changed.
type RuleChangeEvent struct {
	SentAt *time.Time      `json:"sent_at,omitempty"`
	World  string          `json:"world"`
	Metric string          `json:"metric"`
	Rule   json.RawMessage `json:"rule"`
}

// MetricObservationEvent is the payload shape for metric_observation events.
// Sent by Holodeck when a metric value is observed.
type MetricObservationEvent struct {
	SentAt *time.Time `json:"sent_at,omitempty"`
	Metric string     `json:"metric"`
	Value  float64    `json:"value"`
}

// BuildSummary parses payload and returns a populated summary struct for
// the given event kind, serialised as json.RawMessage. Returns nil if the kind
// is unrecognised or the payload cannot be parsed.
//
// Nomad event stream payloads wrap the object under a topic key
// (e.g. {"Node": {...}}, {"Job": {...}}, {"Allocation": {...}}), so BuildSummary
// unwraps that inner object before populating the summary struct.
func BuildSummary(kind string, sentAt *time.Time, payload json.RawMessage) json.RawMessage {
switch {
case strings.HasPrefix(kind, "nomad_node_"):
var outer struct {
Node NomadNodeEvent `json:"Node"`
}
if err := json.Unmarshal(payload, &outer); err != nil {
return nil
}
outer.Node.SentAt = sentAt
out, _ := json.Marshal(outer.Node)
return out

case strings.HasPrefix(kind, "nomad_job_"):
var outer struct {
Job NomadJobEvent `json:"Job"`
}
if err := json.Unmarshal(payload, &outer); err != nil {
return nil
}
outer.Job.SentAt = sentAt
out, _ := json.Marshal(outer.Job)
return out

case strings.HasPrefix(kind, "nomad_allocation_"):
var outer struct {
Allocation NomadAllocationEvent `json:"Allocation"`
}
if err := json.Unmarshal(payload, &outer); err != nil {
return nil
}
outer.Allocation.SentAt = sentAt
out, _ := json.Marshal(outer.Allocation)
return out

case kind == "scale_intent":
var s ScaleIntentEvent
if err := json.Unmarshal(payload, &s); err != nil {
return nil
}
s.SentAt = sentAt
out, _ := json.Marshal(s)
return out

case kind == "metric_rule_change":
var s RuleChangeEvent
if err := json.Unmarshal(payload, &s); err != nil {
return nil
}
s.SentAt = sentAt
out, _ := json.Marshal(s)
return out

case kind == "metric_observation":
var s MetricObservationEvent
if err := json.Unmarshal(payload, &s); err != nil {
return nil
}
s.SentAt = sentAt
out, _ := json.Marshal(s)
return out
}
return nil
}
