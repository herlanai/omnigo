//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
	"os"
	"path/filepath"
	"sync"
)

var nativeDialogOwners = map[objc.ID]func(){}

var uniformTypes struct {
	once sync.Once
	err  error
}

func dialogParent(app objc.ID, id string, dark bool) (objc.ID, error) {
	if id != "" {
		if w := byName[fmt.Sprintf("%s/%t", id, dark)]; w != nil {
			return w.native, nil
		}
		return 0, fmt.Errorf("dialog: unknown parent window %q", id)
	}
	parent := app.Send(sel("keyWindow"))
	if parent == 0 {
		parent = app.Send(sel("mainWindow"))
	}
	if parent == 0 {
		return 0, fmt.Errorf("dialog: no live parent window")
	}
	return parent, nil
}

func nativeDialog(app objc.ID, r dialogRequest, done func(dialogResult)) (func(), error) {
	parentID := r.file.Parent
	parentDark := r.file.ParentDark
	if r.message != nil {
		parentID = r.message.Parent
		parentDark = r.message.ParentDark
	}
	parent, err := dialogParent(app, parentID, parentDark)
	if err != nil {
		return nil, err
	}
	if parent.Send(sel("attachedSheet")) != 0 {
		return nil, ErrDialogBusy
	}
	if r.message != nil {
		return nativeMessage(parent, *r.message, done)
	}
	class, method := "NSOpenPanel", "openPanel"
	if r.save {
		class, method = "NSSavePanel", "savePanel"
	}
	panel := objc.ID(objc.GetClass(class)).Send(sel(method)).Send(sel("retain"))
	o := r.file
	for _, v := range []struct{ key, value string }{{"setTitle:", o.Title}, {"setPrompt:", o.ButtonLabel}, {"setMessage:", o.Message}, {"setNameFieldLabel:", o.NameFieldLabel}} {
		if v.value != "" {
			panel.Send(sel(v.key), stringID(v.value))
		}
	}
	panel.Send(sel("setShowsHiddenFiles:"), o.ShowHiddenFiles)
	panel.Send(sel("setCanCreateDirectories:"), o.CreateDirectories)
	panel.Send(sel("setTreatsFilePackagesAsDirectories:"), o.TreatPackagesAsDirectories)
	if !r.save {
		panel.Send(sel("setCanChooseFiles:"), !o.Directory)
		panel.Send(sel("setCanChooseDirectories:"), o.Directory)
		panel.Send(sel("setAllowsMultipleSelection:"), o.Multiple)
	}
	if o.DefaultPath != "" {
		directory := o.DefaultPath
		if info, e := os.Stat(directory); e != nil || !info.IsDir() {
			directory = filepath.Dir(o.DefaultPath)
			panel.Send(sel("setNameFieldStringValue:"), stringID(filepath.Base(o.DefaultPath)))
		}
		url := objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), stringID(directory))
		panel.Send(sel("setDirectoryURL:"), url)
	}
	if err := setPanelTypes(panel, o.Filters); err != nil {
		panel.Send(sel("release"))
		return nil, err
	}
	parent.Send(sel("retain"))
	finished := false
	finish := func(result dialogResult) {
		if finished {
			return
		}
		finished = true
		delete(nativeDialogOwners, parent)
		panel.Send(sel("orderOut:"), objc.ID(0))
		parent.Send(sel("release"))
		panel.Send(sel("release"))
		done(result)
	}
	block := objc.NewBlock(func(_ objc.Block, response int64) {
		if finished {
			return
		}
		if response != 0 && response != 1 {
			finish(dialogResult{err: fmt.Errorf("dialog: native presentation failed (%d)", response)})
			return
		}
		result := omniwindow.FileDialogResult{Canceled: response != 1}
		if response == 1 {
			if r.save {
				url := panel.Send(sel("URL"))
				if url != 0 {
					result.Paths = []string{objc.Send[string](url.Send(sel("path")), sel("UTF8String"))}
				}
			} else {
				urls := panel.Send(sel("URLs"))
				for i, n := uint64(0), uint64(urls.Send(sel("count"))); i < n; i++ {
					url := urls.Send(sel("objectAtIndex:"), i)
					result.Paths = append(result.Paths, objc.Send[string](url.Send(sel("path")), sel("UTF8String")))
				}
			}
			if len(result.Paths) == 0 {
				finish(dialogResult{err: fmt.Errorf("dialog: accepted without a file URL")})
				return
			}
		}
		finish(dialogResult{file: result})
	})
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), parent, block)
	block.Release() // AppKit copies the block for the lifetime of the sheet.
	cancel := func() {
		if finished {
			return
		}
		panel.Send(sel("retain"))
		defer panel.Send(sel("release"))
		parent.Send(sel("endSheet:returnCode:"), panel, int64(0))
		panel.Send(sel("orderOut:"), objc.ID(0))
		finish(dialogResult{file: omniwindow.FileDialogResult{Canceled: true}})
	}
	if !finished {
		nativeDialogOwners[parent] = cancel
	}
	return cancel, nil
}

