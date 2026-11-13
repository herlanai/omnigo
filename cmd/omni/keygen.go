package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const keygenLock = ".omni-keygen.lock"

func keygen(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	directory := flags.String("o", "", "key output directory (default: user config/omnigo/update-keys)")
	force := flags.Bool("force", false, "replace the existing update key pair")
	if err := parseFlags(flags, args, output); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("keygen takes no positional arguments")
	}
	if *directory == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		*directory = filepath.Join(config, "omnigo", "update-keys")
	}
	if err := writeUpdateKeys(*directory, *force); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "Created %s and %s\nKeep the .key file secret; distribute only the .pub file with the application.\n", filepath.Join(*directory, "omnigo-update.key"), filepath.Join(*directory, "omnigo-update.pub"))
	return err
}

// The directory lock serializes cooperating keygen processes. Backups stay in
// the private lock directory until both replacements are installed. This is a
// recoverable two-file update, not a crash-atomic filesystem transaction.
func writeUpdateKeys(directory string, force bool) error {
	return writeUpdateKeysWith(directory, force, installUpdateKey)
}

func writeUpdateKeysWith(directory string, force bool, install func(*os.Root, string, []byte, fs.FileMode) (bool, error)) (result error) {
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.Mkdir(keygenLock, 0700); err != nil {
		return fmt.Errorf("cannot acquire keygen lock: %w", err)
	}
	keepRecovery := false
	defer func() {
		if !keepRecovery {
			result = errors.Join(result, root.RemoveAll(keygenLock))
		}
	}()
	names := []string{"omnigo-update.key", "omnigo-update.pub"}
	exists := make([]bool, len(names))
	for i, name := range names {
		info, err := root.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular key destination: %s", name)
		}
		if !force {
			return errors.New("update keys already exist; use -force to replace the pair")
		}
		exists[i] = true
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	values := [][]byte{
		[]byte(base64.StdEncoding.EncodeToString(private) + "\n"),
		[]byte(base64.StdEncoding.EncodeToString(public) + "\n"),
	}
	backedUp := make([]bool, len(names))
	installed := make([]bool, len(names))
	rollback := func(cause error) error {
		for i, name := range names {
			if installed[i] {
				if err := root.Remove(name); err != nil {
					keepRecovery = true
					cause = errors.Join(cause, err)
					continue
				}
			}
			if backedUp[i] {
				if _, err := root.Lstat(name); !errors.Is(err, fs.ErrNotExist) {
					keepRecovery = true
					cause = errors.Join(cause, fmt.Errorf("destination changed; recover %s from %s", name, filepath.Join(directory, keygenLock)))
					continue
				}
				if err := root.Rename(filepath.Join(keygenLock, name), name); err != nil {
					keepRecovery = true
					cause = errors.Join(cause, fmt.Errorf("restore %s from %s: %w", name, filepath.Join(directory, keygenLock), err))
				}
			}
		}
		return cause
	}
	for i, name := range names {
		if exists[i] {
			if err = root.Rename(name, filepath.Join(keygenLock, name)); err != nil {
				return rollback(err)
			}
			backedUp[i] = true
		}
	}
	for i, name := range names {
		mode := fs.FileMode(0600)
		if i == 1 {
			mode = 0644
		}
		installed[i], err = install(root, name, values[i], mode)
		if err != nil {
			return rollback(err)
		}
	}
	return nil
}

// Exclusive creation never follows a destination link or clobbers a file that
// appeared since preflight. The bool tells rollback whether this call owns it.
func installUpdateKey(root *os.Root, name string, data []byte, mode fs.FileMode) (bool, error) {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return false, err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	return true, errors.Join(writeErr, syncErr, closeErr)
}
