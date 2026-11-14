// Command idle samples an existing process without input injection or termination.
// ps CPU is an OS averaged indicator; RSS is KiB, not a leak measurement.
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: go run ./scripts/measure/idle PID [seconds]")
	}
	pid, e := strconv.Atoi(args[0])
	if e != nil || pid <= 0 {
		return fmt.Errorf("PID must be positive")
	}
	seconds := 60
	if len(args) == 2 {
		seconds, e = strconv.Atoi(args[1])
		if e != nil || seconds <= 0 {
			return fmt.Errorf("seconds must be positive")
		}
	}
	w := csv.NewWriter(os.Stdout)
	if e = w.Write([]string{"elapsed_s", "cpu_percent", "rss_kib"}); e != nil {
		return e
	}
	start := time.Now()
	for i := 0; i < seconds; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		b, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "%cpu=", "-o", "rss=").Output()
		cancel()
		if err != nil {
			return err
		}
		v := strings.Fields(string(b))
		if len(v) != 2 {
			return fmt.Errorf("process no longer available")
		}
		rss, err := strconv.ParseInt(v[1], 10, 64)
		if err != nil || rss <= 0 {
			return fmt.Errorf("invalid RSS")
		}
		if _, err = strconv.ParseFloat(v[0], 64); err != nil {
			return err
		}
		if e = w.Write([]string{fmt.Sprintf("%.3f", time.Since(start).Seconds()), v[0], v[1]}); e != nil {
			return e
		}
		w.Flush()
		if e = w.Error(); e != nil {
			return e
		}
		if i+1 < seconds {
			time.Sleep(time.Second)
		}
	}
	return nil
}
