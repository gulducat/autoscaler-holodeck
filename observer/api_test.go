package observer_test

import (
"encoding/json"
"net/http"
"net/http/httptest"
"strings"
"testing"
"time"

"github.com/gulducat/autoscaler-holodeck/observer"
"github.com/gulducat/autoscaler-holodeck/observer/mocks"
)

func TestHandleHealth(t *testing.T) {
srv := observer.NewServer(&mocks.MockStore{
CurrentRunFunc: func() int64 { return 0 },
})
req := httptest.NewRequest("GET", "/v1/health", nil)
w := httptest.NewRecorder()
srv.Handler().ServeHTTP(w, req)

if w.Code != http.StatusOK {
t.Errorf("expected 200, got %d", w.Code)
}
var body map[string]string
json.NewDecoder(w.Body).Decode(&body)
if body["status"] != "ok" {
t.Errorf("expected status=ok, got %s", body["status"])
}
}

func TestHandleIngest(t *testing.T) {
now := time.Now()
fixedEvent := observer.Event{Seq: 7, Run: 2, IngestedAt: now}

tests := []struct {
name       string
body       string
store      *mocks.MockStore
wantStatus int
wantSeq    int64
wantRun    int64
}{
{
name: "valid event returns 202 with seq and run",
body: `{"source":"holodeck","kind":"world_reset","sent_at":"2026-04-28T21:00:00Z","payload":{}}`,
store: &mocks.MockStore{
AddFunc: func(input observer.EventInput) observer.Event { return fixedEvent },
},
wantStatus: http.StatusAccepted,
wantSeq:    7,
wantRun:    2,
},
{
name:       "invalid body returns 400",
body:       "not json",
store:      &mocks.MockStore{},
wantStatus: http.StatusBadRequest,
},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
srv := observer.NewServer(tc.store)
req := httptest.NewRequest("POST", "/v1/events", strings.NewReader(tc.body))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
srv.Handler().ServeHTTP(w, req)

if w.Code != tc.wantStatus {
t.Errorf("expected status %d, got %d", tc.wantStatus, w.Code)
}
if tc.wantStatus == http.StatusAccepted {
var resp struct {
Seq int64 `json:"seq"`
Run int64 `json:"run"`
}
json.NewDecoder(w.Body).Decode(&resp)
if resp.Seq != tc.wantSeq || resp.Run != tc.wantRun {
t.Errorf("expected seq=%d run=%d, got seq=%d run=%d", tc.wantSeq, tc.wantRun, resp.Seq, resp.Run)
}
}
})
}
}

func TestHandleQuery(t *testing.T) {
tests := []struct {
name                   string
url                    string
store                  *mocks.MockStore
wantStatus             int
wantCurrentRunCalled   bool
wantCurrentRunNotCalled bool
wantQueriedRun         *int64
checkNonNullEvents     bool
}{
{
name: "omitted run calls CurrentRun and passes its value to Query",
url:  "/v1/events",
store: &mocks.MockStore{
CurrentRunFunc: func() int64 { return 5 },
QueryFunc: func(run, since int64, kind string) (int64, []observer.Event) {
return 5, nil
},
},
wantStatus:           http.StatusOK,
wantCurrentRunCalled: true,
wantQueriedRun:       int64ptr(5),
},
{
name: "explicit run=0 does not call CurrentRun",
url:  "/v1/events?run=0",
store: &mocks.MockStore{
CurrentRunFunc: func() int64 { return 5 },
QueryFunc: func(run, since int64, kind string) (int64, []observer.Event) {
return 0, nil
},
},
wantStatus:              http.StatusOK,
wantCurrentRunNotCalled: true,
wantQueriedRun:          int64ptr(0),
},
{
name:       "invalid run returns 400",
url:        "/v1/events?run=abc",
store:      &mocks.MockStore{},
wantStatus: http.StatusBadRequest,
},
{
name: "invalid since returns 400",
url:  "/v1/events?since=abc",
store: &mocks.MockStore{
CurrentRunFunc: func() int64 { return 0 },
},
wantStatus: http.StatusBadRequest,
},
{
name: "nil events from store returns empty JSON array not null",
url:  "/v1/events",
store: &mocks.MockStore{
CurrentRunFunc: func() int64 { return 0 },
QueryFunc: func(run, since int64, kind string) (int64, []observer.Event) {
return 0, nil
},
},
wantStatus:         http.StatusOK,
checkNonNullEvents: true,
},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
currentRunCalled := false
queriedRun := int64(-999)

if tc.store.CurrentRunFunc != nil {
orig := tc.store.CurrentRunFunc
tc.store.CurrentRunFunc = func() int64 {
currentRunCalled = true
return orig()
}
}
if tc.store.QueryFunc != nil {
orig := tc.store.QueryFunc
tc.store.QueryFunc = func(run, since int64, kind string) (int64, []observer.Event) {
queriedRun = run
return orig(run, since, kind)
}
}

srv := observer.NewServer(tc.store)
req := httptest.NewRequest("GET", tc.url, nil)
w := httptest.NewRecorder()
srv.Handler().ServeHTTP(w, req)

if w.Code != tc.wantStatus {
t.Errorf("expected status %d, got %d", tc.wantStatus, w.Code)
}
if tc.wantCurrentRunCalled && !currentRunCalled {
t.Error("expected CurrentRun() to be called")
}
if tc.wantCurrentRunNotCalled && currentRunCalled {
t.Error("CurrentRun() must not be called")
}
if tc.wantQueriedRun != nil && queriedRun != *tc.wantQueriedRun {
t.Errorf("expected queried run=%d, got %d", *tc.wantQueriedRun, queriedRun)
}
if tc.checkNonNullEvents {
var resp struct {
Events []observer.Event `json:"events"`
}
json.NewDecoder(w.Body).Decode(&resp)
if resp.Events == nil {
t.Error("expected non-nil events array, got JSON null")
}
}
})
}
}

func int64ptr(v int64) *int64 { return &v }
