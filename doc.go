// Package jsonlreplay is an append-only JSONL log with monotonic sequence numbers and replay cursors.
//
// Write path: Open, Append, AppendText, AppendData, Close.
// Read path: MaxSeq, NextSeq, Replay, OpenReplayIter, Poll, ReadAll.
//
// Each line is one JSON object (seq, optional time, kind, text, data) for use with jq, rg, or editors.
// Durability is controlled by Options.Durability (write, flush, or fsync per append).
// Set Options.CreateDir to create parent directories before open.
package jsonlreplay
