//go:build !ios && !android

package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
)

type FileFilter = omniwindow.FileFilter
type FileDialogOptions = omniwindow.FileDialogOptions
type FileDialogResult = omniwindow.FileDialogResult
type MessageDialogOptions = omniwindow.MessageDialogOptions
type MessageDialogResult = omniwindow.MessageDialogResult

// OpenDialog shows a native file/directory sheet. Call from a worker after
// readiness, never synchronously from a UI callback. Context cancellation closes
// the sheet; user cancellation returns Canceled:true with no error. macOS-only.
func (a *Application) OpenDialog(ctx context.Context, o FileDialogOptions) (FileDialogResult, error) {
	h := a.host.Load()
	if h == nil {
		return FileDialogResult{}, ErrNotReady
	}
	return h.OpenDialog(ctx, o)
}

// SaveDialog asks for a destination without creating or writing a file.
// An accepted result contains exactly one path. Same threading as OpenDialog.
func (a *Application) SaveDialog(ctx context.Context, o FileDialogOptions) (FileDialogResult, error) {
	h := a.host.Load()
	if h == nil {
		return FileDialogResult{}, ErrNotReady
	}
	return h.SaveDialog(ctx, o)
}

// MessageDialog shows an owned native alert. Same threading as OpenDialog.
func (a *Application) MessageDialog(ctx context.Context, o MessageDialogOptions) (MessageDialogResult, error) {
	h := a.host.Load()
	if h == nil {
		return MessageDialogResult{}, ErrNotReady
	}
	return h.MessageDialog(ctx, o)
}
