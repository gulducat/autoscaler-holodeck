package observer

import (
"encoding/json"
"net/http"
"strconv"
"time"
)

// Server holds the HTTP mux and event store.
type Server struct {
store EventStore
mux   *http.ServeMux
}

func NewServer(store EventStore) *Server {
s := &Server{store: store, mux: http.NewServeMux()}
s.mux.HandleFunc("GET /", s.handleUI)
s.mux.HandleFunc("POST /v1/events", s.handleIngest)
s.mux.HandleFunc("GET /v1/events", s.handleQuery)
s.mux.HandleFunc("GET /v1/health", s.handleHealth)
return s
}

func (s *Server) Handler() http.Handler {
return s.mux
}

type ingestRequest struct {
Source  string          `json:"source"`
Kind    string          `json:"kind"`
SentAt  *time.Time      `json:"sent_at"`
Payload json.RawMessage `json:"payload"`
}

type ingestResponse struct {
Seq        int64     `json:"seq"`
IngestedAt time.Time `json:"ingested_at"`
Run        int64     `json:"run"`
}

type queryResponse struct {
Run    int64   `json:"run"`
Events []Event `json:"events"`
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
var req ingestRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "invalid request body", http.StatusBadRequest)
return
}

e := s.store.Add(EventInput{
Source:  req.Source,
Kind:    req.Kind,
SentAt:  req.SentAt,
Payload: req.Payload,
})

w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusAccepted)
json.NewEncoder(w).Encode(ingestResponse{
Seq:        e.Seq,
IngestedAt: e.IngestedAt,
Run:        e.Run,
})
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
q := r.URL.Query()

// run=0 is a valid run ID (pre-reset events). Omitting ?run= means "current
// run". We resolve the two cases here so Query always receives an explicit
// run ID and never has to guess.
var run int64
if rawRun := q.Get("run"); rawRun == "" {
run = s.store.CurrentRun()
} else {
var err error
run, err = strconv.ParseInt(rawRun, 10, 64)
if err != nil {
http.Error(w, "invalid run", http.StatusBadRequest)
return
}
}

var since int64
if rawSince := q.Get("since"); rawSince != "" {
var err error
since, err = strconv.ParseInt(rawSince, 10, 64)
if err != nil {
http.Error(w, "invalid since", http.StatusBadRequest)
return
}
}

_, events := s.store.Query(run, since, q.Get("kind"))
if events == nil {
events = []Event{}
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(queryResponse{Run: run, Events: events})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
