// Command omni-update is an external macOS bundle-update supervisor. Install it
// outside the application being replaced. --wait-stdin waits for the old app's
// inherited pipe to close before touching any bundle files.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/herlanai/omnigo/omniui/plugins/updater"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	target := flag.String("target", "", "installed .app path")
	archive := flag.String("archive", "", "downloaded signed ZIP path")
	manifest := flag.String("manifest", "", "release JSON path")
	signature := flag.String("signature", "", "raw or Base64 Ed25519 signature path")
	public := flag.String("public-key", "", "trusted raw or omni keygen Base64 public key path")
	executable := flag.String("executable", "", "CFBundleExecutable filename")
	wait := flag.Bool("wait-stdin", false, "wait for inherited stdin pipe EOF after the old app exits")
	recover := flag.Bool("recover", false, "recover an abandoned bundle transaction instead of installing")
	rollback := flag.Bool("rollback", false, "restore the previous bundle without launching (requires --recover)")
	flag.Parse()
	if *target == "" || *public == "" || (!*rollback && *executable == "") {
		return errors.New("omni-update: target, public-key and executable are required")
	}
	if *rollback && !*recover {
		return errors.New("omni-update: rollback requires recover")
	}
	key, err := os.ReadFile(*public)
	if err != nil {
		return err
	}
	key, err = updater.ParsePublicKey(key)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *wait {
		done := make(chan error, 1)
		go func() { _, e := io.Copy(io.Discard, os.Stdin); done <- e }()
		select {
		case err = <-done:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var tx *updater.BundleInstallation
	if *recover {
		tx, err = updater.RecoverBundleInstallation(ctx, *target, key)
	} else {
		if *archive == "" || *manifest == "" || *signature == "" {
			return errors.New("omni-update: archive, manifest and signature are required")
		}
		m, e := os.ReadFile(*manifest)
		if e != nil {
			return e
		}
		s, e := os.ReadFile(*signature)
		if e != nil {
			return e
		}
		s, e = updater.ParseSignature(s)
		if e != nil {
			return e
		}
		tx, err = updater.InstallBundleVerified(ctx, *target, *archive, m, s, key)
	}
	if err != nil {
		return err
	}
	defer tx.Close()
	if *rollback {
		return tx.Rollback()
	}
	_, err = tx.RestartAndConfirm(ctx, *executable, flag.Args()...)
	return err
}
