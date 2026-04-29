package holodeck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	hclog "github.com/hashicorp/go-hclog"
)

// ObserverClient sends events to the Observer service on a best-effort basis.
// All sends are fire-and-forget; errors are logged but never returned to the caller.
type ObserverClient struct {
	addr   string
	logger hclog.Logger
	http   *http.Client
}

// NewObserverClient creates an ObserverClient targeting addr. An empty addr
// disables all sending (no-op), which allows the Holodeck to run without an Observer.
func NewObserverClient(addr string, logger hclog.Logger) *ObserverClient {
	return &ObserverClient{
		addr:   addr,
		logger: logger,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

type observerEvent struct {
	Source  string    `json:"source"`
	Kind    string    `json:"kind"`
	SentAt  time.Time `json:"sent_at"`
	Payload any       `json:"payload"`
}

// Send fires and forgets an event to the Observer. Safe to call concurrently.
func (c *ObserverClient) Send(kind string, payload any) {
	if c.addr == "" {
		return
	}
	go func() {
		if err := c.send(kind, payload); err != nil {
			c.logger.Warn("failed to send observer event", "kind", kind, "error", err)
		}
	}()
}

func (c *ObserverClient) send(kind string, payload any) error {
	body, err := json.Marshal(observerEvent{
		Source:  "holodeck",
		Kind:    kind,
		SentAt:  time.Now().UTC(),
		Payload: payload,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := c.http.Post(c.addr+"/v1/events", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
