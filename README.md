# jsonlreplay

Append-only JSONL log with monotonic sequence numbers and incremental replay (`sinceSeq`, `limit`, `Poll`).

Each line is one JSON object (`seq`, optional `time`, `kind`, `text`, `data`) so logs stay easy to inspect with `jq`, `rg`, or an editor.

## Install

From [pkg.go.dev](https://pkg.go.dev/github.com/brandonkramer/jsonlreplay):

```bash
go get github.com/brandonkramer/jsonlreplay
```

## Quick start

```go
w, err := jsonlreplay.Open("/var/log/myapp/events.jsonl", jsonlreplay.Options{CreateDir: true})
if err != nil {
    return err
}
defer w.Close()

ev, err := w.AppendText("started", "worker online")

events, err := jsonlreplay.Replay(w.Path(), 0, 50, jsonlreplay.ReadOptions{})
hi, err := jsonlreplay.MaxSeq(w.Path(), jsonlreplay.ReadOptions{})

batch, highSeq, err := jsonlreplay.Poll(ctx, w.Path(), hi, 2*time.Second, 50, jsonlreplay.ReadOptions{}, 0)
```

## Writer API

| Function | Purpose |
| --- | --- |
| `Open(path, opts)` | Open or create log; resume `seq` from existing file |
| `Append(ev)` | Append event (assigns `seq` / `time` when empty) |
| `AppendText(kind, text)` | Shorthand text event |
| `AppendData(kind, data)` | JSON-marshal `data` into `data` field |
| `Close()` | Close file |

`Writer` is safe for concurrent `Append` calls — sequence numbers are assigned under a mutex.

## Read API

| Function | Purpose |
| --- | --- |
| `MaxSeq(path, ro)` | Highest `seq` in the active file and `path+".1"` when present |
| `NextSeq(path, ro)` | Next `seq` for append (1 on empty file) |
| `ReadAll(path, ro)` | Load all events from the active file |
| `Replay(path, sinceSeq, limit, ro)` | Stream replay: `seq > sinceSeq`, optional cap (single pass) |
| `OpenReplayIter` / `Next` / `Close` | Iterator replay without loading the full log |
| `Poll(ctx, path, sinceSeq, timeout, limit, ro, interval)` | Wait for new `seq`, then replay |
| `FilterReplay(all, sinceSeq, limit)` | In-memory pagination |

## Options

**Writer (`Options`)**

- `MaxLineBytes` — cap encoded line size (default 4 MiB)
- `MaxFileBytes` — rotate active file to `path+".1"` before append when over cap (0 disables)
- `CorruptLines` — `CorruptSkip` (default) or `CorruptError` when scanning on open
- `Durability` — `DurabilityWrite` (default), `DurabilityFlush`, or `DurabilityFsync`
- `Clock` — inject time for tests (`Event.Time` when empty)
- `FileMode` — created file mode (default `0644`)
- `CreateDir` — `MkdirAll` parent directories with `0755` before open

**Reader (`ReadOptions`)**

- `MaxLineBytes`, `CorruptLines` — same semantics while replaying

## Durability

| Mode | Behavior |
| --- | --- |
| `DurabilityWrite` | Returns after `Write` (may lose recent lines on crash) |
| `DurabilityFlush` / `DurabilityFsync` | `Sync` after each successful append |

**Rotation:** only the active `path` is replayed. `NextSeq` / `MaxSeq` also scan `path+".1"` so sequence numbers stay continuous after rotation.

## Non-goals

- Cross-process file locking (single `Writer` per path in one process)
- Cross-process tailing of a live log
- Compaction or indexing beyond single-file rotation

## Development

Lefthook and golangci-lint are pinned in `go.mod` as **tools** (dev-only; not library dependencies). Install git hooks once per clone:

```bash
make install-hooks
```

That runs `go tool lefthook install`. Hooks and `make lint` use `go tool golangci-lint` from the same `go.mod` pins. CI runs `./scripts/check.sh` (no lefthook required).

```bash
make check
make test
make lint
```
