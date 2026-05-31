package jsonlreplay_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandonkramer/jsonlreplay"
)

type auditRow struct {
	Seq    int64  `json:"seq"`
	Action string `json:"action"`
	Actor  string `json:"actor"`
}

func TestAppendJSONAndReplayRaw(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	custom, err := w.AppendJSON(json.RawMessage(`{"action":"login","actor":"u1"}`))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(custom, &m); err != nil {
		t.Fatal(err)
	}
	if m["seq"].(float64) != 1 {
		t.Fatalf("seq=%v", m["seq"])
	}
	w.Close()

	lines, err := jsonlreplay.ReplayRaw(path, 0, 0, jsonlreplay.ReadOptions{})
	if err != nil || len(lines) != 1 {
		t.Fatalf("lines=%v err=%v", lines, err)
	}
	if err := json.Unmarshal(lines[0], &m); err != nil {
		t.Fatal(err)
	}
	if m["action"] != "login" {
		t.Fatalf("m=%v", m)
	}
}

func TestAppendAsCustomStruct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonlreplay.AppendAs(w, auditRow{Action: "save", Actor: "u2"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Seq != 1 || got.Action != "save" {
		t.Fatalf("got=%+v", got)
	}
	w.Close()
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil || len(events) != 1 || events[0].Seq != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestReadAllRawAndRawIter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendJSON(json.RawMessage(`{"k":"a"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendJSON(json.RawMessage(`{"k":"b"}`)); err != nil {
		t.Fatal(err)
	}
	w.Close()

	all, err := jsonlreplay.ReadAllRaw(path, jsonlreplay.ReadOptions{})
	if err != nil || len(all) != 2 {
		t.Fatalf("all=%v err=%v", all, err)
	}
	it, err := jsonlreplay.OpenRawIter(path, 1, 0, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	line, err := it.NextRaw()
	if err != nil || string(line) == "" {
		t.Fatalf("line=%s err=%v", line, err)
	}
	it.Close()
}

func TestSingleRotatorScanPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".1", []byte("{\"seq\":9}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hi, err := jsonlreplay.MaxSeq(path, jsonlreplay.ReadOptions{Rotator: jsonlreplay.SingleRotator{}})
	if err != nil || hi != 9 {
		t.Fatalf("hi=%d err=%v", hi, err)
	}
}
