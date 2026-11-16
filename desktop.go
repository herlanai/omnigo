//go:build !ios && !android

package omnigo

import (
	"context"

	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
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

type MenuItem = omniwindow.MenuItem
type TrayOptions = omniwindow.TrayOptions
type TrayEvent = omniwindow.TrayEvent
type TrayState = omniwindow.TrayState
type Tray = host.Tray

// NewTray creates a native status item after readiness. Call from a worker.
// The application owns its lifetime; Close removes it earlier. macOS-only.
func (a *Application) NewTray(ctx context.Context, o TrayOptions) (*Tray, error) {
	h := a.host.Load()
	if h == nil {
		return nil, ErrNotReady
	}
	return h.NewTray(ctx, o)
}

type MenuRole = omniwindow.MenuRole

const (
	MenuRoleCopy      = omniwindow.MenuRoleCopy
	MenuRoleCut       = omniwindow.MenuRoleCut
	MenuRolePaste     = omniwindow.MenuRolePaste
	MenuRoleSelectAll = omniwindow.MenuRoleSelectAll
	MenuRoleUndo      = omniwindow.MenuRoleUndo
	MenuRoleRedo      = omniwindow.MenuRoleRedo
)
