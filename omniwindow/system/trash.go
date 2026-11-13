package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Trash moves one item to the system trash, preserving OS restore metadata.
// It never falls back to permanent deletion. A symlink is trashed, not its target.
// Cancellation cannot undo an operation already accepted by the operating system.
func Trash(ctx context.Context, path string) error {
	if ctx == nil {
		return fmt.Errorf("trash: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	absolute, err := trashPath(path)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nativeTrash(ctx, absolute)
}
func trashPath(path string) (string, error) {
	if path == "" || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("trash: invalid path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if filepath.Dir(absolute) == absolute {
		return "", fmt.Errorf("trash: filesystem root is not an item")
	}
	if _, err := os.Lstat(absolute); err != nil {
		return "", err
	}
	return absolute, nil
}
