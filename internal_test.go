package jsonlreplay

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type failReader struct {
	fail bool
}

func (f *failReader) Read(p []byte) (int, error) {
	if !f.fail {
		f.fail = true
		return 0, errors.New("read failed")
	}
	return 0, io.EOF
}

func TestReadLineReadError(t *testing.T) {
	_, err := readLine(bufio.NewReader(&failReader{}), 1024)
	if err == nil || err.Error() != "read failed" {
		t.Fatalf("err=%v", err)
	}
}

func TestScanFileCallbackError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := scanFile(path, ReadOptions{}, func(Event) error {
		return errors.New("callback stop")
	})
	if err == nil || err.Error() != "callback stop" {
		t.Fatalf("err=%v", err)
	}
}

func TestPollReplayError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n{\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Poll(context.Background(), path, 0, 0, 1, ReadOptions{CorruptLines: CorruptError}, time.Millisecond)
	if err == nil {
		t.Fatal("expected poll replay error")
	}
}

func TestScanFileMissingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonl")
	if err := scanFile(path, ReadOptions{}, func(Event) error { return nil }); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestPollReplayReplayError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := pollReplay(context.Background(), dir, 0, 0, ReadOptions{}, 0)
	if err == nil {
		t.Fatal("expected replay error")
	}
}

func TestPollReplayErrorOnTimeout(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	_, _, err := Poll(context.Background(), path, 0, 0, 0, ReadOptions{}, time.Millisecond)
	_ = os.Chmod(root, 0o755)
	if err == nil {
		t.Fatal("expected poll replay stat error")
	}
}

func TestAppendMarshalHookError(t *testing.T) {
	old := eventJSONMarshal
	defer func() { eventJSONMarshal = old }()
	eventJSONMarshal = func(Event) ([]byte, error) { return nil, errors.New("marshal") }
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = w.AppendText("a", "b")
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestOpenFileHookError(t *testing.T) {
	old := logOpenFile
	defer func() { logOpenFile = old }()
	logOpenFile = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("open failed")
	}
	_, err := Open(filepath.Join(t.TempDir(), "events.jsonl"), Options{})
	if err == nil {
		t.Fatal("expected open error")
	}
}

func TestAppendWriteHookError(t *testing.T) {
	old := logFileWrite
	defer func() { logFileWrite = old }()
	logFileWrite = func(*os.File, []byte) (int, error) { return 0, errors.New("write failed") }
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = w.AppendText("a", "b")
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestAppendSyncHookError(t *testing.T) {
	old := logFileSync
	defer func() { logFileSync = old }()
	logFileSync = func(*os.File) error { return errors.New("sync failed") }
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{Durability: DurabilityFsync})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = w.AppendText("a", "b")
	if err == nil {
		t.Fatal("expected sync error")
	}
}

func TestCloseHookError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	old := logFileClose
	defer func() { logFileClose = old }()
	logFileClose = func(*os.File) error { return errors.New("close failed") }
	if err := w.Close(); err == nil {
		t.Fatal("expected close error")
	}
}

func TestRotateCloseHookError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{MaxFileBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("x", "1234567890"); err != nil {
		t.Fatal(err)
	}
	old := logFileClose
	defer func() { logFileClose = old }()
	logFileClose = func(*os.File) error { return errors.New("close failed") }
	_, err = w.AppendText("x", "1234567890")
	if err == nil {
		t.Fatal("expected rotate close error")
	}
	w.Close()
}

func TestPollReplayHookError(t *testing.T) {
	old := replayPoll
	defer func() { replayPoll = old }()
	replayPoll = func(string, int64, int, ReadOptions) ([]Event, error) {
		return nil, errors.New("replay failed")
	}
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Poll(context.Background(), path, 0, 0, 0, ReadOptions{}, time.Millisecond)
	if err == nil {
		t.Fatal("expected poll replay error")
	}
}

func TestPollReplayCancelledWithEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{\"seq\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	events, hi, err := pollReplay(ctx, path, 0, 0, ReadOptions{}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if len(events) != 1 || hi != 1 {
		t.Fatalf("events=%v hi=%d", events, hi)
	}
}

func TestRotateReopenHookError(t *testing.T) {
	oldOpen := logOpenFile
	defer func() { logOpenFile = oldOpen }()
	calls := 0
	logOpenFile = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		calls++
		if calls >= 2 {
			return nil, errors.New("reopen failed")
		}
		return oldOpen(path, flag, perm)
	}
	path := filepath.Join(t.TempDir(), "events.jsonl")
	w, err := Open(path, Options{MaxFileBytes: 16})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.AppendText("x", "1234567890"); err != nil {
		t.Fatal(err)
	}
	_, err = w.AppendText("x", "1234567890")
	if err == nil {
		t.Fatal("expected reopen error")
	}
	w.Close()
}
