package jsonlreplay

import (
	"encoding/json"
	"os"
)

// Test hooks (overridden in package tests for error-path coverage).
var (
	eventJSONMarshal = func(ev Event) ([]byte, error) { return json.Marshal(ev) }
	logOpenFile      = os.OpenFile
	logFileWrite     = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
	logFileSync      = func(f *os.File) error { return f.Sync() }
	logFileClose     = func(f *os.File) error { return f.Close() }
	replayPoll       = Replay
)
