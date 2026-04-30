package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	nomadapi "github.com/hashicorp/nomad/api"
)

var streamTopics = map[nomadapi.Topic][]string{
	nomadapi.TopicJob:        {"*"},
	nomadapi.TopicAllocation: {"*"},
	nomadapi.TopicNode:       {"*"},
}

// StreamListener subscribes to the Nomad event stream and stores received
// events. It reconnects from the last seen index on error.
type StreamListener struct {
	client *nomadapi.Client
	store  EventStore
	logger hclog.Logger
}

func NewStreamListener(client *nomadapi.Client, store EventStore, logger hclog.Logger) *StreamListener {
	return &StreamListener{client: client, store: store, logger: logger}
}

// Run subscribes to the Nomad event stream and stores events until ctx is
// cancelled. On error, it reconnects from the last seen Nomad index with
// exponential backoff (1s–30s).
func (l *StreamListener) Run(ctx context.Context) error {
	var lastIndex uint64
	backoff := time.Second

	for ctx.Err() == nil {
		ch, err := l.client.EventStream().Stream(ctx, streamTopics, lastIndex, nil)
		if err != nil {
			l.logger.Error("stream connect error", "error", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		l.logger.Info("stream connected", "index", lastIndex)
		backoff = time.Second // reset on successful connect

		for events := range ch {
			if events.Err != nil {
				l.logger.Error("stream error", "error", events.Err, "last_index", lastIndex)
				break
			}
			lastIndex = events.Index
			for _, e := range events.Events {
				if err := l.ingest(e); err != nil {
					l.logger.Error("failed to ingest event", "error", err, "topic", e.Topic, "type", e.Type)
				}
			}
		}
	}
	return ctx.Err()
}

func (l *StreamListener) ingest(e nomadapi.Event) error {
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	kind := fmt.Sprintf("nomad_%s_%s",
		strings.ToLower(string(e.Topic)),
		strings.ToLower(e.Type),
	)
	ingestTime := time.Now().UTC()
	l.store.Add(EventInput{
		Source:  "nomad",
		Kind:    kind,
		Payload: payload,
		Summary: BuildSummary(kind, &ingestTime, payload),
		SentAt:  &ingestTime,
	})
	return nil
}
