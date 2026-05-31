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
		Rotator:      opts.Rotator,
	}
	next, err := NextSeq(path, ro)
	if err != nil {
		return nil, fmt.Errorf("resume seq: %w", err)
	}
	f, err := logOpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(opts.fileMode()))
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
func (w *Writer) Append(ev Event) (Event, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return Event{}, ErrClosed
	}
	w.assignEventSeq(&ev)
	if ev.Time == "" {
		ev.Time = FormatTime(w.opts.clock()())
	}
	line, err := eventJSONMarshal(ev)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event: %w", err)
	}
	if err := w.appendPrepared(line); err != nil {
		return Event{}, err
	}
	return ev, nil
}

// AppendJSON appends one JSON object. Assigns seq when missing; preserves other fields.
func (w *Writer) AppendJSON(line json.RawMessage) (json.RawMessage, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil, ErrClosed
	}
	b, err := w.prepareJSONLine([]byte(line))
	if err != nil {
		return nil, err
	}
	if err := w.appendPrepared(b); err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// AppendAs marshals v with Options.Codec, injects seq when missing, and appends the line.
func AppendAs[T any](w *Writer, v T) (T, error) {
	var zero T
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return zero, ErrClosed
	}
	line, err := w.opts.codec().Marshal(v)
	if err != nil {
		return zero, fmt.Errorf("marshal event: %w", err)
	}
	b, err := w.prepareJSONLine(line)
	if err != nil {
		return zero, err
	}
	if err := w.appendPrepared(b); err != nil {
		return zero, err
	}
	if err := w.opts.codec().Unmarshal(b, &v); err != nil {
		return zero, fmt.Errorf("unmarshal stored line: %w", err)
	}
	return v, nil
}

func (w *Writer) assignEventSeq(ev *Event) {
	if ev.Seq == 0 {
		ev.Seq = w.next
		w.next++
	} else if ev.Seq >= w.next {
		w.next = ev.Seq + 1
	}
}

func (w *Writer) prepareJSONLine(line []byte) ([]byte, error) {
	if seq, ok, err := seqFromLine(line); err != nil {
		return nil, fmt.Errorf("read seq: %w", err)
	} else if ok {
		if seq >= w.next {
			w.next = seq + 1
		}
		return line, nil
	}
	out, err := injectSeq(line, w.next, "", false)
	if err != nil {
		return nil, fmt.Errorf("inject seq: %w", err)
	}
	w.next++
	return out, nil
}

func (w *Writer) appendPrepared(line []byte) error {
	lineLen := len(line) + 1
	if lineLen > w.opts.maxLineBytes() {
		return fmt.Errorf("%w (%d bytes)", ErrLineTooLong, w.opts.maxLineBytes())
	}
	if err := w.rotateIfNeeded(lineLen); err != nil {
		return err
	}
	if _, err := logFileWrite(w.f, append(line, '\n')); err != nil {
		return fmt.Errorf("append log %s: %w", w.path, err)
	}
	return w.sync()
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
	err := logFileClose(w.f)
	w.f = nil
	if err != nil {
		return fmt.Errorf("close log %s: %w", w.path, err)
	}
	return nil
}

func (w *Writer) rotateIfNeeded(lineLen int) error {
	rotated, err := w.opts.rotator().MaybeRotate(w.path, w.opts.MaxFileBytes, lineLen)
	if err != nil || !rotated {
		return err
	}
	if err := logFileClose(w.f); err != nil {
		return fmt.Errorf("close log %s: %w", w.path, err)
	}
	f, err := logOpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.FileMode(w.opts.fileMode()))
	if err != nil {
		return fmt.Errorf("reopen log %s: %w", w.path, err)
	}
	w.f = f
	return nil
}

func (w *Writer) sync() error {
	switch w.opts.Durability {
	case DurabilityFlush, DurabilityFsync:
		if err := logFileSync(w.f); err != nil {
			return fmt.Errorf("sync log %s: %w", w.path, err)
		}
		return nil
	default:
		return nil
	}
}

// ErrClosed is returned when appending after Close.
var ErrClosed = errors.New("jsonlreplay: writer closed")
