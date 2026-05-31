package jsonlreplay

// Test hooks for coverage of unexported helpers.
var (
	RotateIfOverCapForTest = rotateIfOverCap
	ScanFileForTest        = scanFile
	PollReplayForTest      = pollReplay
)
