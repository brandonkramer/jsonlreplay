package jsonlreplay

//
// ────────────────────────────────────────
// options.
//

import "time"

// CorruptLinePolicy controls how scan/replay handles invalid JSONL lines.
type CorruptLinePolicy int

const (
	// CorruptSkip ignores corrupt lines and continues scanning.
	CorruptSkip CorruptLinePolicy = iota
	// CorruptError stops with an error on the first corrupt line.
	CorruptError
)

// Durability controls how much of each append is forced to stable storage.
type Durability int

const (
	// DurabilityWrite returns after the kernel accepts the write (default).
	DurabilityWrite Durability = iota
	// DurabilityFlush calls (*os.File).Sync after each append.
	DurabilityFlush
	// DurabilityFsync calls (*os.File).Sync after each append (same syscall; named for intent).
	DurabilityFsync
)

// DefaultMaxLineBytes is the default maximum JSONL line size when MaxLineBytes is zero.
const DefaultMaxLineBytes = 4 << 20 // 4 MiB.

// DefaultPollInterval is the sleep between checks in Poll when the log is unchanged.
const DefaultPollInterval = 200 * time.Millisecond

// Options configures a Writer.
type Options struct {
	// MaxLineBytes caps encoded line size (including newline). Zero uses DefaultMaxLineBytes.
	MaxLineBytes int
	// MaxFileBytes rotates the log when the active file would exceed this size (zero disables).
	// The previous segment is renamed to path+".1" (replacing any existing ".1").
	MaxFileBytes int64
	// CorruptLines applies when reopening to recover the next sequence (scan only).
	CorruptLines CorruptLinePolicy
	// Durability selects write vs flush/fsync after each successful append.
	Durability Durability
	// Clock supplies timestamps when Event.Time is empty. Nil uses time.Now.
	Clock func() time.Time
	// FileMode is the mode for a newly created log file. Zero uses 0o644.
	FileMode uint32
	// CreateDir creates parent directories with 0o755 before opening the log.
	CreateDir bool
	// Codec marshals custom types for AppendAs (nil uses EventCodec).
	Codec Codec
	// Rotator archives segments when MaxFileBytes is set (nil uses SingleRotator → path+".1").
	Rotator Rotator
}

func (o Options) maxLineBytes() int {
	if o.MaxLineBytes > 0 {
		return o.MaxLineBytes
	}
	return DefaultMaxLineBytes
}

func (o Options) clock() func() time.Time {
	if o.Clock != nil {
		return o.Clock
	}
	return time.Now
}

func (o Options) fileMode() uint32 {
	if o.FileMode != 0 {
		return o.FileMode
	}
	return 0o644
}

// ReadOptions configures replay, poll, and scan helpers.
type ReadOptions struct {
	// MaxLineBytes caps line size while reading. Zero uses DefaultMaxLineBytes.
	MaxLineBytes int
	// CorruptLines selects skip vs error on invalid lines.
	CorruptLines CorruptLinePolicy
	// Rotator selects archive segments for MaxSeq (nil uses SingleRotator).
	Rotator Rotator
}

func (o ReadOptions) maxLineBytes() int {
	if o.MaxLineBytes > 0 {
		return o.MaxLineBytes
	}
	return DefaultMaxLineBytes
}
