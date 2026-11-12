package download

import (
	"context"
	"errors"
	"os"
)

// Validate checks artifact metadata without making requests or creating files.
func (f File) Validate() error { _, err := f.normalized(); return err }

// Stage returns a private verified file. The caller owns a successful result;
// cancellation and failures remove both the reservation and incomplete download.
func Stage(ctx context.Context, file File, directory string) (string, error) {
	if ctx == nil {
		return "", errors.New("download: nil context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f, err := file.normalized()
	if err != nil {
		return "", err
	}
	reserved, err := os.CreateTemp(directory, "omnigo-artifact-*")
	if err != nil {
		return "", err
	}
	path := reserved.Name()
	if err = reserved.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	if err = get(ctx, f, path); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}
