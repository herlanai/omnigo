//go:build darwin || linux

package terminal

import (
	"context"
	"errors"
	"github.com/creack/pty"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type process struct {
	file *os.File
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	once sync.Once
}

func start(ctx context.Context, o Options) (backend, error) {
	cmd := exec.Command(o.Command, o.Args...)
	cmd.Dir = o.Dir
	cmd.Env = append(os.Environ(), o.Env...)
	file, e := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(o.Columns), Rows: uint16(o.Rows)})
	if e != nil {
		return nil, e
	}
	p := &process{file: file, cmd: cmd, done: make(chan struct{})}
	stop := context.AfterFunc(ctx, func() { p.Close() })
	go func() { p.err = cmd.Wait(); close(p.done); stop() }()
	return p, nil
}
func (p *process) Read(b []byte) (int, error) {
	n, e := p.file.Read(b)
	if errors.Is(e, syscall.EIO) {
		e = io.EOF
	}
	return n, e
}
func (p *process) Write(b []byte) (int, error) { return p.file.Write(b) }
func (p *process) Resize(columns, rows int) error {
	if !validSize(columns, rows) {
		return errors.New("terminal: invalid size")
	}
	return pty.Setsize(p.file, &pty.Winsize{Cols: uint16(columns), Rows: uint16(rows)})
}
func (p *process) Wait() error { <-p.done; return p.err }
func (p *process) Close() error {
	var err error
	p.once.Do(func() {
		err = p.file.Close()
		select {
		case <-p.done:
		default:
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return err
}
