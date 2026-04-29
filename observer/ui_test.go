package observer_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/gulducat/autoscaler-holodeck/observer"
	"github.com/gulducat/autoscaler-holodeck/observer/mocks"
)

var uiFixtureEvents = []observer.Event{
	{
		Seq:        1,
		Run:        1,
		IngestedAt: time.Now(),
		Source:     "holodeck",
		Kind:       "world_reset",
		Payload:    json.RawMessage(`{}`),
	},
	{
		Seq:        2,
		Run:        1,
		IngestedAt: time.Now(),
		Source:     "holodeck",
		Kind:       "metric_rule_change",
		Payload:    json.RawMessage(`{"metric":"cpu_utilization","value":0.75}`),
	},
	{
		Seq:        3,
		Run:        1,
		IngestedAt: time.Now(),
		Source:     "holodeck-apm",
		Kind:       "metric_observation",
		Payload:    json.RawMessage(`{"query":"cpu_utilization","value":0.75}`),
	},
	{
		Seq:        4,
		Run:        1,
		IngestedAt: time.Now(),
		Source:     "nodesim-target",
		Kind:       "scale_intent",
		Payload:    json.RawMessage(`{"group":"my-group","desired_count":5,"current_count":3}`),
	},
	{
		Seq:        5,
		Run:        1,
		IngestedAt: time.Now(),
		Source:     "nomad",
		Kind:       "nomad_job_scalingevent",
		Payload:    json.RawMessage(`{"job":"my-job","task_group":"web"}`),
	},
}

// TestUI_Visual serves the Observer UI with fixture events so it can be
// visually inspected in a browser. Skipped unless VISUAL=1 is set.
//
// Usage:
//
// VISUAL=1 go test ./... -run TestUI_Visual -v
func TestUI_Visual(t *testing.T) {
	if os.Getenv("VISUAL") == "" {
		t.Skip("set VISUAL=1 to run visual UI test")
	}

	store := &mocks.MockStore{
		CurrentRunFunc: func() int64 { return 1 },
		QueryFunc: func(run, since int64, kind string) (int64, []observer.Event) {
			if since > 0 {
				return 1, []observer.Event{}
			}
			return 1, uiFixtureEvents
		},
	}

	ts := httptest.NewServer(observer.NewServer(store).Handler())
	time.Sleep(1 * time.Second)
	defer ts.Close()
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	openBrowser(ts.URL)
	fmt.Printf("\nObserver UI: %s", ts.URL)
	time.Sleep(3 * time.Second)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Start() //nolint:errcheck
}
