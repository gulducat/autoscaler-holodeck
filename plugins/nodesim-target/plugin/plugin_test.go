package plugin

import (
	"encoding/json"
	"errors"
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
			name:    "valid nodesim only",
			config:  map[string]string{configKeyNodesim: "http://localhost:8082"},
			wantErr: false,
		},
		{
			name:    "valid nodesim with trailing slash",
			config:  map[string]string{configKeyNodesim: "http://localhost:8082/"},
			wantErr: false,
		},
		{
			name:    "valid nodesim and observer",
			config:  map[string]string{configKeyNodesim: "http://localhost:8082", configKeyObserver: "http://localhost:8081"},
			wantErr: false,
		},
		{
			name:    "missing nodesim_address",
			config:  map[string]string{},
			wantErr: true,
		},
		{
			name:    "invalid nodesim URL",
			config:  map[string]string{configKeyNodesim: "not-a-url"},
			wantErr: true,
		},
		{
			name:    "invalid observer URL",
			config:  map[string]string{configKeyNodesim: "http://localhost:8082", configKeyObserver: "not-a-url"},
			wantErr: true,
		},
		{
			name:    "empty observer is allowed",
			config:  map[string]string{configKeyNodesim: "http://localhost:8082", configKeyObserver: ""},
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

// TestPluginInfo verifies the plugin reports the correct name and type.
func TestPluginInfo(t *testing.T) {
	p := newTestPlugin(t)
	info, err := p.PluginInfo()
	if err != nil {
		t.Fatalf("PluginInfo: %v", err)
	}
	if info.Name != pluginName {
		t.Errorf("name: got %q, want %q", info.Name, pluginName)
	}
	if info.PluginType != sdk.PluginTypeTarget {
		t.Errorf("type: got %q, want %q", info.PluginType, sdk.PluginTypeTarget)
	}
}

func groupJSON(nodes int64, ready bool) string {
	return fmt.Sprintf(`{"name":"test-group","node_pool":"default","count":%d,"nodes":%d,"ready":%v}`, nodes, nodes, ready)
}

// TestStatus covers the Status method.
func TestStatus(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantCount   int64
		wantReady   bool
		wantErr     bool
		errContains string
	}{
		{
			name:      "ready group",
			status:    http.StatusOK,
			body:      groupJSON(3, true),
			wantCount: 3,
			wantReady: true,
		},
		{
			name:      "not-yet-ready group",
			status:    http.StatusOK,
			body:      groupJSON(2, false),
			wantCount: 2,
			wantReady: false,
		},
		{
			name:        "group not found",
			status:      http.StatusNotFound,
			body:        `{"error":"group not found"}`,
			wantErr:     true,
			errContains: "404",
		},
		{
			name:        "invalid JSON",
			status:      http.StatusOK,
			body:        `not json`,
			wantErr:     true,
			errContains: "parsing nodesim response",
		},
		{
			name:        "missing node_group config",
			status:      http.StatusOK,
			body:        groupJSON(1, true),
			wantErr:     true,
			errContains: configKeyGroup,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer ns.Close()

			p := newTestPlugin(t)
			if err := p.SetConfig(map[string]string{configKeyNodesim: ns.URL}); err != nil {
				t.Fatalf("SetConfig: %v", err)
			}

			config := map[string]string{configKeyGroup: "test-group"}
			if tc.name == "missing node_group config" {
				config = map[string]string{}
			}

			status, err := p.Status(config)
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
			if status.Count != tc.wantCount {
				t.Errorf("count: got %d, want %d", status.Count, tc.wantCount)
			}
			if status.Ready != tc.wantReady {
				t.Errorf("ready: got %v, want %v", status.Ready, tc.wantReady)
			}
		})
	}
}

// TestScale_DryRun verifies that a dry-run action makes no calls.
func TestScale_DryRun(t *testing.T) {
	called := false
	ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ns.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{configKeyNodesim: ns.URL}) //nolint:errcheck

	err := p.Scale(sdk.ScalingAction{Count: sdk.StrategyActionMetaValueDryRunCount}, map[string]string{configKeyGroup: "g"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("nodesim should not be called during dry-run")
	}
}

