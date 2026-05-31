package jsonlreplay

import "os"

//
// ────────────────────────────────────────
// rotation.
//

// Rotator archives the active log segment and reports paths to scan for seq recovery.
type Rotator interface {
	MaybeRotate(path string, maxBytes int64, lineLen int) (rotated bool, err error)
	ScanPaths(path string) []string
}

// SingleRotator rotates to path+".1" (default). Use via Options.Rotator / ReadOptions.Rotator.
type SingleRotator struct{}

// MaybeRotate moves path to path+".1" when size+lineLen would exceed maxBytes.
func (SingleRotator) MaybeRotate(path string, maxBytes int64, lineLen int) (bool, error) {
	return rotateIfOverCap(path, maxBytes, lineLen)
}

// ScanPaths returns the active path and path+".1" when the archive exists.
func (SingleRotator) ScanPaths(path string) []string {
	archived := path + ".1"
	if _, err := os.Stat(archived); err == nil {
		return []string{path, archived}
	}
	return []string{path}
}

func (o Options) rotator() Rotator {
	if o.Rotator != nil {
		return o.Rotator
	}
	return SingleRotator{}
}

func (o ReadOptions) rotator() Rotator {
	if o.Rotator != nil {
		return o.Rotator
	}
	return SingleRotator{}
}
