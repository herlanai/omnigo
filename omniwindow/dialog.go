package omniwindow

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// FileFilter combines extensions (with or without a dot) and MIME types.
// macOS combines filters into one allowed-type list. Names are descriptive;
// they do not create a filter-selection dropdown.
type FileFilter struct {
	Name       string
	Extensions []string
	MIMETypes  []string
}

// FileDialogOptions configures an owned native open/save sheet. Parent is a Go
// content window ID; empty chooses the key/main window. Directory and Multiple
// apply only to OpenDialog. The default open selection is a single file.
type FileDialogOptions struct {
	ParentDark                                                                          bool // selects the dark variant of a named Go content window
	Parent, Title, DefaultPath, ButtonLabel, Message, NameFieldLabel                    string
	Filters                                                                             []FileFilter
	Directory, Multiple, ShowHiddenFiles, CreateDirectories, TreatPackagesAsDirectories bool
}

type FileDialogResult struct {
	Paths    []string `json:"paths"`
	Canceled bool     `json:"canceled"`
}

// MessageDialogOptions configures a native alert. Button indices are zero-based;
// defaults select the first button. Empty Buttons becomes a single OK button.
// CancelButton is returned for native dismissal; it gets the Escape shortcut
// when distinct from DefaultButton. Context cancellation
// instead returns a context error. Callbacks are not embedded in menu/dialog DTOs.
type MessageDialogOptions struct {
	ParentDark                     bool
	Parent, Title, Message, Detail string
	Kind                           string // empty, info, warning, error, question
	Buttons                        []string
	DefaultButton, CancelButton    int
	DestructiveButtons             []int
	CheckboxLabel                  string
	CheckboxChecked                bool
}

type MessageDialogResult struct {
	Response        int  `json:"response"`
	CheckboxChecked bool `json:"checkboxChecked"`
}

// PrepareFileDialog validates and snapshots caller-owned collections.
func PrepareFileDialog(o FileDialogOptions, save bool) (FileDialogOptions, error) {
	for _, s := range []string{o.Parent, o.Title, o.DefaultPath, o.ButtonLabel, o.Message, o.NameFieldLabel} {
		if strings.ContainsRune(s, 0) {
			return o, fmt.Errorf("dialog: NUL in option")
		}
	}
	if save && (o.Directory || o.Multiple) {
		return o, fmt.Errorf("dialog: save cannot select directories or multiple files")
	}
	if o.DefaultPath != "" {
		p, err := filepath.Abs(o.DefaultPath)
		if err != nil {
			return o, err
		}
		o.DefaultPath = p
	}
	if len(o.Filters) > 128 {
		return o, fmt.Errorf("dialog: too many filters")
	}
	o.Filters = slices.Clone(o.Filters)
	for i := range o.Filters {
		f := &o.Filters[i]
		f.Extensions = slices.Clone(f.Extensions)
		f.MIMETypes = slices.Clone(f.MIMETypes)
		if len(f.Extensions)+len(f.MIMETypes) > 128 {
			return o, fmt.Errorf("dialog: too many types")
		}
		for j, e := range f.Extensions {
			e = strings.TrimPrefix(e, ".")
			if e == "" || strings.ContainsAny(e, "/\\\x00\r\n") || strings.Contains(e, "*") && e != "*" {
				return o, fmt.Errorf("dialog: invalid extension %q", e)
			}
			f.Extensions[j] = e
		}
		for _, m := range f.MIMETypes {
			parts := strings.Split(m, "/")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(m, "\x00\r\n \t") {
				return o, fmt.Errorf("dialog: invalid MIME type %q", m)
			}
		}
	}
	return o, nil
}

func PrepareMessageDialog(o MessageDialogOptions) (MessageDialogOptions, error) {
	switch o.Kind {
	case "", "info", "warning", "error", "question":
	default:
		return o, fmt.Errorf("dialog: invalid message kind")
	}
	if len(o.Buttons) == 0 {
		o.Buttons = []string{"OK"}
	} else {
		o.Buttons = slices.Clone(o.Buttons)
	}
	if len(o.Buttons) > 32 {
		return o, fmt.Errorf("dialog: too many buttons")
	}
	for _, s := range append([]string{o.Parent, o.Title, o.Message, o.Detail, o.CheckboxLabel}, o.Buttons...) {
		if strings.ContainsRune(s, 0) {
			return o, fmt.Errorf("dialog: NUL in option")
		}
	}
	for _, b := range o.Buttons {
		if b == "" {
			return o, fmt.Errorf("dialog: empty button")
		}
	}
	o.DestructiveButtons = slices.Clone(o.DestructiveButtons)
	for _, i := range append([]int{o.DefaultButton, o.CancelButton}, o.DestructiveButtons...) {
		if i < 0 || i >= len(o.Buttons) {
			return o, fmt.Errorf("dialog: button index out of bounds")
		}
	}
	return o, nil
}
