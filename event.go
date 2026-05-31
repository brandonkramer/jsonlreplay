package jsonlreplay

//
// ────────────────────────────────────────
// event types.
//

import (
	"encoding/json"
	"time"
)

// Event is one JSONL record. Seq is 1-based and monotonic within a log file.
type Event struct {
	Seq  int64           `json:"seq"`
	Time string          `json:"time,omitempty"`
	Kind string          `json:"kind,omitempty"`
	Text string          `json:"text,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// FormatTime formats t as RFC3339 UTC for event timestamps.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
