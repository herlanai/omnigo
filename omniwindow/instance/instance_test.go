package instance

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestOwnershipForwardingRelease(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	owner, err := Open(t.Context(), root, "app")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if second, err := Open(t.Context(), root, "app"); !errors.Is(err, ErrRunning) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("%v", err)
	}
	want := Launch{[]string{"a", "中文", "'$(echo nope)'"}, root, "activation"}
	if err := Notify(t.Context(), root, "app", want); err != nil {
		t.Fatal(err)
	}
	if got := <-owner.Messages(); !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	owner.Close()
	owner.Close()
	next, err := Open(t.Context(), root, "app")
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}
func TestQueueBoundAndCancellation(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	owner, err := Open(t.Context(), root, "app")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	for i := 0; i < 32; i++ {
		if err := Notify(t.Context(), root, "app", Launch{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := Notify(t.Context(), root, "app", Launch{}); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Notify(ctx, root, "app", Launch{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Open(ctx, root, "another"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestProcessLockSurvivesStaleEndpoint(t *testing.T) {
	if os.Getenv("OMNIGO_INSTANCE_CHILD") == "1" {
		owner, err := Open(context.Background(), os.Getenv("OMNIGO_INSTANCE_ROOT"), "process")
		if err != nil {
			fmt.Fprintln(os.Stdout, err)
			os.Exit(2)
		}
		_ = owner
		fmt.Fprintln(os.Stdout, "ready")
		io.Copy(io.Discard, os.Stdin)
		os.Exit(0) // Intentionally no Close: simulate process exit without cleanup.
	}
	root := t.TempDir()
	os.Chmod(root, 0700)
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessLockSurvivesStaleEndpoint$")
	cmd.Env = append(os.Environ(), "OMNIGO_INSTANCE_CHILD=1", "OMNIGO_INSTANCE_ROOT="+root)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	ready, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("%q %v", ready, err)
	}
	if l, err := Open(t.Context(), root, "process"); !errors.Is(err, ErrRunning) {
		if l != nil {
			l.Close()
		}
		t.Fatal(err)
	}
	if err := Notify(t.Context(), root, "process", Launch{Args: []string{"child-message"}}); err != nil {
		t.Fatal(err)
	}
	input.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	l, err := Open(t.Context(), root, "process")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

func TestNotifyNotReadyBeforePayload(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	if err := Notify(t.Context(), root, "absent", Launch{}); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}

func TestRejectedPayloadNeverEnqueued(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	owner, err := Open(t.Context(), root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	valid, _ := json.Marshal(request{owner.endpoint.Token, Launch{Args: []string{"unexpected"}}})
	wrong, _ := json.Marshal(request{"wrong-token", Launch{}})
	for _, raw := range [][]byte{append(wrong, '\n'), append(append(append([]byte(nil), valid...), []byte(" {}")...), '\n'), []byte("{bad}\n")} {
		c, err := net.Dial("tcp4", owner.endpoint.Address)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Write(raw); err != nil {
			c.Close()
			t.Fatal(err)
		}
		var result string
		err = json.NewDecoder(c).Decode(&result)
		c.Close()
		if err != nil || result != "invalid" {
			t.Fatalf("%q %v", result, err)
		}
		select {
		case got := <-owner.Messages():
			t.Fatalf("unexpected message: %+v", got)
		default:
		}
	}
}