func setPanelTypes(panel objc.ID, filters []omniwindow.FileFilter) error {
	if len(filters) == 0 {
		return nil
	}
	uniformTypes.once.Do(func() {
		_, uniformTypes.err = purego.Dlopen("/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	if uniformTypes.err != nil {
		return uniformTypes.err
	}
	types := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	cls := objc.ID(objc.GetClass("UTType"))
	all := false
	for _, f := range filters {
		for _, ext := range f.Extensions {
			if ext == "*" {
				all = true
				continue
			}
			t := cls.Send(sel("typeWithFilenameExtension:"), stringID(ext))
			if t == 0 {
				return fmt.Errorf("dialog: unsupported extension %q", ext)
			}
			types.Send(sel("addObject:"), t)
		}
		for _, mime := range f.MIMETypes {
			if mime == "*/*" {
				all = true
				continue
			}
			var t objc.ID
			if id := map[string]string{"image/*": "public.image", "audio/*": "public.audio", "video/*": "public.movie", "text/*": "public.text", "font/*": "public.font"}[mime]; id != "" {
				t = cls.Send(sel("typeWithIdentifier:"), stringID(id))
			} else {
				t = cls.Send(sel("typeWithMIMEType:"), stringID(mime))
			}
			if t == 0 {
				return fmt.Errorf("dialog: unsupported MIME type %q", mime)
			}
			types.Send(sel("addObject:"), t)
		}
	}
	if all {
		types = objc.ID(objc.GetClass("NSArray")).Send(sel("array"))
	}
	panel.Send(sel("setAllowedContentTypes:"), types)
	return nil
}

func nativeMessage(parent objc.ID, o omniwindow.MessageDialogOptions, done func(dialogResult)) (func(), error) {
	alert := objc.ID(objc.GetClass("NSAlert")).Send(sel("new"))
	style := int64(1)
	if o.Kind == "warning" {
		style = 0
	}
	if o.Kind == "error" {
		style = 2
	}
	alert.Send(sel("setAlertStyle:"), style)
	text := o.Message
	if text == "" {
		text = o.Title
	}
	alert.Send(sel("setMessageText:"), stringID(text))
	alert.Send(sel("setInformativeText:"), stringID(o.Detail))
	window := alert.Send(sel("window"))
	if o.Title != "" {
		window.Send(sel("setTitle:"), stringID(o.Title))
	}
	for i, label := range o.Buttons {
		button := alert.Send(sel("addButtonWithTitle:"), stringID(label))
		key := ""
		if i == o.CancelButton {
			key = "\x1b"
		}
		if i == o.DefaultButton {
			key = "\r"
		}
		button.Send(sel("setKeyEquivalent:"), stringID(key))
		for _, index := range o.DestructiveButtons {
			if index == i {
				if button.Send(sel("respondsToSelector:"), sel("setHasDestructiveAction:")) == 0 {
					alert.Send(sel("release"))
					return nil, fmt.Errorf("dialog: destructive buttons require macOS 11 or newer")
				}
				button.Send(sel("setHasDestructiveAction:"), true)
			}
		}
	}
	if o.CheckboxLabel != "" {
		alert.Send(sel("setShowsSuppressionButton:"), true)
		button := alert.Send(sel("suppressionButton"))
		button.Send(sel("setTitle:"), stringID(o.CheckboxLabel))
		state := int64(0)
		if o.CheckboxChecked {
			state = 1
		}
		button.Send(sel("setState:"), state)
	}
	parent.Send(sel("retain"))
	finished := false
	finish := func(response int) {
		if finished {
			return
		}
		finished = true
		delete(nativeDialogOwners, parent)
		parent.Send(sel("release"))
		checked := o.CheckboxChecked
		if o.CheckboxLabel != "" {
			checked = alert.Send(sel("suppressionButton")).Send(sel("state")) == 1
		}
		alert.Send(sel("release"))
		done(dialogResult{message: omniwindow.MessageDialogResult{Response: response, CheckboxChecked: checked}})
	}
	block := objc.NewBlock(func(_ objc.Block, response int64) {
		if finished {
			return
		}
		index := int(response) - 1000
		if index < 0 || index >= len(o.Buttons) {
			index = o.CancelButton
		}
		finish(index)
	})
	alert.Send(sel("beginSheetModalForWindow:completionHandler:"), parent, block)
	block.Release()
	cancel := func() {
		if finished {
			return
		}
		window.Send(sel("retain"))
		defer window.Send(sel("release"))
		parent.Send(sel("endSheet:returnCode:"), window, int64(1000+o.CancelButton))
		window.Send(sel("orderOut:"), objc.ID(0))
		finish(o.CancelButton)
	}
	if !finished {
		nativeDialogOwners[parent] = cancel
	}
	return cancel, nil
}
