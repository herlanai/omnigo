package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
)

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
