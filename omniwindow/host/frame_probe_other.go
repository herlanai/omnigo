//go:build !darwin || (!amd64 && !arm64)

package host

import "time"

func CheckFrameSubmission() ([]time.Duration, error) { return nil, ErrUnsupported }
