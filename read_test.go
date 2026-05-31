package jsonlreplay_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandonkramer/jsonlreplay"
)

func TestReplaySinceSeqAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if _, err := w.Append(jsonlreplay.Event{Kind: "e", Text: string(rune('a' + i))}); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	got, err := jsonlreplay.Replay(path, 2, 0, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Seq != 3 || got[1].Seq != 4 {
		t.Fatalf("got=%+v", got)
	}

	got, err = jsonlreplay.Replay(path, 0, 2, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Seq != 1 || got[1].Seq != 2 {
		t.Fatalf("got=%+v", got)
	}
}

func TestNextSeqEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonl")
	seq, err := jsonlreplay.NextSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || seq != 1 {
		t.Fatalf("seq=%d err=%v", seq, err)
	}
}

func TestReadLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	long := make([]byte, 128)
	for i := range long {
		long[i] = 'x'
	}
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{MaxLineBytes: 64})
	if !errors.Is(err, jsonlreplay.ErrLineTooLong) {
		t.Fatalf("err=%v", err)
	}
}

func TestCorruptLineSkip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := "{\"seq\":1,\"kind\":\"ok\"}\nnot json\n{\"seq\":2,\"kind\":\"ok\"}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptSkip})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[1].Seq != 2 {
		t.Fatalf("events=%+v", events)
	}
}

func TestCorruptLineError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptError})
	if !errors.Is(err, jsonlreplay.ErrCorruptLine) {
		t.Fatalf("err=%v", err)
	}
}

func TestAppendDataRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]int{"n": 42}
	if _, err := w.AppendData("metric", payload); err != nil {
		t.Fatal(err)
	}
	w.Close()
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil || len(events) != 1 || events[0].Kind != "metric" {
		t.Fatalf("events=%+v err=%v", events, err)
	}
}
