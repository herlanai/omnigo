//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
	"sync"
	"unsafe"
)

var trayTargets = map[objc.ID]*macTray{}
var trayClass struct {
	once  sync.Once
	class objc.Class
	err   error
}

type macTray struct {
	item, target, menu objc.ID
	options            omniwindow.TrayOptions
	actions            map[int64]string
	closed             bool
}

func newNativeTray(o omniwindow.TrayOptions) (trayDriver, error) {
	trayClass.once.Do(func() {
		trayClass.class, trayClass.err = objc.RegisterClass("OmniGoStatusItemActions", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("invoke:"), Fn: func(target objc.ID, _ objc.SEL, sender objc.ID) {
			guard(func() {
				t := trayTargets[target]
				if t == nil || t.closed || t.options.OnEvent == nil {
					return
				}
				event := omniwindow.TrayEvent{Kind: "menu"}
				if sender == t.item.Send(sel("button")) {
					event.Kind = "click"
					native := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("currentEvent"))
					event.Button = int(native.Send(sel("buttonNumber")))
					event.ClickCount = int(native.Send(sel("clickCount")))
				} else {
					id, ok := t.actions[int64(sender.Send(sel("tag")))]
					if !ok {
						return
					}
					event.ItemID = id
				}
				t.options.OnEvent(event)
			})
		}}})
	})
	if trayClass.err != nil {
		return nil, trayClass.err
	}
	bar := objc.ID(objc.GetClass("NSStatusBar")).Send(sel("systemStatusBar"))
	item := bar.Send(sel("statusItemWithLength:"), float64(-1))
	if item == 0 {
		return nil, fmt.Errorf("tray: status bar unavailable")
	}
	item.Send(sel("retain"))
	t := &macTray{item: item, target: objc.ID(trayClass.class).Send(sel("new"))}
	trayTargets[t.target] = t
	item.Send(sel("setAutosaveName:"), stringID(o.ID))
	button := item.Send(sel("button"))
	button.Send(sel("setTarget:"), t.target)
	button.Send(sel("setAction:"), sel("invoke:"))
	button.Send(sel("sendActionOn:"), uint64((1<<2)|(1<<4)))
	if err := t.update(o); err != nil {
		t.close()
		return nil, err
	}
	return t, nil
}
func (t *macTray) update(o omniwindow.TrayOptions) error {
	var icon objc.ID
	if len(o.Icon) > 0 {
		data := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&o.Icon[0]), uint64(len(o.Icon)))
		icon = objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), data)
		if icon == 0 {
			return fmt.Errorf("tray: native image decode failed")
		}
		defer icon.Send(sel("release"))
		icon.Send(sel("setTemplate:"), o.Template)
		icon.Send(sel("setSize:"), size{W: 18, H: 18})
	}
	actions := map[int64]string{}
	tag := int64(0)
	var build func([]omniwindow.MenuItem, bool) objc.ID
	build = func(items []omniwindow.MenuItem, parentEnabled bool) objc.ID {
		menu := objc.ID(objc.GetClass("NSMenu")).Send(sel("alloc")).Send(sel("initWithTitle:"), stringID(""))
		menu.Send(sel("setAutoenablesItems:"), false)
		for _, entry := range items {
			if entry.Hidden {
				continue
			}
			if entry.Separator {
				menu.Send(sel("addItem:"), objc.ID(objc.GetClass("NSMenuItem")).Send(sel("separatorItem")))
				continue
			}
			item := objc.ID(objc.GetClass("NSMenuItem")).Send(sel("alloc")).Send(sel("initWithTitle:action:keyEquivalent:"), stringID(entry.Label), sel("invoke:"), stringID(""))
			tag++
			item.Send(sel("setTag:"), tag)
			item.Send(sel("setTarget:"), t.target)
			enabled := parentEnabled && !entry.Disabled
			item.Send(sel("setEnabled:"), enabled)
			state := int64(0)
			if entry.Checked {
				state = 1
			}
			item.Send(sel("setState:"), state)
			if entry.ToolTip != "" {
				item.Send(sel("setToolTip:"), stringID(entry.ToolTip))
			}
			if len(entry.Items) > 0 {
				submenu := build(entry.Items, enabled)
				item.Send(sel("setSubmenu:"), submenu)
				submenu.Send(sel("release"))
			} else if enabled {
				actions[tag] = entry.ID
			}
			menu.Send(sel("addItem:"), item)
			item.Send(sel("release"))
		}
		return menu
	}
	var menu objc.ID
	if len(o.Menu) > 0 {
		menu = build(o.Menu, true)
	}
	if t.menu != 0 {
		t.menu.Send(sel("cancelTracking"))
	}
	t.item.Send(sel("setMenu:"), menu)
	if t.menu != 0 {
		clearMenuTargets(t.menu)
		t.menu.Send(sel("release"))
	}
	t.menu = menu
	t.actions = actions
	t.options = o
	button := t.item.Send(sel("button"))
	button.Send(sel("setTitle:"), stringID(o.Title))
	button.Send(sel("setToolTip:"), stringID(o.ToolTip))
	button.Send(sel("setImage:"), icon)
	return nil
}
func clearMenuTargets(menu objc.ID) {
	for i, n := int64(0), int64(menu.Send(sel("numberOfItems"))); i < n; i++ {
		item := menu.Send(sel("itemAtIndex:"), i)
		item.Send(sel("setTarget:"), objc.ID(0))
		if sub := item.Send(sel("submenu")); sub != 0 {
			clearMenuTargets(sub)
		}
	}
}
func (t *macTray) state() (omniwindow.TrayState, error) {
	button := t.item.Send(sel("button"))
	window := button.Send(sel("window"))
	bounds := objc.Send[rect](button, sel("bounds"))
	bounds = objc.Send[rect](button, sel("convertRect:toView:"), bounds, objc.ID(0))
	bounds = objc.Send[rect](window, sel("convertRectToScreen:"), bounds)
	screens := objc.ID(objc.GetClass("NSScreen")).Send(sel("screens"))
	if screens.Send(sel("count")) == 0 {
		return omniwindow.TrayState{}, fmt.Errorf("tray: no displays")
	}
	primary := objc.Send[rect](screens.Send(sel("objectAtIndex:"), uint64(0)), sel("frame"))
	return omniwindow.TrayState{Bounds: screenRect(bounds.P.X, bounds.P.Y, bounds.S.W, bounds.S.H, primary.P.Y+primary.S.H), Dark: appearanceDark(button.Send(sel("effectiveAppearance"))), Visible: t.item.Send(sel("isVisible")) != 0}, nil
}
func (t *macTray) dismiss() {
	if t.menu != 0 {
		t.menu.Send(sel("cancelTracking"))
	}
}

func (t *macTray) popup() error {
	if t.menu == 0 {
		return fmt.Errorf("tray: no menu")
	}
	item := t.item.Send(sel("retain"))
	defer item.Send(sel("release"))
	menu := t.menu.Send(sel("retain"))
	defer menu.Send(sel("release"))
	// AppKit owns menu tracking; keep native objects alive across its nested loop.
	target := t.target.Send(sel("retain"))
	defer target.Send(sel("release"))
	item.Send(sel("popUpStatusItemMenu:"), menu)
	return nil
}
func (t *macTray) close() {
	if t.closed {
		return
	}
	t.closed = true
	delete(trayTargets, t.target)
	if t.menu != 0 {
		t.menu.Send(sel("cancelTracking"))
		clearMenuTargets(t.menu)
	}
	button := t.item.Send(sel("button"))
	button.Send(sel("setTarget:"), objc.ID(0))
	t.item.Send(sel("setMenu:"), objc.ID(0))
	objc.ID(objc.GetClass("NSStatusBar")).Send(sel("systemStatusBar")).Send(sel("removeStatusItem:"), t.item)
	if t.menu != 0 {
		t.menu.Send(sel("release"))
	}
	t.item.Send(sel("release"))
	t.target.Send(sel("release"))
}
