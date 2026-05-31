package jsonlreplay

import (
	"context"
	"fmt"
	"time"
)

//
// ────────────────────────────────────────
// poll.
//

// Poll waits until MaxSeq(path) exceeds sinceSeq or timeout elapses, then replays new events.
// HighSeq is the current log high-water mark (use as the next sinceSeq cursor).
// Limit <= 0 means no cap on returned events. Interval <= 0 uses DefaultPollInterval.
func Poll(
	ctx context.Context,
	path string,
	sinceSeq int64,
	timeout time.Duration,
	limit int,
	ro ReadOptions,
	interval time.Duration,
) (events []Event, highSeq int64, err error) {
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return pollReplay(ctx, path, sinceSeq, limit, ro, 0)
		}
		hi, err := MaxSeq(path, ro)
		if err != nil {
			return nil, 0, fmt.Errorf("poll max seq: %w", err)
		}
		if hi > sinceSeq || time.Now().After(deadline) {
			events, err = replayPoll(path, sinceSeq, limit, ro)
			if err != nil {
				return nil, 0, fmt.Errorf("poll replay: %w", err)
			}
			return events, hi, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return pollReplay(ctx, path, sinceSeq, limit, ro, hi)
		case <-timer.C:
		}
	}
}

func pollReplay(ctx context.Context, path string, sinceSeq int64, limit int, ro ReadOptions, hi int64) ([]Event, int64, error) {
	events, err := replayPoll(path, sinceSeq, limit, ro)
	if err != nil {
		return nil, 0, fmt.Errorf("poll replay: %w", err)
	}
	return events, hi, ctx.Err()
}
