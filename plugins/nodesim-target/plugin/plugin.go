package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName        = "nodesim-target"
	configKeyNodesim  = "nodesim_address"
	configKeyObserver = "observer_address"
	configKeyGroup    = "node_group"
	observerTimeout   = 3 * time.Second
	scaleTimeout      = 15 * time.Second
)

// Ensure Plugin implements target.Target at compile time.
var _ target.Target = (*Plugin)(nil)

type Plugin struct {
	logger     hclog.Logger
	nodesim    *url.URL
	observer   *url.URL
	httpClient *http.Client
}

func New(log hclog.Logger) *Plugin {
	return &Plugin{
		logger:     log,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *Plugin) PluginInfo() (*base.PluginInfo, error) {
	return &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}, nil
}

func (p *Plugin) SetConfig(config map[string]string) error {
	addr := strings.TrimRight(config[configKeyNodesim], "/")
	if addr == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyNodesim)
	}
	nURL, err := url.ParseRequestURI(addr)
	if err != nil || nURL.Scheme == "" || nURL.Host == "" {
		return fmt.Errorf("%q is not a valid URL: %s", configKeyNodesim, addr)
	}
	p.nodesim = nURL

	if obs := strings.TrimRight(config[configKeyObserver], "/"); obs != "" {
		oURL, err := url.ParseRequestURI(obs)
		if err != nil || oURL.Scheme == "" || oURL.Host == "" {
			return fmt.Errorf("%q is not a valid URL: %s", configKeyObserver, obs)
		}
		p.observer = oURL
	}

	return nil
}

// groupResponse is the shape of GET /v1/groups/{name} responses.
type groupResponse struct {
	Name     string `json:"name"`
	NodePool string `json:"node_pool"`
	Count    int64  `json:"count"`
	Nodes    int64  `json:"nodes"`
	Ready    bool   `json:"ready"`
}

// Status implements target.Target. It calls GET /v1/groups/{name} on nodesim
// and maps the response onto a TargetStatus.
func (p *Plugin) Status(config map[string]string) (*sdk.TargetStatus, error) {
	group := config[configKeyGroup]
	if group == "" {
		return nil, fmt.Errorf("%q config value cannot be empty", configKeyGroup)
	}

	u := *p.nodesim
	u.Path = "/v1/groups/" + group

	resp, err := p.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("nodesim get group: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading nodesim response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nodesim returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var g groupResponse
	if err := json.Unmarshal(body, &g); err != nil {
		return nil, fmt.Errorf("parsing nodesim response: %w", err)
	}

	return &sdk.TargetStatus{
		Ready: g.Ready,
		Count: g.Nodes,
	}, nil
}

// scaleRequest is the POST body for POST /v1/groups/{name}/scale.
type scaleRequest struct {
	Count int64 `json:"count"`
}

// Scale implements target.Target. It emits a scale_intent event to the
// Observer (synchronously, best-effort) before calling nodesim.
func (p *Plugin) Scale(action sdk.ScalingAction, config map[string]string) error {
	if action.Count == sdk.StrategyActionMetaValueDryRunCount {
		return nil
	}

	group := config[configKeyGroup]
	if group == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyGroup)
	}

	status, err := p.Status(config)
	if err != nil {
		return fmt.Errorf("getting current status: %w", err)
	}

	if action.Count == status.Count {
		return sdk.NewTargetScalingNoOpError("already at desired count %d", action.Count)
	}

	// Emit intent to the Observer before calling nodesim. Synchronous so that
	// the event is recorded before the scale call completes; errors are logged
	// and never propagated.
	p.emitScaleIntent(group, action.Count, status.Count)

	u := *p.nodesim
	u.Path = "/v1/groups/" + group + "/scale"

	body, err := json.Marshal(scaleRequest{Count: action.Count})
	if err != nil {
		return fmt.Errorf("marshaling scale request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), scaleTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating scale request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nodesim scale: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nodesim scale returned status %d", resp.StatusCode)
	}

	return nil
}

// observerEvent is the POST body shape for POST /v1/events.
type observerEvent struct {
	Source  string          `json:"source"`
	Kind    string          `json:"kind"`
	SentAt  time.Time       `json:"sent_at"`
	Payload json.RawMessage `json:"payload"`
}

// emitScaleIntent posts a scale_intent event to the Observer. It is
// synchronous so the event is recorded before the nodesim call; any failure
// is logged and discarded.
func (p *Plugin) emitScaleIntent(group string, desired, current int64) {
	if p.observer == nil {
		return
	}

	inner, err := json.Marshal(map[string]any{
		"group":         group,
		"desired_count": desired,
		"current_count": current,
	})
	if err != nil {
		p.logger.Warn("failed to marshal scale_intent payload", "error", err)
		return
	}

	event := observerEvent{
		Source:  pluginName,
		Kind:    "scale_intent",
		SentAt:  time.Now().UTC(),
		Payload: inner,
	}
	eventBody, err := json.Marshal(event)
	if err != nil {
		p.logger.Warn("failed to marshal observer event", "error", err)
		return
	}

	u := *p.observer
	u.Path = "/v1/events"

	ctx, cancel := context.WithTimeout(context.Background(), observerTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(eventBody))
	if err != nil {
		p.logger.Warn("failed to create observer request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.logger.Warn("observer emit failed", "error", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode != http.StatusAccepted {
		p.logger.Warn("observer returned unexpected status", "status", resp.StatusCode)
	}
}
