package jsonlreplay

import (
	"fmt"
	"os"
)

//
// ────────────────────────────────────────
// rotation.
//

// rotateIfOverCap moves a non-empty path to path+".1" when size+lineLen would exceed cap.
// Returns true when rotation occurred.
func rotateIfOverCap(path string, maxBytes int64, lineLen int) (bool, error) {
	if maxBytes <= 0 {
		return false, nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if fi.Size() == 0 {
		return false, nil
	}
	if fi.Size()+int64(lineLen) <= maxBytes {
		return false, nil
	}
	archived := path + ".1"
	_ = os.Remove(archived)
	if err := os.Rename(path, archived); err != nil {
		return false, fmt.Errorf("jsonlreplay: rotate %s: %w", path, err)
	}
	return true, nil
}
