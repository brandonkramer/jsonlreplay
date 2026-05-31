package jsonlreplay_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/brandonkramer/jsonlreplay"
)

func TestWriterConcurrentAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{
		Clock: func() time.Time { return time.Unix(100, 0).UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if _, err := w.AppendText("msg", "x"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	next, err := jsonlreplay.NextSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || next != n+1 {
		t.Fatalf("next=%d err=%v", next, err)
	}
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != n {
		t.Fatalf("len=%d", len(events))
	}
	seen := make(map[int64]bool, n)
	for _, ev := range events {
		if seen[ev.Seq] {
			t.Fatalf("duplicate seq %d", ev.Seq)
		}
		seen[ev.Seq] = true
	}
	for i := int64(1); i <= n; i++ {
		if !seen[i] {
			t.Fatalf("missing seq %d", i)
		}
	}
}

func TestWriterReopenContinuity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fixed := time.Unix(200, 0).UTC()
	opts := jsonlreplay.Options{Clock: func() time.Time { return fixed }}

	w1, err := jsonlreplay.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := w1.AppendText("a", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}

	w2, err := jsonlreplay.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	for range 5 {
		ev, err := w2.AppendText("b", "2")
		if err != nil {
			t.Fatal(err)
		}
		if ev.Seq < 6 || ev.Seq > 10 {
			t.Fatalf("seq=%d", ev.Seq)
		}
	}
	next, err := jsonlreplay.NextSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || next != 11 {
		t.Fatalf("next=%d err=%v", next, err)
	}
}

func TestWriterMaxLineSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{MaxLineBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = w.AppendText("big", string(make([]byte, 64)))
	if !errors.Is(err, jsonlreplay.ErrLineTooLong) {
		t.Fatalf("err=%v", err)
	}
}

func TestWriterDurabilityFsync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{Durability: jsonlreplay.DurabilityFsync})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("ping", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestOpenCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{CreateDir: true})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRotationPreservesSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{MaxFileBytes: 48})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := w.AppendText("x", "1234567890"); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	next, err := jsonlreplay.NextSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || next <= 3 {
		t.Fatalf("next=%d err=%v", next, err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatal("expected archived segment", err)
	}
}
