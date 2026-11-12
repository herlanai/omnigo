// Package terminal owns real pseudo-terminal processes. The current backend
// supports macOS/Linux. VT rendering and Windows ConPTY remain separate work.
package terminal

import (
	"context"
	"errors"
	"io"
)

var ErrUnsupported = errors.New("terminal: PTY backend is not available on this platform")

type Options struct {
	Command       string
	Args, Env     []string
	Dir           string
	Columns, Rows int
}
type backend interface {
	io.ReadWriteCloser
	Resize(int, int) error
	Wait() error
}
type Session struct{ backend }

func Start(ctx context.Context, o Options) (*Session, error) {
	if ctx == nil {
		return nil, errors.New("terminal: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.Command == "" {
		return nil, errors.New("terminal: explicit command required")
	}
	if o.Columns == 0 {
		o.Columns = 80
	}
	if o.Rows == 0 {
		o.Rows = 24
	}
	if !validSize(o.Columns, o.Rows) {
		return nil, errors.New("terminal: invalid size")
	}
	b, e := start(ctx, o)
	if e != nil {
		return nil, e
	}
	return &Session{b}, nil
}
func validSize(columns, rows int) bool {
	return columns > 0 && rows > 0 && columns <= 4096 && rows <= 4096
}
