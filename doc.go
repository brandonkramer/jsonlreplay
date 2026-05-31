// Package jsonlreplay is an append-only JSONL log with monotonic sequence numbers and replay cursors.
//
// Write path: Open, Append, AppendJSON, AppendAs, AppendText, AppendData, Close.
// Read path: MaxSeq, NextSeq, Replay, ReplayRaw, OpenReplayIter, OpenRawIter, Poll, ReadAll.
//
// Each line is one JSON object. The default Event shape uses seq, time, kind, text, and data.
// Use AppendJSON / ReplayRaw for your own schema, or AppendAs with a custom Codec.
// Durability is controlled by Options.Durability (write, flush, or fsync per append).
package jsonlreplay