// TestScale_NoOp verifies that scaling to the current count returns a NoOpError.
func TestScale_NoOp(t *testing.T) {
	ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only Status calls (GET) should arrive; Scale calls (POST to /scale) must not.
		if r.Method == http.MethodPost {
			t.Error("nodesim scale should not be called for no-op")
		}
		fmt.Fprint(w, groupJSON(3, true))
	}))
	defer ns.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{configKeyNodesim: ns.URL}) //nolint:errcheck

	err := p.Scale(sdk.ScalingAction{Count: 3}, map[string]string{configKeyGroup: "g"})
	if err == nil {
		t.Fatal("expected NoOpError, got nil")
	}
	var noOp *sdk.TargetScalingNoOpError
	if !errors.As(err, &noOp) {
		t.Errorf("expected TargetScalingNoOpError, got: %v", err)
	}
}

// TestScale_Success verifies a normal scale-up calls nodesim and returns nil.
func TestScale_Success(t *testing.T) {
	var scaleCalled bool
	ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			fmt.Fprint(w, groupJSON(3, true))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/scale"):
			scaleCalled = true
			var req scaleRequest
			json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
			if req.Count != 5 {
				t.Errorf("scale count: got %d, want 5", req.Count)
			}
			fmt.Fprint(w, `{"name":"g","count":5,"nodes":5}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ns.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{configKeyNodesim: ns.URL}) //nolint:errcheck

	err := p.Scale(sdk.ScalingAction{Count: 5}, map[string]string{configKeyGroup: "g"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !scaleCalled {
		t.Error("nodesim scale endpoint was not called")
	}
}

// TestScale_ObserverPayload verifies the observer receives the correct
// scale_intent event shape, and that it is sent before nodesim is called.
func TestScale_ObserverPayload(t *testing.T) {
	observerReceived := make(chan []byte, 1)
	obs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		observerReceived <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer obs.Close()

	observerCalledBeforeNodesim := false
	ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			fmt.Fprint(w, groupJSON(2, true))
		case r.Method == http.MethodPost:
			// At this point the observer should have already been called.
			observerCalledBeforeNodesim = len(observerReceived) > 0
			fmt.Fprint(w, `{"name":"g","count":4,"nodes":4}`)
		}
	}))
	defer ns.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{ //nolint:errcheck
		configKeyNodesim:  ns.URL,
		configKeyObserver: obs.URL,
	})

	if err := p.Scale(sdk.ScalingAction{Count: 4}, map[string]string{configKeyGroup: "my-group"}); err != nil {
		t.Fatalf("Scale: %v", err)
	}

	if !observerCalledBeforeNodesim {
		t.Error("observer was not called before nodesim scale")
	}

	select {
	case body := <-observerReceived:
		var event observerEvent
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatalf("invalid observer payload: %v", err)
		}
		if event.Source != pluginName {
			t.Errorf("source: got %q, want %q", event.Source, pluginName)
		}
		if event.Kind != "scale_intent" {
			t.Errorf("kind: got %q, want %q", event.Kind, "scale_intent")
		}
		var payload map[string]any
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("invalid observer payload.payload: %v", err)
		}
		if payload["group"] != "my-group" {
			t.Errorf("payload.group: got %v, want %q", payload["group"], "my-group")
		}
		if payload["desired_count"] != float64(4) {
			t.Errorf("payload.desired_count: got %v, want 4", payload["desired_count"])
		}
		if payload["current_count"] != float64(2) {
			t.Errorf("payload.current_count: got %v, want 2", payload["current_count"])
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("observer not called within timeout")
	}
}

// TestScale_ObserverFailureDoesNotBlockScale verifies that an unreachable
// Observer does not cause Scale to fail.
func TestScale_ObserverFailureDoesNotBlockScale(t *testing.T) {
	ns := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, groupJSON(1, true))
		} else {
			fmt.Fprint(w, `{"name":"g","count":2,"nodes":2}`)
		}
	}))
	defer ns.Close()

	p := newTestPlugin(t)
	p.SetConfig(map[string]string{ //nolint:errcheck
		configKeyNodesim:  ns.URL,
		configKeyObserver: "http://127.0.0.1:19999",
	})

	done := make(chan error, 1)
	go func() {
		done <- p.Scale(sdk.ScalingAction{Count: 2}, map[string]string{configKeyGroup: "g"})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Scale should succeed despite observer failure: %v", err)
		}
	case <-time.After(observerTimeout + time.Second):
		t.Fatal("Scale blocked too long with unreachable observer")
	}
}
