package jsonlreplay

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

//
// ────────────────────────────────────────
// raw jsonl api.
//

// ReadAllRaw loads every JSON line from the active file (bytes per line, no Event decode).
func ReadAllRaw(path string, ro ReadOptions) ([][]byte, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out [][]byte
	err := scanFileRaw(path, ro, func(line []byte, _ int64) error {
		out = append(out, append([]byte(nil), line...))
		return nil
	})
	return out, err
}

// ReplayRaw returns lines with seq > sinceSeq. Each slice is a copy of the JSON object bytes.
func ReplayRaw(path string, sinceSeq int64, limit int, ro ReadOptions) ([][]byte, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out [][]byte
	err := scanFileRaw(path, ro, func(line []byte, seq int64) error {
		if sinceSeq > 0 && seq > 0 && seq <= sinceSeq {
			return nil
		}
		if sinceSeq > 0 && seq == 0 {
			return nil
		}
		out = append(out, append([]byte(nil), line...))
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

// RawIter streams raw JSON lines without loading the full log.
type RawIter struct {
	f        *os.File
	rd       *bufio.Reader
	sinceSeq int64
	limit    int
	seen     int
	ro       ReadOptions
	eof      bool
}

// OpenRawIter opens path for sequential NextRaw calls.
func OpenRawIter(path string, sinceSeq int64, limit int, ro ReadOptions) (*RawIter, error) {
	it := &RawIter{sinceSeq: sinceSeq, limit: limit, ro: ro}
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

// NextRaw returns the next JSON object bytes. Io.EOF means iteration is complete.
func (it *RawIter) NextRaw() (json.RawMessage, error) {
	if it.eof {
		return nil, io.EOF
	}
	maxBytes := it.ro.maxLineBytes()
	for {
		line, err := readLine(it.rd, maxBytes)
		if errors.Is(err, io.EOF) {
			it.eof = true
			return nil, io.EOF
		}
		if err != nil {
			return nil, err
		}
		if len(line) == 0 {
			continue
		}
		seq, _, seqErr := seqFromLine(line)
		if seqErr != nil {
			switch it.ro.CorruptLines {
			case CorruptError:
				return nil, fmt.Errorf("%w: %v", ErrCorruptLine, seqErr)
			default:
				continue
			}
		}
		if it.sinceSeq > 0 && seq > 0 && seq <= it.sinceSeq {
			continue
		}
		if it.sinceSeq > 0 && seq == 0 {
			continue
		}
		it.seen++
		if it.limit > 0 && it.seen > it.limit {
			it.eof = true
			return nil, io.EOF
		}
		out := append([]byte(nil), line...)
		return json.RawMessage(out), nil
	}
}

// Close closes the underlying file.
func (it *RawIter) Close() error {
	if it.f == nil {
		return nil
	}
	err := it.f.Close()
	it.f = nil
	it.eof = true
	return err
}

func scanFileRaw(path string, ro ReadOptions, fn func([]byte, int64) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	maxBytes := ro.maxLineBytes()
	rd := bufio.NewReader(f)
	for {
		line, err := readLine(rd, maxBytes)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(line) == 0 {
			continue
		}
		seq, _, seqErr := seqFromLine(line)
		if seqErr != nil {
			switch ro.CorruptLines {
			case CorruptError:
				return fmt.Errorf("%w: %v", ErrCorruptLine, seqErr)
			default:
				continue
			}
		}
		if err := fn(line, seq); err != nil {
			if errors.Is(err, errScanStop) {
				return nil
			}
			return err
		}
	}
}
