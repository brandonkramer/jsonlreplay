package jsonlreplay_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/brandonkramer/jsonlreplay"
)

func TestReplayIterMatchesReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if _, err := w.AppendText("e", string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	want, err := jsonlreplay.Replay(path, 2, 2, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	it, err := jsonlreplay.OpenReplayIter(path, 2, 2, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	var got []jsonlreplay.Event
	for {
		ev, err := it.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, ev)
	}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i].Seq != want[i].Seq {
			t.Fatalf("i=%d got=%v want=%v", i, got[i], want[i])
		}
	}
}

func TestReplayStreamingLargeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 1000; i++ {
		if _, err := fmt.Fprintf(f, "{\"seq\":%d,\"kind\":\"x\"}\n", i); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	got, err := jsonlreplay.Replay(path, 998, 0, jsonlreplay.ReadOptions{})
	if err != nil || len(got) != 2 || got[0].Seq != 999 {
		t.Fatalf("len=%d got=%+v err=%v", len(got), got, err)
	}
}
