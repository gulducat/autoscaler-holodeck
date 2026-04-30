package holodeck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	hclog "github.com/hashicorp/go-hclog"
)

// SampledMetricEntry holds a metric value and the source it was fetched from.
type SampledMetricEntry struct {
	Source string  `json:"source"`
	Value  float64 `json:"value"`
}

const (
	MetricSourcePrometheus   = "prometheus"
	MetricSourceNomadMetrics = "nomad_metrics"
	MetricSourceHolodeckAPM  = "holodeck_apm"
)

// SampleMetric identifies a metric endpoint and its format to sample at startup.
type SampleMetric struct {
	MetricSource string
	MetricURL    string
}

// ParseSampleMetrics parses a comma-separated list of "source:url" pairs.
// The split is on the first colon only, so URLs containing colons (e.g. http://)
// are handled correctly.
func ParseSampleMetrics(raw string) ([]SampleMetric, error) {
	var samples []SampleMetric
	for _, entry := range splitEntries(raw) {
		idx := indexOf(entry, ':')
		if idx < 0 {
			return nil, fmt.Errorf("invalid sample metric entry %q: expected \"source:url\"", entry)
		}
		samples = append(samples, SampleMetric{
			MetricSource: entry[:idx],
			MetricURL:    entry[idx+1:],
		})
	}
	return samples, nil
}

// splitEntries splits a comma-separated string, trimming whitespace from each part.
func splitEntries(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			entry := trim(s[start:i])
			if entry != "" {
				out = append(out, entry)
			}
			start = i + 1
		}
	}
	return out
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// IngestSampledMetrics fetches each SampleMetric once at startup and stores
// the results for display in the UI.
func IngestSampledMetrics(ctx context.Context, samples []SampleMetric, manager *WorldManager, logger hclog.Logger) {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, s := range samples {
		metrics, err := fetchMetrics(ctx, client, s)
		if err != nil {
			logger.Warn("failed to ingest sampled metrics", "source", s.MetricSource, "url", s.MetricURL, "error", err)
			continue
		}
		for name, value := range metrics {
			manager.StoreSampledMetric(name, SampledMetricEntry{Source: s.MetricSource, Value: value})
			logger.Info("stored sampled metric", "metric", name, "value", value, "source", s.MetricSource)
		}
	}
}

// fetchMetrics dispatches to the appropriate fetcher based on MetricSource.
func fetchMetrics(ctx context.Context, client *http.Client, s SampleMetric) (map[string]float64, error) {
	switch s.MetricSource {
	case MetricSourceHolodeckAPM:
		return fetchHolodeckAPM(ctx, client, s.MetricURL)
	case MetricSourceNomadMetrics:
		return fetchNomadMetrics(ctx, client, s.MetricURL)
	case MetricSourcePrometheus:
		return nil, fmt.Errorf("prometheus source not yet implemented")
	default:
		return nil, fmt.Errorf("unknown metric source: %q", s.MetricSource)
	}
}

// fetchHolodeckAPM fetches a single metric from a holodeck-apm-compatible
// endpoint, which returns {"metric": "name", "value": float64}.
func fetchHolodeckAPM(ctx context.Context, client *http.Client, url string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var m struct {
		Metric string  `json:"metric"`
		Value  float64 `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if m.Metric == "" {
		return nil, fmt.Errorf("response missing metric name")
	}
	return map[string]float64{m.Metric: m.Value}, nil
}

// nomadMetricsResponse is the JSON shape of Nomad's GET /v1/metrics endpoint.
type nomadMetricsResponse struct {
	Gauges []struct {
		Name  string  `json:"Name"`
		Value float64 `json:"Value"`
	} `json:"Gauges"`
}

// fetchNomadMetrics fetches all gauge metrics from a Nomad /v1/metrics endpoint
// and returns them as a name→value map.
func fetchNomadMetrics(ctx context.Context, client *http.Client, url string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var m nomadMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	out := make(map[string]float64, len(m.Gauges))
	for _, g := range m.Gauges {
		if g.Name != "" {
			out[g.Name] = g.Value
		}
	}
	return out, nil
}
