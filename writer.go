package jsonlreplay

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//
// ────────────────────────────────────────
// writer.
//

// Writer appends JSONL events with monotonic sequence numbers.
type Writer struct {
	path string
	opts Options
	f    *os.File
	mu   sync.Mutex
	next int64
}

// Open opens or creates an append-only log at path and resumes sequence numbers from existing content.
func Open(path string, opts Options) (*Writer, error) {
	if opts.CreateDir {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create log dir: %w", err)
			}
		}
	}
	ro := ReadOptions{
		MaxLineBytes: opts.MaxLineBytes,
		CorruptLines: opts.CorruptLines,
	}
	next, err := NextSeq(path, ro)
	if err != nil {
		return nil, fmt.Errorf("resume seq: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(opts.fileMode()))
	if err != nil {
		return nil, fmt.Errorf("open log %s: %w", path, err)
	}
	return &Writer{path: path, opts: opts, f: f, next: next}, nil
}

// Path returns the log file path.
func (w *Writer) Path() string {
	return w.path
}

// Append writes ev, assigning Seq and Time when unset. Returns the stored event.
//
//nolint:gocritic // Event is the public value API; copying keeps callers simple.
func (w *Writer) Append(ev Event) (Event, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return Event{}, ErrClosed
	}

	if ev.Seq == 0 {
		ev.Seq = w.next
		w.next++
	} else if ev.Seq >= w.next {
		w.next = ev.Seq + 1
	}
	if ev.Time == "" {
		ev.Time = FormatTime(w.opts.clock()())
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event: %w", err)
	}
	lineLen := len(line) + 1
	if lineLen > w.opts.maxLineBytes() {
		return Event{}, fmt.Errorf("%w (%d bytes)", ErrLineTooLong, w.opts.maxLineBytes())
	}
	if err := w.rotateIfNeeded(lineLen); err != nil {
		return Event{}, err
	}
	if _, err := w.f.Write(append(line, '\n')); err != nil {
		return Event{}, fmt.Errorf("append log %s: %w", w.path, err)
	}
	if err := w.sync(); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// AppendText appends a text event with the given kind.
func (w *Writer) AppendText(kind, text string) (Event, error) {
	return w.Append(Event{Kind: kind, Text: text})
}

// AppendData appends an event with JSON-marshaled data.
func (w *Writer) AppendData(kind string, data any) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal data: %w", err)
	}
	return w.Append(Event{Kind: kind, Data: raw})
}

// Close closes the underlying file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	if err != nil {
		return fmt.Errorf("close log %s: %w", w.path, err)
	}
	return nil
}

func (w *Writer) rotateIfNeeded(lineLen int) error {
	rotated, err := rotateIfOverCap(w.path, w.opts.MaxFileBytes, lineLen)
	if err != nil || !rotated {
		return err
	}
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("close log %s: %w", w.path, err)
	}
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(w.opts.fileMode()))
	if err != nil {
		return fmt.Errorf("reopen log %s: %w", w.path, err)
	}
	w.f = f
	return nil
}

func (w *Writer) sync() error {
	switch w.opts.Durability {
	case DurabilityFlush, DurabilityFsync:
		if err := w.f.Sync(); err != nil {
			return fmt.Errorf("sync log %s: %w", w.path, err)
		}
		return nil
	default:
		return nil
	}
}

// ErrClosed is returned when appending after Close.
var ErrClosed = errors.New("jsonlreplay: writer closed")
