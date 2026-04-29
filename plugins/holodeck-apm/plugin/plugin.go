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
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName        = "holodeck-apm"
	configKeyHolodeck = "holodeck_address"
	configKeyObserver = "observer_address"
	observerTimeout   = 3 * time.Second
)

// Ensure Plugin implements apm.APM at compile time.
var _ apm.APM = (*Plugin)(nil)

type Plugin struct {
	logger     hclog.Logger
	holodeck   *url.URL
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
		PluginType: sdk.PluginTypeAPM,
	}, nil
}

func (p *Plugin) SetConfig(config map[string]string) error {
	addr := strings.TrimRight(config[configKeyHolodeck], "/")
	if addr == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyHolodeck)
	}
	hURL, err := url.ParseRequestURI(addr)
	if err != nil || hURL.Scheme == "" || hURL.Host == "" {
		return fmt.Errorf("%q is not a valid URL: %s", configKeyHolodeck, addr)
	}
	p.holodeck = hURL

	if obs := strings.TrimRight(config[configKeyObserver], "/"); obs != "" {
		oURL, err := url.ParseRequestURI(obs)
		if err != nil || oURL.Scheme == "" || oURL.Host == "" {
			return fmt.Errorf("%q is not a valid URL: %s", configKeyObserver, obs)
		}
		p.observer = oURL
	}

	return nil
}

// holodeckMetricResponse is the expected shape of GET /v1/metrics responses.
type holodeckMetricResponse struct {
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	QueriedAt time.Time `json:"queried_at"`
}

// holodeckErrorResponse is the shape of Holodeck error responses.
type holodeckErrorResponse struct {
	Error string `json:"error"`
}

// Query implements apm.APM. The timeRange parameter is intentionally ignored
// because Holodeck only exposes the current metric value — it has no history.
// A single TimestampedMetric is returned, stamped with the queried_at time
// from the Holodeck response.
func (p *Plugin) Query(query string, _ sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	u := *p.holodeck
	u.Path = "/v1/metrics"
	q := url.Values{}
	q.Set("metric", query)
	u.RawQuery = q.Encode()

	resp, err := p.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("holodeck query failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading holodeck response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp holodeckErrorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return nil, fmt.Errorf("holodeck: %s (status %d)", errResp.Error, resp.StatusCode)
		}
		return nil, fmt.Errorf("holodeck returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result holodeckMetricResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing holodeck response: %w", err)
	}
	if result.QueriedAt.IsZero() {
		return nil, fmt.Errorf("holodeck response missing queried_at")
	}

	// Emit observation to the Observer in the background; best-effort only.
	go p.emitObservation(query, result.Value)

	return sdk.TimestampedMetrics{
		{Timestamp: result.QueriedAt, Value: result.Value},
	}, nil
}

// QueryMultiple implements apm.APM. It wraps Query since Holodeck exposes a
// single current value per metric, not multiple series.
func (p *Plugin) QueryMultiple(query string, timeRange sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	m, err := p.Query(query, timeRange)
	if err != nil {
		return nil, err
	}
	return []sdk.TimestampedMetrics{m}, nil
}

// observerEvent is the POST body shape for /v1/events.
type observerEvent struct {
	Source  string          `json:"source"`
	Kind    string          `json:"kind"`
	SentAt  time.Time       `json:"sent_at"`
	Payload json.RawMessage `json:"payload"`
}

// emitObservation posts a metric_observation event to the Observer.
// It is always called in a goroutine; any failure is logged and discarded.
func (p *Plugin) emitObservation(query string, value float64) {
	if p.observer == nil {
		return
	}

	inner, err := json.Marshal(map[string]any{"query": query, "value": value})
	if err != nil {
		p.logger.Warn("failed to marshal observation payload", "error", err)
		return
	}

	event := observerEvent{
		Source:  pluginName,
		Kind:    "metric_observation",
		SentAt:  time.Now().UTC(),
		Payload: inner,
	}
	body, err := json.Marshal(event)
	if err != nil {
		p.logger.Warn("failed to marshal observer event", "error", err)
		return
	}

	u := *p.observer
	u.Path = "/v1/events"

	ctx, cancel := context.WithTimeout(context.Background(), observerTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
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
