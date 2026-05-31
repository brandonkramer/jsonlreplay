package jsonlreplay

import (
	"bufio"
	"errors"
	"io"
	"os"
)

//
// ────────────────────────────────────────
// read api.
//

// MaxSeq returns the highest seq in path (and path+".1" when present). Missing files return 0.
func MaxSeq(path string, ro ReadOptions) (int64, error) {
	return maxSeq(path, ro)
}

// NextSeq returns the sequence number for the next append (1-based). Empty or missing files return 1.
func NextSeq(path string, ro ReadOptions) (int64, error) {
	hi, err := maxSeq(path, ro)
	if err != nil {
		return 0, err
	}
	if hi == 0 {
		return 1, nil
	}
	return hi + 1, nil
}

func collectReplay(path string, sinceSeq int64, limit int, ro ReadOptions) ([]Event, error) {
	var out []Event
	err := scanFile(path, ro, func(ev Event) error {
		if sinceSeq > 0 && ev.Seq <= sinceSeq {
			return nil
		}
		out = append(out, ev)
		if limit > 0 && len(out) >= limit {
			return errScanStop
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadAll loads every event from path in file order (active segment only).
func ReadAll(path string, ro ReadOptions) ([]Event, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Event
	err := scanFile(path, ro, func(ev Event) error {
		out = append(out, ev)
		return nil
	})
	return out, err
}

// Replay returns events with seq > sinceSeq, optionally limited (limit <= 0 means no limit).
// Reads the active file in a single pass without loading unrelated lines into memory first.
func Replay(path string, sinceSeq int64, limit int, ro ReadOptions) ([]Event, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return collectReplay(path, sinceSeq, limit, ro)
}

// FilterReplay filters an in-memory slice (useful when events are already loaded).
func FilterReplay(all []Event, sinceSeq int64, limit int) []Event {
	out := make([]Event, 0, len(all))
	for _, ev := range all {
		if sinceSeq > 0 && ev.Seq <= sinceSeq {
			continue
		}
		out = append(out, ev)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ReplayIter streams replay results without loading the full log into memory.
type ReplayIter struct {
	f        *os.File
	rd       *bufio.Reader
	sinceSeq int64
	limit    int
	seen     int
	ro       ReadOptions
	eof      bool
}

// OpenReplayIter opens path for sequential Next calls. Missing files yield an exhausted iterator.
func OpenReplayIter(path string, sinceSeq int64, limit int, ro ReadOptions) (*ReplayIter, error) {
	it := &ReplayIter{sinceSeq: sinceSeq, limit: limit, ro: ro}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			it.eof = true
			return it, nil
		}
		return nil, err
	}
	it.f = f
	it.rd = bufio.NewReader(f)
	return it, nil
}

// Next returns the next matching event. Io.EOF means iteration is complete.
func (it *ReplayIter) Next() (Event, error) {
	if it.eof {
		return Event{}, io.EOF
	}
	maxBytes := it.ro.maxLineBytes()
	for {
		line, err := readLine(it.rd, maxBytes)
		if errors.Is(err, io.EOF) {
			it.eof = true
			return Event{}, io.EOF
		}
		if err != nil {
			return Event{}, err
		}
		if len(line) == 0 {
			continue
		}
		ev, err := decodeLine(line, it.ro)
		if errors.Is(err, errSkipLine) {
			continue
		}
		if err != nil {
			return Event{}, err
		}
		if it.sinceSeq > 0 && ev.Seq <= it.sinceSeq {
			continue
		}
		it.seen++
		if it.limit > 0 && it.seen > it.limit {
			it.eof = true
			return Event{}, io.EOF
		}
		return ev, nil
	}
}

// Close closes the underlying file.
func (it *ReplayIter) Close() error {
	if it.f == nil {
		return nil
	}
	err := it.f.Close()
	it.f = nil
	it.eof = true
	return err
}
