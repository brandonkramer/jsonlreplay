package jsonlreplay_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brandonkramer/jsonlreplay"
)

func TestPollWaitsForNewEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("a", "1"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	var got []jsonlreplay.Event
	var hi int64
	var pollErr error
	go func() {
		defer close(done)
		got, hi, pollErr = jsonlreplay.Poll(ctx, path, 1, time.Second, 0, jsonlreplay.ReadOptions{}, 20*time.Millisecond)
	}()

	time.Sleep(80 * time.Millisecond)
	if _, err := w.AppendText("b", "2"); err != nil {
		t.Fatal(err)
	}
	<-done
	if pollErr != nil {
		t.Fatal(pollErr)
	}
	if len(got) != 1 || got[0].Seq != 2 || hi != 2 {
		t.Fatalf("got=%v hi=%d", got, hi)
	}
	w.Close()
}

func TestMaxSeqWithArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path+".1", []byte("{\"seq\":5,\"kind\":\"old\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"seq\":1,\"kind\":\"new\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hi, err := jsonlreplay.MaxSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || hi != 5 {
		t.Fatalf("hi=%d err=%v", hi, err)
	}
	next, err := jsonlreplay.NextSeq(path, jsonlreplay.ReadOptions{})
	if err != nil || next != 6 {
		t.Fatalf("next=%d err=%v", next, err)
	}
}
