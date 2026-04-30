package holodeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	m := NewWorldManager(NewNomadTracker(), NewObserverClient("", noopLogger()))
	return NewServer(context.Background(), m)
}

func doRequest(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "GET", "/v1/health", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("health: got %d", rr.Code)
	}
}

func TestHandleListWorlds(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "GET", "/v1/worlds", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list worlds: got %d", rr.Code)
	}
	var resp map[string][]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp["worlds"]) == 0 {
		t.Error("expected at least the default world")
	}
}

func TestHandleGetWorld_NotFound(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "GET", "/v1/worlds/nonexistent", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleSetAndGetWorld(t *testing.T) {
	srv := newTestServer(t)
	body := `{"metrics":{"cpu":{"type":"authored","value":0.42}}}`

	rr := doRequest(t, srv, "PUT", "/v1/worlds/default", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put world: got %d: %s", rr.Code, rr.Body.String())
	}

	rr = doRequest(t, srv, "GET", "/v1/worlds/default", "")
	var resp worldStateResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Metrics["cpu"].Rule.Value != 0.42 {
		t.Errorf("expected 0.42, got %v", resp.Metrics["cpu"].Rule.Value)
	}
}

func TestHandleResetWorld(t *testing.T) {
	srv := newTestServer(t)
	doRequest(t, srv, "PUT", "/v1/worlds/default", `{"metrics":{"cpu":{"type":"authored","value":1}}}`)
	doRequest(t, srv, "POST", "/v1/worlds/default/reset", "")

	rr := doRequest(t, srv, "GET", "/v1/worlds/default", "")
	var resp worldStateResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Metrics) != 0 {
		t.Errorf("expected empty metrics after reset, got %v", resp.Metrics)
	}
}

func TestHandleQueryMetric(t *testing.T) {
	srv := newTestServer(t)
	doRequest(t, srv, "PUT", "/v1/worlds/default", `{"metrics":{"cpu":{"type":"authored","value":0.5}}}`)

	rr := doRequest(t, srv, "GET", "/v1/worlds/default/metrics?metric=cpu", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("query metric: got %d", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["value"].(float64) != 0.5 {
		t.Errorf("expected 0.5, got %v", resp["value"])
	}
}

func TestHandleQueryMetric_NotFound(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "GET", "/v1/worlds/default/metrics?metric=missing", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleQueryMetric_MissingParam(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "GET", "/v1/worlds/default/metrics", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestShorthandRoutesDefaultWorld(t *testing.T) {
	srv := newTestServer(t)
	body := `{"metrics":{"load":{"type":"authored","value":0.7}}}`

	// PUT /v1/world → world=default
	rr := doRequest(t, srv, "PUT", "/v1/world", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /v1/world: got %d", rr.Code)
	}

	// GET /v1/world → world=default
	rr = doRequest(t, srv, "GET", "/v1/world", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/world: got %d", rr.Code)
	}

	// GET /v1/metrics?metric=load → world=default
	rr = doRequest(t, srv, "GET", "/v1/metrics?metric=load", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /v1/metrics: got %d: %s", rr.Code, rr.Body.String())
	}

	// POST /v1/world/reset → world=default
	rr = doRequest(t, srv, "POST", "/v1/world/reset", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /v1/world/reset: got %d", rr.Code)
	}
}

func TestHandleSetWorld_CreatesNewWorld(t *testing.T) {
	srv := newTestServer(t)
	rr := doRequest(t, srv, "PUT", "/v1/worlds/new-world",
		`{"metrics":{"x":{"type":"authored","value":99}}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
