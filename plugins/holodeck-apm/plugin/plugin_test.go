package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

func newTestPlugin(t *testing.T) *Plugin {
	t.Helper()
	return New(hclog.NewNullLogger())
}

// TestSetConfig covers config validation paths.
func TestSetConfig(t *testing.T) {
	cases := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name:    "valid holodeck only",
			config:  map[string]string{configKeyHolodeck: "http://localhost:9091"},
			wantErr: false,
		},
		{
			name:    "valid holodeck with trailing slash",
			config:  map[string]string{configKeyHolodeck: "http://localhost:9091/"},
			wantErr: false,
		},
		{
			name:    "valid holodeck and observer",
			config:  map[string]string{configKeyHolodeck: "http://localhost:9091", configKeyObserver: "http://localhost:9090"},
			wantErr: false,
		},
		{
			name:    "missing holodeck_address",
			config:  map[string]string{},
			wantErr: true,
		},
		{
			name:    "invalid holodeck URL",
			config:  map[string]string{configKeyHolodeck: "not-a-url"},
			wantErr: true,
		},
		{
			name:    "invalid observer URL",
			config:  map[string]string{configKeyHolodeck: "http://localhost:9091", configKeyObserver: "not-a-url"},
			wantErr: true,
		},
		{
			name:    "empty observer is allowed",
			config:  map[string]string{configKeyHolodeck: "http://localhost:9091", configKeyObserver: ""},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestPlugin(t)
			err := p.SetConfig(tc.config)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestQuery covers the Query method.
func TestQuery(t *testing.T) {
	fixedTime := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)

	cases := []struct {
		name           string
		holodeckStatus int
		holodeckBody   string
		wantValue      float64
		wantTimestamp  time.Time
		wantErr        bool
		errContains    string
	}{
		{
			name:           "success",
			holodeckStatus: http.StatusOK,
			holodeckBody:   fmt.Sprintf(`{"metric":"cpu","value":0.75,"queried_at":%q}`, fixedTime.Format(time.RFC3339Nano)),
			wantValue:      0.75,
			wantTimestamp:  fixedTime,
		},
		{
			name:           "holodeck 404 with error body",
			holodeckStatus: http.StatusNotFound,
			holodeckBody:   `{"error":"metric not found: cpu"}`,
			wantErr:        true,
			errContains:    "metric not found: cpu",
		},
		{
			name:           "holodeck 500 with plain body",
			holodeckStatus: http.StatusInternalServerError,
			holodeckBody:   `internal server error`,
			wantErr:        true,
			errContains:    "status 500",
		},
		{
			name:           "holodeck invalid JSON",
			holodeckStatus: http.StatusOK,
			holodeckBody:   `not json`,
			wantErr:        true,
			errContains:    "parsing holodeck response",
		},
		{
			name:           "holodeck missing queried_at",
			holodeckStatus: http.StatusOK,
			holodeckBody:   `{"metric":"cpu","value":0.5}`,
			wantErr:        true,
			errContains:    "missing queried_at",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/metrics" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tc.holodeckStatus)
				fmt.Fprint(w, tc.holodeckBody)
			}))
			defer hs.Close()

			p := newTestPlugin(t)
			if err := p.SetConfig(map[string]string{configKeyHolodeck: hs.URL}); err != nil {
				t.Fatalf("SetConfig: %v", err)
			}

			metrics, err := p.Query("cpu", sdk.TimeRange{})
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(metrics) != 1 {
				t.Fatalf("expected 1 metric, got %d", len(metrics))
			}
			if metrics[0].Value != tc.wantValue {
				t.Errorf("value: got %v, want %v", metrics[0].Value, tc.wantValue)
			}
			if !metrics[0].Timestamp.Equal(tc.wantTimestamp) {
				t.Errorf("timestamp: got %v, want %v", metrics[0].Timestamp, tc.wantTimestamp)
			}
		})
	}
}

