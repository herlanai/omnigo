//go:build darwin && (amd64 || arm64)

package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniwindow"
	"image"
	"image/png"
	"time"
)

// CheckDesktopServices runs an opt-in native smoke probe on the process main
// thread. It opens and dismisses its own sheets, creates/removes its own status
// item, and posts synthetic OS notifications. It does not write selected files,
// change system appearance, or put the computer to sleep. No sleeps are used.
func CheckDesktopServices() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result := make(chan error, 1)
	err := Run(ctx, Options{InitialWindow: "desktop-probe", CreateWindow: func(string, bool, WindowServices) (ContentWindowOptions, error) {
		return ContentWindowOptions{Title: "OmniGo desktop service probe", Width: 480, Height: 240, View: func() *ui.Element { return ui.Text("probe", "Native desktop service verification") }}, nil
	}, OnReady: func(h *Controller) { go func() { defer cancel(); result <- checkDesktopWorker(ctx, h) }() }})
	if err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
	default:
		return fmt.Errorf("desktop probe did not finish: %w", ctx.Err())
	}
}
func checkDesktopWorker(ctx context.Context, h *Controller) error {
	// Cancel native sheets using AppKit's own cancellation, then exercise a real
	// alert completion response. A wrapper schedules actions after presentation.
	original := h.startDialog
	if err := h.callUI(ctx, func() error {
		h.startDialog = func(r dialogRequest, done func(dialogResult)) (func(), error) {
			cancel, err := original(r, done)
			if err != nil {
				return cancel, err
			}
			parent := byName["desktop-probe/false"].native
			h.control(func() {
				sheet := parent.Send(sel("attachedSheet"))
				if r.message != nil {
					parent.Send(sel("endSheet:returnCode:"), sheet, int64(1001))
				} else {
					sheet.Send(sel("cancel:"), objc.ID(0))
				}
			})
			return cancel, nil
		}
		return nil
	}); err != nil {
		return err
	}
	for _, save := range []bool{false, true} {
		o := omniwindow.FileDialogOptions{Parent: "desktop-probe", Title: "Automated native probe", Filters: []omniwindow.FileFilter{{Extensions: []string{"txt"}, MIMETypes: []string{"text/plain"}}}}
		var r omniwindow.FileDialogResult
		var err error
		if save {
			r, err = h.SaveDialog(ctx, o)
		} else {
			r, err = h.OpenDialog(ctx, o)
		}
		if err != nil {
			return fmt.Errorf("file dialog save=%v: %w", save, err)
		}
		if !r.Canceled || len(r.Paths) != 0 {
			return fmt.Errorf("file dialog cancel result: %+v", r)
		}
	}
	message, err := h.MessageDialog(ctx, omniwindow.MessageDialogOptions{Parent: "desktop-probe", Message: "Native alert probe", Buttons: []string{"Keep", "Cancel"}, CancelButton: 1, DestructiveButtons: []int{0}, CheckboxLabel: "Remember", CheckboxChecked: true})
	if err != nil {
		return err
	}
	if message.Response != 1 || !message.CheckboxChecked {
		return fmt.Errorf("alert result: %+v", message)
	}
	if err := h.callUI(ctx, func() error { h.startDialog = original; return nil }); err != nil {
		return err
	}

	var icon bytes.Buffer
	if err := png.Encode(&icon, image.NewRGBA(image.Rect(0, 0, 18, 18))); err != nil {
		return err
	}
	selected := make(chan string, 4)
	options := omniwindow.TrayOptions{ID: "omnigo.desktop-probe", Title: "Omni probe", Icon: icon.Bytes(), Template: true, Menu: []omniwindow.MenuItem{{ID: "enabled", Label: "Enabled", Checked: true}, {ID: "disabled", Label: "Disabled", Disabled: true}, {ID: "sub", Label: "Submenu", Items: []omniwindow.MenuItem{{ID: "child", Label: "Child"}}}}, OnEvent: func(e omniwindow.TrayEvent) { selected <- e.ItemID }}
	tray, err := h.NewTray(ctx, options)
	if err != nil {
		return err
	}
	state, err := tray.State(ctx)
	if err != nil {
		return err
	}
	if !state.Visible || state.Bounds.Width <= 0 {
		return fmt.Errorf("status item state: %+v", state)
	}
	if err := h.callUI(ctx, func() error {
		native := tray.driver.(*macTray)
		enabled := native.menu.Send(sel("itemAtIndex:"), int64(0))
		disabled := native.menu.Send(sel("itemAtIndex:"), int64(1))
		if enabled.Send(sel("state")) != 1 || disabled.Send(sel("isEnabled")) != 0 {
			return fmt.Errorf("native menu state mismatch")
		}
		native.target.Send(sel("invoke:"), disabled)
		native.target.Send(sel("invoke:"), enabled)
		return nil
	}); err != nil {
		return err
	}
	select {
	case id := <-selected:
		if id != "enabled" {
			return fmt.Errorf("disabled menu invoked: %s", id)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	options.Title = "Updated probe"
	options.Menu = []omniwindow.MenuItem{{ID: "replacement", Label: "Replacement"}}
	if err := tray.Update(ctx, options); err != nil {
		return err
	}
	if err := tray.Close(ctx); err != nil {
		return err
	}
	if _, err := tray.State(ctx); !errors.Is(err, ErrTrayClosed) {
		return fmt.Errorf("closed tray remained accessible: %v", err)
	}

	subscription, err := h.WatchSystemEvents(ctx)
	if err != nil {
		return err
	}
	defer subscription.Close()
	if _, err := h.SetAppearance(ctx, omniwindow.AppearanceDark); err != nil {
		return err
	}
	wait := func(kind omniwindow.SystemEventKind) (omniwindow.SystemEvent, error) {
		for {
			select {
			case event := <-subscription.Events:
				if event.Kind == kind {
					return event, event.Err
				}
			case <-ctx.Done():
				return omniwindow.SystemEvent{}, ctx.Err()
			}
		}
	}
	event, err := wait(omniwindow.SystemAppearanceChanged)
	if err != nil {
		return err
	}
	if event.Appearance == nil || !event.Appearance.Dark {
		return fmt.Errorf("effective appearance KVO missing state")
	}
	if _, err := h.SetAppearance(ctx, omniwindow.AppearanceSystem); err != nil {
		return err
	}
	if err := h.callUI(ctx, func() error {
		center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
		center.Send(sel("postNotificationName:object:"), stringID("NSApplicationDidChangeScreenParametersNotification"), objc.ID(0))
		workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(sel("sharedWorkspace")).Send(sel("notificationCenter"))
		workspace.Send(sel("postNotificationName:object:"), stringID("NSWorkspaceDidWakeNotification"), objc.ID(0))
		return nil
	}); err != nil {
		return err
	}
	event, err = wait(omniwindow.SystemDisplaysChanged)
	if err != nil {
		return err
	}
	if event.Screen == nil || len(event.Screen.Displays) == 0 {
		return fmt.Errorf("display notification missing snapshot")
	}
	if _, err := wait(omniwindow.SystemDidWake); err != nil {
		return err
	}
	subscription.Close()
	if err := h.callUI(ctx, func() error {
		if len(h.subscriptions) != 0 || len(trayTargets) != 0 {
			return fmt.Errorf("native resources remained registered")
		}
		return nil
	}); err != nil {
		return err
	}

	// Closing a different owned parent must finish its sheet without closing
	// the main probe window or retaining the native parent.
	if err := h.Open(ctx, "dialog-parent", false); err != nil {
		return err
	}
	if err := h.callUI(ctx, func() error {
		h.startDialog = func(r dialogRequest, done func(dialogResult)) (func(), error) {
			cancel, err := original(r, done)
			if err == nil {
				h.control(func() { byName["dialog-parent/false"].native.Send(sel("close")) })
			}
			return cancel, err
		}
		return nil
	}); err != nil {
		return err
	}
	closedParent, err := h.OpenDialog(ctx, omniwindow.FileDialogOptions{Parent: "dialog-parent"})
	if err != nil {
		return err
	}
	if !closedParent.Canceled {
		return fmt.Errorf("parent close did not cancel sheet")
	}
	if err := h.callUI(ctx, func() error {
		if byName["dialog-parent/false"] != nil || len(nativeDialogOwners) != 0 {
			return fmt.Errorf("closed dialog parent retained")
		}
		return nil
	}); err != nil {
		return err
	}

	// Context cancellation after native creation must release the sheet even if
	// the caller stops waiting before the completion block runs.
	dialogCtx, stop := context.WithCancel(ctx)
	defer stop()
	if err := h.callUI(ctx, func() error {
		h.startDialog = func(r dialogRequest, done func(dialogResult)) (func(), error) {
			cancel, err := original(r, done)
			stop()
			return cancel, err
		}
		return nil
	}); err != nil {
		return err
	}
	if _, err := h.OpenDialog(dialogCtx, omniwindow.FileDialogOptions{Parent: "desktop-probe"}); !errors.Is(err, context.Canceled) {
		return fmt.Errorf("context cancellation: %v", err)
	}
	return h.callUI(ctx, func() error {
		if h.dialog != nil {
			return fmt.Errorf("canceled native dialog still active")
		}
		return nil
	})
}
