package holodeck

import (
	"fmt"
	"time"
)

// ScheduleRequest describes a time-series of metric rule saves.
type ScheduleRequest struct {
	Name            string  `json:"name"`
	StartValue      float64 `json:"start_value"`
	Count           int     `json:"count"`
	Delta           string  `json:"delta"`           // "static" | "linear"
	ChangeToValue   float64 `json:"change_to_value"` // per-step value delta for "linear"
	IntervalSeconds float64 `json:"interval_seconds"`

	// Optional MetricRule modifiers.
	LagSeconds               float64 `json:"lag_seconds,omitempty"`
	Saturation               float64 `json:"saturation,omitempty"`
	DiminishingReturnsFactor float64 `json:"diminishing_returns_factor,omitempty"`
}

// runSchedule writes the metric rule n times, waiting IntervalSeconds between
// each save. The first save is immediate. Stops early if ctx is cancelled.
func (s *Server) runSchedule(worldID string, req ScheduleRequest) {
	for i := 0; i < req.Count; i++ {
		change := req.ChangeToValue
		if req.Delta == "linear" {
			change = req.StartValue
		}
		value := req.StartValue + (change * float64((i + 1)))
		rule := MetricRule{
			Type:                     "repeated",
			Value:                    value,
			Delta:                    req.Delta,
			Count:                    req.Count,
			LagSeconds:               req.LagSeconds,
			Saturation:               req.Saturation,
			DiminishingReturnsFactor: req.DiminishingReturnsFactor,
		}
		var staleModifier = i - 1
		if i == 0 {
			s.manager.SetOne(worldID, req.Name, rule)
			staleModifier = 1
		} else {
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(time.Duration(float64(time.Second) * req.IntervalSeconds)):

			}
		}

		s.manager.MakeRuleOrStale(worldID, req.Name, fmt.Sprintf("%s_%d", req.Name, staleModifier), rule)

	}
}