// TestQuery_TimeRangeIgnored confirms that timeRange has no effect on the result.
func TestQuery_TimeRangeIgnored(t *testing.T) {
	fixedTime := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	body := fmt.Sprintf(`{"metric":"cpu","value":0.5,"queried_at":%q}`, fixedTime.Format(time.RFC3339Nano))

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer hs.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{configKeyHolodeck: hs.URL}) //nolint:errcheck

	r1, err := p.Query("cpu", sdk.TimeRange{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := p.Query("cpu", sdk.TimeRange{From: time.Now().Add(-1 * time.Hour), To: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if r1[0].Value != r2[0].Value {
		t.Error("timeRange should not affect the result")
	}
}

// TestQuery_ObserverFailureDoesNotBlockQuery verifies that an unreachable
// Observer does not cause Query to fail or block.
func TestQuery_ObserverFailureDoesNotBlockQuery(t *testing.T) {
	fixedTime := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	hBody := fmt.Sprintf(`{"metric":"cpu","value":0.75,"queried_at":%q}`, fixedTime.Format(time.RFC3339Nano))

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, hBody)
	}))
	defer hs.Close()

	p := newTestPlugin(t)
	// Point observer at a port with nothing listening.
	p.SetConfig(map[string]string{ //nolint:errcheck
		configKeyHolodeck: hs.URL,
		configKeyObserver: "http://127.0.0.1:19999",
	})

	done := make(chan error, 1)
	go func() {
		_, err := p.Query("cpu", sdk.TimeRange{})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Query should succeed despite observer failure: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Query blocked too long with unreachable observer")
	}
}

// TestQueryMultiple verifies it wraps Query correctly.
func TestQueryMultiple(t *testing.T) {
	fixedTime := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	hBody := fmt.Sprintf(`{"metric":"cpu","value":0.9,"queried_at":%q}`, fixedTime.Format(time.RFC3339Nano))

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, hBody)
	}))
	defer hs.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{configKeyHolodeck: hs.URL}) //nolint:errcheck

	results, err := p.QueryMultiple("cpu", sdk.TimeRange{})
	if err != nil {
		t.Fatalf("QueryMultiple: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 series, got %d", len(results))
	}
	if len(results[0]) != 1 {
		t.Fatalf("expected 1 metric in series, got %d", len(results[0]))
	}
	if results[0][0].Value != 0.9 {
		t.Errorf("value: got %v, want 0.9", results[0][0].Value)
	}
}

// TestQuery_ObserverPayload verifies the observer receives the correct event shape.
func TestQuery_ObserverPayload(t *testing.T) {
	fixedTime := time.Date(2026, 4, 29, 14, 0, 0, 0, time.UTC)
	hBody := fmt.Sprintf(`{"metric":"cpu","value":0.42,"queried_at":%q}`, fixedTime.Format(time.RFC3339Nano))

	received := make(chan []byte, 1)
	observerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer observerServer.Close()

	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, hBody)
	}))
	defer hs.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{ //nolint:errcheck
		configKeyHolodeck: hs.URL,
		configKeyObserver: observerServer.URL,
	})
	p.Query("cpu", sdk.TimeRange{}) //nolint:errcheck

	select {
	case body := <-received:
		var event observerEvent
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatalf("invalid observer payload: %v", err)
		}
		if event.Source != pluginName {
			t.Errorf("source: got %q, want %q", event.Source, pluginName)
		}
		if event.Kind != "metric_observation" {
			t.Errorf("kind: got %q, want %q", event.Kind, "metric_observation")
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("invalid observer payload.payload: %v", err)
		}
		if payload["query"] != "cpu" {
			t.Errorf("payload.query: got %v, want %q", payload["query"], "cpu")
		}
		if payload["value"] != 0.42 {
			t.Errorf("payload.value: got %v, want 0.42", payload["value"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("observer not called within timeout")
	}
}
