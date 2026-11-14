//go:build !ios && !android

package mobile

import "errors"

// Run requires an iOS or Android application bundle; desktop APIs remain separate.
func Run(Options) error { return errors.New("mobile: build for ios or android with omni mobile build") }
