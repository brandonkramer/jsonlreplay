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

ev, err := w.AppendText("started", "work")

events, err := jsonlreplay.Replay(w.Path(), 0, 50, jsonlreplay.ReadOptions{})
hi, err := jsonlreplay.MaxSeq(w.Path(), jsonlreplay.ReadOptions{})

batch, highSeq, err := jsonlreplay.Poll(ctx, w.Path(), hi, 2*time.Second, 50, jsonlreplay.ReadOptions{}, 0)
```

## Custom JSON shape

Each line is one JSON object. Include a numeric **`seq`** so replay and polling (`sinceSeq`, `Poll`, `MaxSeq`) can resume where you left off—the rest of the fields are up to you.

### Raw JSON (`AppendJSON` / `ReplayRaw`)

Use when your events are not the built-in `Event` type:

```go
line, err := w.AppendJSON(json.RawMessage(`{"type":"order.created","id":"42"}`))
// → {"type":"order.created","id":"42","seq":1}

rows, err := jsonlreplay.ReplayRaw(path, since, 50, jsonlreplay.ReadOptions{})
for _, row := range rows {
    var order OrderEvent
    json.Unmarshal(row, &order)
}
```

`ReadAllRaw`, `OpenRawIter`, and `NextRaw` stream bytes without decoding into `Event`.

### Typed records (`AppendAs` + `Codec`)

```go
type Row struct {
    Seq    int64  `json:"seq"`
    Action string `json:"action"`
}
got, err := jsonlreplay.AppendAs(w, Row{Action: "login"})
```

`Options.Codec` defaults to `EventCodec` (`encoding/json`). Implement `Codec` for custom encoding (protobuf JSON, redaction, versioning wrappers).

### Built-in `Event` (convenience)

`Append`, `AppendText`, and `AppendData` use `kind` / `text` / `data` — best for agent/tool transcripts and simple logs.

| Approach | When |
| --- | --- |
| `Event` + `AppendText` | Tool summaries, chatty agent logs |
| `AppendJSON` | Arbitrary JSON, metrics blobs, API audit rows |
| `AppendAs` + custom struct | Strongly typed domain events |
| `Codec` | Non-standard encoding while keeping seq/replay machinery |

## Rotation

Default: `SingleRotator` archives to `path+".1"` when `MaxFileBytes` is exceeded. Implement `Rotator` for dated segments (`.2025-05-31`), tiered archives, or object-store offload:

```go
type Rotator interface {
    MaybeRotate(path string, maxBytes int64, lineLen int) (rotated bool, err error)
    ScanPaths(path string) []string // active + archives, for MaxSeq / NextSeq
}
```

Set `Options.Rotator` and `ReadOptions.Rotator` to the same implementation so append and replay agree on segments.


## Writer API

| Function | Purpose |
| --- | --- |
| `Open(path, opts)` | Open or create log; resume `seq` from existing file |
| `Append(ev)` | Append event (assigns `seq` / `time` when empty) |
| `AppendText(kind, text)` | Shorthand text event |
| `AppendData(kind, data)` | JSON-marshal `data` into `data` field |
| `AppendJSON(line)` | Append any JSON object; assigns `seq` when missing |
| `AppendAs(w, v)` | Marshal via `Codec`, inject `seq`, append |
| `Close()` | Close file |

`Writer` is safe for concurrent `Append` calls — sequence numbers are assigned under a mutex.

## Read API

| Function | Purpose |
| --- | --- |
| `MaxSeq(path, ro)` | Highest `seq` in the active file and `path+".1"` when present |
| `NextSeq(path, ro)` | Next `seq` for append (1 on empty file) |
| `ReadAll(path, ro)` | Load all events from the active file |
| `Replay(path, sinceSeq, limit, ro)` | Stream replay: `seq > sinceSeq`, optional cap (single pass) |
| `ReplayRaw(path, sinceSeq, limit, ro)` | Replay as raw JSON bytes per line |
| `ReadAllRaw(path, ro)` | Load active file as `[][]byte` |
| `OpenReplayIter` / `Next` / `Close` | Iterator replay without loading the full log |
| `OpenRawIter` / `NextRaw` / `Close` | Iterator over raw JSON lines |
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
- `Codec` — marshal/unmarshal for `AppendAs` (default `EventCodec`)
- `Rotator` — archive policy (default `SingleRotator` → `path+".1"`)

**Reader (`ReadOptions`)**

- `MaxLineBytes`, `CorruptLines` — same semantics while replaying
- `Rotator` — archive paths scanned for `MaxSeq` / `NextSeq`

## Durability

| Mode | Behavior |
| --- | --- |
| `DurabilityWrite` | Returns after `Write` (may lose recent lines on crash) |
| `DurabilityFlush` / `DurabilityFsync` | `Sync` after each successful append |

**Rotation:** only the active `path` is replayed. `NextSeq` / `MaxSeq` also scan `path+".1"` so sequence numbers stay continuous after rotation.

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
