// Command framecheck measures native AppKit submission, not screen presentation.
package main

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/host"
	"os"
	"runtime"
	"sort"
)

func init() { runtime.LockOSThread() }
func main() {
	samples, err := host.CheckFrameSubmission()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	fmt.Printf("%s %s/%s samples=%d 640x480 DIP 2x: submit p50=%s p95=%s p99=%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, len(samples), samples[49], samples[94], samples[98])
}
