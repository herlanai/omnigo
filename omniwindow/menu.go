package omniwindow

import (
	"fmt"
	"github.com/herlanai/omnigo/omniwindow/accelerator"
	"runtime"
)

// MenuRole invokes the native responder chain. Unavailable responders leave the
// command unhandled; OnEvent still reports the menu selection.
type MenuRole string

const (
	MenuRoleCopy      MenuRole = "copy"
	MenuRoleCut       MenuRole = "cut"
	MenuRolePaste     MenuRole = "paste"
	MenuRoleSelectAll MenuRole = "selectAll"
	MenuRoleUndo      MenuRole = "undo"
	MenuRoleRedo      MenuRole = "redo"
)

func validateMenuRole(role MenuRole) error {
	switch role {
	case "", MenuRoleCopy, MenuRoleCut, MenuRolePaste, MenuRoleSelectAll, MenuRoleUndo, MenuRoleRedo:
		return nil
	default:
		return fmt.Errorf("menu: unknown role %q", role)
	}
}

// MenuKeyEquivalent resolves a portable chord for the native menu backend.
func MenuKeyEquivalent(chord string) (accelerator.Accelerator, error) {
	return accelerator.Parse(chord, runtime.GOOS)
}
