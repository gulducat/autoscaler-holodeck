package observer_test

import (
"encoding/json"
"testing"

"github.com/gulducat/autoscaler-holodeck/observer"
)

func newPayload() json.RawMessage { return json.RawMessage(`{}`) }

func TestStore_Add(t *testing.T) {
tests := []struct {
name          string
events        []observer.EventInput
wantSeqs      []int64
wantRuns      []int64
wantCurrentRun int64
}{
{
name: "pre-reset events belong to run 0 with ascending seqs",
events: []observer.EventInput{
{Kind: "metric_observation", Payload: newPayload()},
{Kind: "scale_intent", Payload: newPayload()},
},
wantSeqs:       []int64{1, 2},
wantRuns:       []int64{0, 0},
wantCurrentRun: 0,
},
{
name: "world_reset advances run to its own seq",
events: []observer.EventInput{
{Kind: "metric_observation", Payload: newPayload()},
{Kind: "world_reset", Payload: newPayload()},
{Kind: "metric_observation", Payload: newPayload()},
},
wantSeqs:       []int64{1, 2, 3},
wantRuns:       []int64{0, 2, 2},
wantCurrentRun: 2,
},
{
name: "multiple resets each open a new run",
events: []observer.EventInput{
{Kind: "world_reset", Payload: newPayload()},
{Kind: "metric_observation", Payload: newPayload()},
{Kind: "world_reset", Payload: newPayload()},
{Kind: "metric_observation", Payload: newPayload()},
},
wantSeqs:       []int64{1, 2, 3, 4},
wantRuns:       []int64{1, 1, 3, 3},
wantCurrentRun: 3,
},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
s := observer.NewStore()
var got []observer.Event
for _, input := range tc.events {
got = append(got, s.Add(input))
}
for i, e := range got {
if e.Seq != tc.wantSeqs[i] {
t.Errorf("event[%d]: expected seq=%d, got %d", i, tc.wantSeqs[i], e.Seq)
}
if e.Run != tc.wantRuns[i] {
t.Errorf("event[%d]: expected run=%d, got %d", i, tc.wantRuns[i], e.Run)
}
}
if s.CurrentRun() != tc.wantCurrentRun {
t.Errorf("expected currentRun=%d, got %d", tc.wantCurrentRun, s.CurrentRun())
}
})
}
}

func TestStore_Query(t *testing.T) {
tests := []struct {
name        string
setup       func(s *observer.Store)
run         int64
since       int64
kind        string
wantLen     int
wantKinds   []string
wantSeqs    []int64
wantCurRun  int64
}{
{
name: "since is exclusive",
setup: func(s *observer.Store) {
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()}) // seq=1
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()}) // seq=2
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()}) // seq=3
},
run: 0, since: 1, kind: "",
wantLen: 2, wantSeqs: []int64{2, 3},
},
{
name: "kind filter returns only matching events",
setup: func(s *observer.Store) {
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()})
s.Add(observer.EventInput{Kind: "scale_intent", Payload: newPayload()})
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()})
},
run: 0, since: 0, kind: "scale_intent",
wantLen: 1, wantKinds: []string{"scale_intent"},
},
{
name: "empty kind returns all events",
setup: func(s *observer.Store) {
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()})
s.Add(observer.EventInput{Kind: "scale_intent", Payload: newPayload()})
},
run: 0, since: 0, kind: "",
wantLen: 2,
},
{
name: "run=0 is explicitly queryable for pre-reset events",
setup: func(s *observer.Store) {
s.Add(observer.EventInput{Kind: "metric_observation", Payload: newPayload()}) // run=0
s.Add(observer.EventInput{Kind: "world_reset", Payload: newPayload()})        // opens run=2
},
run: 0, since: 0, kind: "",
wantLen: 1, wantKinds: []string{"metric_observation"}, wantCurRun: 2,
},
{
name: "returns consistent currentRun snapshot",
setup: func(s *observer.Store) {
s.Add(observer.EventInput{Kind: "world_reset", Payload: newPayload()}) // seq=1, run=1
},
run: 1, since: 0, kind: "",
wantLen: 1, wantCurRun: 1,
},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
s := observer.NewStore()
tc.setup(s)

curRun, events := s.Query(tc.run, tc.since, tc.kind)

if len(events) != tc.wantLen {
t.Fatalf("expected %d events, got %d", tc.wantLen, len(events))
}
if tc.wantCurRun != 0 && curRun != tc.wantCurRun {
t.Errorf("expected currentRun=%d, got %d", tc.wantCurRun, curRun)
}
for i, seq := range tc.wantSeqs {
if events[i].Seq != seq {
t.Errorf("event[%d]: expected seq=%d, got %d", i, seq, events[i].Seq)
}
}
for i, kind := range tc.wantKinds {
if events[i].Kind != kind {
t.Errorf("event[%d]: expected kind=%s, got %s", i, kind, events[i].Kind)
}
}
})
}
}
