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
// scan.
//

// ErrLineTooLong is returned when a line exceeds MaxLineBytes.
var ErrLineTooLong = errors.New("jsonlreplay: line exceeds max size")

// ErrCorruptLine is returned when CorruptError is set and a line cannot be decoded.
var ErrCorruptLine = errors.New("jsonlreplay: corrupt line")

var (
	errScanStop = errors.New("jsonlreplay: scan stop")
	errSkipLine = errors.New("jsonlreplay: skip line")
)

func readLine(r *bufio.Reader, maxBytes int) ([]byte, error) {
	var line []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(line) == 0 {
					return nil, io.EOF
				}
				return line, nil
			}
			return nil, err
		}
		if b == '\n' {
			return line, nil
		}
		line = append(line, b)
		if len(line) > maxBytes {
			return nil, fmt.Errorf("%w (%d bytes)", ErrLineTooLong, maxBytes)
		}
	}
}

func decodeLine(line []byte, ro ReadOptions) (Event, error) {
	var ev Event
	if err := json.Unmarshal(line, &ev); err != nil {
		switch ro.CorruptLines {
		case CorruptError:
			return Event{}, fmt.Errorf("%w: %v", ErrCorruptLine, err)
		default:
			return Event{}, errSkipLine
		}
	}
	return ev, nil
}

func scanFile(path string, ro ReadOptions, fn func(Event) error) error {
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
		ev, err := decodeLine(line, ro)
		if errors.Is(err, errSkipLine) {
			continue
		}
		if err != nil {
			return err
		}
		if err := fn(ev); err != nil {
			if errors.Is(err, errScanStop) {
				return nil
			}
			return err
		}
	}
}

func maxSeq(path string, ro ReadOptions) (int64, error) {
	var hi int64
	for _, p := range ro.rotator().ScanPaths(path) {
		err := scanFileRaw(p, ro, func(_ []byte, seq int64) error {
			if seq > hi {
				hi = seq
			}
			return nil
		})
		if err != nil {
			return 0, err
		}
	}
	return hi, nil
}
