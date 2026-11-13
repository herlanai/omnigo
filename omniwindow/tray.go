package omniwindow

import (
	"bytes"
	"fmt"
	"image/png"
	"slices"
	"strings"
)

type MenuItem struct {
	ID, Label, ToolTip                   string
	Disabled, Hidden, Checked, Separator bool
	Items                                []MenuItem
}
type TrayEvent struct {
	Kind               string
	ItemID             string
	Button, ClickCount int
}

// TrayOptions describes a status item snapshot. Menu selection and click
// callbacks run on the UI goroutine; schedule workers for blocking operations.
// With a menu, AppKit opens it directly; OnEvent then reports menu selections.
type TrayOptions struct {
	ID, Title, ToolTip string
	Icon               []byte // PNG; optional if Title is supplied
	Template           bool
	Menu               []MenuItem
	OnEvent            func(TrayEvent)
}
type TrayState struct {
	Bounds        ScreenRect
	Dark, Visible bool
}

func PrepareTray(o TrayOptions) (TrayOptions, error) {
	if o.ID == "" || strings.ContainsAny(o.ID, "\x00\r\n") {
		return o, fmt.Errorf("tray: stable ID required")
	}
	if o.Title == "" && len(o.Icon) == 0 {
		return o, fmt.Errorf("tray: title or PNG icon required")
	}
	if strings.ContainsRune(o.Title+o.ToolTip, 0) {
		return o, fmt.Errorf("tray: NUL in text")
	}
	if len(o.Icon) > 4<<20 {
		return o, fmt.Errorf("tray: icon exceeds 4 MiB")
	}
	if len(o.Icon) > 0 {
		config, err := png.DecodeConfig(bytes.NewReader(o.Icon))
		if err != nil {
			return o, fmt.Errorf("tray: invalid PNG: %w", err)
		}
		if config.Width > 4096 || config.Height > 4096 {
			return o, fmt.Errorf("tray: icon exceeds 4096 pixels")
		}
	}
	o.Icon = slices.Clone(o.Icon)
	count := 0
	ids := map[string]bool{}
	var copyItems func([]MenuItem, int) ([]MenuItem, error)
	copyItems = func(items []MenuItem, depth int) ([]MenuItem, error) {
		if depth > 8 {
			return nil, fmt.Errorf("tray: menu nesting exceeds 8")
		}
		result := slices.Clone(items)
		for i := range result {
			count++
			if count > 512 {
				return nil, fmt.Errorf("tray: too many menu items")
			}
			item := &result[i]
			if strings.ContainsRune(item.ID+item.Label+item.ToolTip, 0) {
				return nil, fmt.Errorf("tray: NUL in menu")
			}
			if item.Separator {
				if len(item.Items) > 0 {
					return nil, fmt.Errorf("tray: separator cannot own submenu")
				}
				continue
			}
			if item.ID == "" || ids[item.ID] {
				return nil, fmt.Errorf("tray: unique menu ID required")
			}
			ids[item.ID] = true
			if item.Label == "" {
				return nil, fmt.Errorf("tray: menu label required")
			}
			if len(item.Items) > 0 {
				children, err := copyItems(item.Items, depth+1)
				if err != nil {
					return nil, err
				}
				item.Items = children
			}
		}
		return result, nil
	}
	var err error
	o.Menu, err = copyItems(o.Menu, 0)
	return o, err
}
