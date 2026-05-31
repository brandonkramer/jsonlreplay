package jsonlreplay_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brandonkramer/jsonlreplay"
)

func TestFilterReplayLimitTrim(t *testing.T) {
	all := []jsonlreplay.Event{{Seq: 1}, {Seq: 2}, {Seq: 3}}
	got := jsonlreplay.FilterReplay(all, 0, 2)
	if len(got) != 2 || got[0].Seq != 1 || got[1].Seq != 2 {
		t.Fatalf("got=%v", got)
	}
}

func TestFilterReplay(t *testing.T) {
	all := []jsonlreplay.Event{
		{Seq: 1}, {Seq: 2}, {Seq: 3}, {Seq: 4},
	}
	got := jsonlreplay.FilterReplay(all, 2, 2)
	if len(got) != 2 || got[0].Seq != 3 || got[1].Seq != 4 {
		t.Fatalf("got=%v", got)
	}
	got = jsonlreplay.FilterReplay(all, 0, 0)
	if len(got) != 4 {
		t.Fatalf("got=%v", got)
	}
}

func TestWriterPathAndExplicitSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{FileMode: 0o600, Durability: jsonlreplay.DurabilityFlush})
	if err != nil {
		t.Fatal(err)
	}
	if w.Path() != path {
		t.Fatalf("path=%q", w.Path())
	}
	ev, err := w.Append(jsonlreplay.Event{Seq: 99, Kind: "x"})
	if err != nil || ev.Seq != 99 {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
	if _, err := w.AppendText("y", "z"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", st.Mode(), err)
	}
}

func TestWriterAppendAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	_, err = w.AppendText("a", "b")
	if !errors.Is(err, jsonlreplay.ErrClosed) {
		t.Fatalf("err=%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAppendDataMarshalError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = w.AppendData("bad", make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestReadAllAndReplayMissingAndDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "adir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ro := jsonlreplay.ReadOptions{}
	if events, err := jsonlreplay.ReadAll(filepath.Join(dir, "nope.jsonl"), ro); err != nil || events != nil {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if events, err := jsonlreplay.Replay(filepath.Join(dir, "nope.jsonl"), 0, 0, ro); err != nil || events != nil {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if _, err := jsonlreplay.ReadAll(dir, ro); err == nil {
		t.Fatal("expected readall directory error")
	}
	if _, err := jsonlreplay.Replay(dir, 0, 0, ro); err == nil {
		t.Fatal("expected replay directory error")
	}
}

func TestNextSeqErrorOnCorruptResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ro := jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptError}
	if _, err := jsonlreplay.NextSeq(path, ro); err == nil {
		t.Fatal("expected next seq error")
	}
	_, err := jsonlreplay.Open(path, jsonlreplay.Options{CorruptLines: jsonlreplay.CorruptError})
	if err == nil {
		t.Fatal("expected open resume error")
	}
}

func TestOpenCreateDirBlocked(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "sub", "events.jsonl")
	_, err := jsonlreplay.Open(path, jsonlreplay.Options{CreateDir: true})
	if err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestPollTimeoutAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("a", "1"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	ctx := context.Background()
	events, hi, err := jsonlreplay.Poll(ctx, path, 5, 10*time.Millisecond, 0, jsonlreplay.ReadOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 || hi != 1 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}

	events, hi, err = jsonlreplay.Poll(ctx, path, 0, time.Millisecond, 1, jsonlreplay.ReadOptions{}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || hi != 1 || events[0].Seq != 1 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}
}

func TestPollCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events, hi, err := jsonlreplay.Poll(ctx, path, 0, time.Second, 0, jsonlreplay.ReadOptions{}, time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if events != nil || hi != 0 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}
}

func TestPollCancelledWhileWaiting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var events []jsonlreplay.Event
	var hi int64
	var pollErr error
	go func() {
		defer close(done)
		events, hi, pollErr = jsonlreplay.Poll(ctx, path, 0, time.Second, 0, jsonlreplay.ReadOptions{}, time.Millisecond)
	}()
	time.Sleep(5 * time.Millisecond)
	cancel()
	<-done
	if !errors.Is(pollErr, context.Canceled) {
		t.Fatalf("err=%v", pollErr)
	}
	if events != nil || hi != 0 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}
}

func TestPollReplayForTest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events, hi, err := jsonlreplay.PollReplayForTest(ctx, path, 0, 0, jsonlreplay.ReadOptions{}, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if events != nil || hi != 0 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}
}

func TestOpenReplayIterMissingAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonl")
	it, err := jsonlreplay.OpenReplayIter(path, 0, 0, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = it.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err=%v", err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}
	if err := it.Close(); err != nil {
		t.Fatal(err)
	}

	locked := filepath.Join(t.TempDir(), "locked.jsonl")
	if err := os.WriteFile(locked, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := jsonlreplay.OpenReplayIter(locked, 0, 0, jsonlreplay.ReadOptions{}); err == nil {
		t.Fatal("expected open error")
	}
}

func TestReplayIterCorruptAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := "{\"seq\":1,\"kind\":\"a\"}\nbad\n{\"seq\":2,\"kind\":\"b\"}\n{\"seq\":3,\"kind\":\"c\"}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := jsonlreplay.OpenReplayIter(path, 0, 1, jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptSkip})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := it.Next()
	if err != nil || ev.Seq != 1 {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
	_, err = it.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err=%v", err)
	}

	it2, err := jsonlreplay.OpenReplayIter(path, 0, 0, jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptError})
	if err != nil {
		t.Fatal(err)
	}
	defer it2.Close()
	if _, err := it2.Next(); err != nil {
		t.Fatalf("first next: %v", err)
	}
	_, err = it2.Next()
	if !errors.Is(err, jsonlreplay.ErrCorruptLine) {
		t.Fatalf("err=%v", err)
	}
}

func TestReplayIterLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	long := make([]byte, 80)
	for i := range long {
		long[i] = 'x'
	}
	if err := os.WriteFile(path, long, 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := jsonlreplay.OpenReplayIter(path, 0, 0, jsonlreplay.ReadOptions{MaxLineBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close()
	_, err = it.Next()
	if !errors.Is(err, jsonlreplay.ErrLineTooLong) {
		t.Fatalf("err=%v", err)
	}
}

func TestRotateIfOverCapBranches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	ok, err := jsonlreplay.RotateIfOverCapForTest(path, 0, 10)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = jsonlreplay.RotateIfOverCapForTest(path, 100, 10)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = jsonlreplay.RotateIfOverCapForTest(path, 100, 1)
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = jsonlreplay.RotateIfOverCapForTest(path, 8, 4)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path2 := filepath.Join(sub, "two.jsonl")
	if err := os.WriteFile(path2, []byte("123456"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	_, err = jsonlreplay.RotateIfOverCapForTest(path2, 8, 4)
	_ = os.Chmod(sub, 0o755)
	if err == nil {
		t.Fatal("expected rename error")
	}

	badPath := filepath.Join(dir, string([]byte{0})+"x.jsonl")
	_, err = jsonlreplay.RotateIfOverCapForTest(badPath, 8, 4)
	if err == nil {
		t.Fatal("expected stat error")
	}
}

func TestScanFileOpenError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := jsonlreplay.ScanFileForTest(dir, jsonlreplay.ReadOptions{}, func(jsonlreplay.Event) error { return nil })
	if err == nil {
		t.Fatal("expected scan open error")
	}
}

func TestMaxSeqScanError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.MaxSeq(dir, jsonlreplay.ReadOptions{})
	if err == nil {
		t.Fatal("expected max seq error")
	}
}

func TestPollMaxSeqError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := jsonlreplay.Poll(context.Background(), dir, 0, time.Millisecond, 0, jsonlreplay.ReadOptions{}, time.Millisecond)
	if err == nil {
		t.Fatal("expected poll max seq error")
	}
}

func TestCollectReplayScanError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.Replay(path, 0, 0, jsonlreplay.ReadOptions{CorruptLines: jsonlreplay.CorruptError})
	if !errors.Is(err, jsonlreplay.ErrCorruptLine) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadLineWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1,\"kind\":\"x\"}"), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestReplayEmptyLinesSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := "\n\n{\"seq\":1,\"kind\":\"a\"}\n\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	events, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%v err=%v", events, err)
	}
}

func TestWriterRotateReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{MaxFileBytes: 20})
	if err != nil {
		t.Fatal(err)
	}
	for range 4 {
		if _, err := w.AppendText("x", "1234567890"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadAllStatError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.ReadAll(path, jsonlreplay.ReadOptions{})
	_ = os.Chmod(root, 0o755)
	if err == nil {
		t.Fatal("expected stat error")
	}
}

func TestReplayStatError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.Replay(path, 0, 0, jsonlreplay.ReadOptions{})
	_ = os.Chmod(root, 0o755)
	if err == nil {
		t.Fatal("expected stat error")
	}
}

func TestOpenReplayIterSinceSeqSkip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	body := "{\"seq\":1}\n{\"seq\":2}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := jsonlreplay.OpenReplayIter(path, 1, 0, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := it.Next()
	if err != nil || ev.Seq != 2 {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
	_, err = it.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err=%v", err)
	}
	it.Close()
}

func TestOpenReplayIterSkipsBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("\n\n{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	it, err := jsonlreplay.OpenReplayIter(path, 0, 0, jsonlreplay.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := it.Next()
	if err != nil || ev.Seq != 1 {
		t.Fatalf("ev=%+v err=%v", ev, err)
	}
	it.Close()
}

func TestOpenOnDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := jsonlreplay.Open(path, jsonlreplay.Options{})
	if err == nil {
		t.Fatal("expected open error")
	}
}

func TestWriterRotateReopenReadonlyDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	w, err := jsonlreplay.Open(path, jsonlreplay.Options{MaxFileBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("x", "1234567890"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	_, err = w.AppendText("x", "1234567890")
	_ = os.Chmod(root, 0o755)
	if err == nil {
		t.Fatal("expected rotate reopen error")
	}
	w.Close()
}
