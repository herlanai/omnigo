package system

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func nativeTrash(ctx context.Context, path string) error {
	name, err := exec.LookPath("gio")
	if err != nil {
		return fmt.Errorf("%w: gio trash is required", ErrUnavailable)
	}
	output, err := exec.CommandContext(ctx, name, "trash", "--", path).CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("trash: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
