package download

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheProcessHelper(t *testing.T) {
	if os.Getenv("OMNIGO_DOWNLOAD_TEST_CHILD") != "1" {
		return
	}
	file := File{URL: os.Getenv("OMNIGO_DOWNLOAD_TEST_URL"), SHA256: digest("process")}
	file.wait = func(ctx context.Context, _ time.Duration) error {
		fmt.Println("waiting-for-cache-lock")
		var gate [1]byte
		_, err := io.ReadFull(os.Stdin, gate[:])
		return err
	}
	if err := Cached(t.Context(), file, os.Getenv("OMNIGO_DOWNLOAD_TEST_PATH")); err != nil {
		t.Fatal(err)
	}
}
func TestCacheSerializesProcesses(t *testing.T) {
	var requests atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(entered)
			<-release
		}
		io.WriteString(w, "process")
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "artifact")
	done := make(chan error, 1)
	go func() { done <- Cached(t.Context(), File{URL: server.URL, SHA256: digest("process")}, path) }()
	<-entered
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCacheProcessHelper$")
	command.Env = append(os.Environ(), "OMNIGO_DOWNLOAD_TEST_CHILD=1", "OMNIGO_DOWNLOAD_TEST_URL="+server.URL, "OMNIGO_DOWNLOAD_TEST_PATH="+path)
	output, err := command.StdoutPipe()
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	input, err := command.StdinPipe()
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err = command.Start(); err != nil {
		close(release)
		t.Fatal(err)
	}
	reader := bufio.NewReader(output)
	line, err := reader.ReadString('\n')
	close(release)
	if err != nil || line != "waiting-for-cache-lock\n" {
		input.Close()
		command.Wait()
		t.Fatal("child did not observe lock", line, err)
	}
	if err = <-done; err != nil {
		input.Close()
		command.Wait()
		t.Fatal(err)
	}
	if _, err = input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	input.Close()
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Wait(); err != nil {
		t.Fatal(err, string(rest))
	}
	if requests.Load() != 1 || !Check(path, digest("process")) {
		t.Fatal("cache downloaded twice")
	}
}
