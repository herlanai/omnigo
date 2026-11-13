//go:build darwin && (amd64 || arm64)

package host

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// All delegate and panel state belongs to the AppKit main thread.
var uploadDelegateClass struct {
	once  sync.Once
	class objc.Class
	err   error
}
var copyUploadBlock func(unsafe.Pointer) unsafe.Pointer
var releaseUploadBlock func(unsafe.Pointer)

var webUploadCancels = map[objc.ID]func(){}

func attachUploadDelegate(web objc.ID) (func(), error) {
	uploadDelegateClass.once.Do(func() {
		copyAddress, err := purego.Dlsym(purego.RTLD_DEFAULT, "_Block_copy")
		if err != nil {
			uploadDelegateClass.err = err
			return
		}
		releaseAddress, err := purego.Dlsym(purego.RTLD_DEFAULT, "_Block_release")
		if err != nil {
			uploadDelegateClass.err = err
			return
		}
		purego.RegisterFunc(&copyUploadBlock, copyAddress)
		purego.RegisterFunc(&releaseUploadBlock, releaseAddress)
		uploadDelegateClass.class, uploadDelegateClass.err = objc.RegisterClass("OmniGoWebUploadDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("webView:runOpenPanelWithParameters:initiatedByFrame:completionHandler:"), Fn: func(_ objc.ID, _ objc.SEL, view, parameters, _ objc.ID, completion unsafe.Pointer) {
				showWebUpload(view, parameters, completion)
			}},
		})
	})
	if uploadDelegateClass.err != nil {
		return nil, uploadDelegateClass.err
	}
	delegate := objc.ID(uploadDelegateClass.class).Send(sel("new"))
	web.Send(sel("setUIDelegate:"), delegate)
	return func() {
		web.Send(sel("setUIDelegate:"), objc.ID(0))
		if cancel := webUploadCancels[web]; cancel != nil {
			cancel()
		}
		delegate.Send(sel("release"))
	}, nil
}

func showWebUpload(web, parameters objc.ID, completion unsafe.Pointer) {
	// WebKit owns this native block. Copy it for the asynchronous sheet, and invoke
	// its ABI entry point directly (objc.Block.Invoke only handles Go-created blocks).
	handler := copyUploadBlock(completion)
	finishHandler := func(urls objc.ID) {
		header := (*struct {
			Isa             uintptr
			Flags, Reserved uint32
			Invoke          uintptr
		})(handler)
		var resolve func(unsafe.Pointer, objc.ID)
		purego.RegisterFunc(&resolve, header.Invoke)
		resolve(handler, urls)
		releaseUploadBlock(handler)
	}
	parent := web.Send(sel("window"))
	if parent == 0 || parent.Send(sel("attachedSheet")) != 0 || webUploadCancels[web] != nil {
		finishHandler(0)
		return
	}
	panel := objc.ID(objc.GetClass("NSOpenPanel")).Send(sel("openPanel")).Send(sel("retain"))
	panel.Send(sel("setCanChooseFiles:"), true)
	panel.Send(sel("setAllowsMultipleSelection:"), parameters.Send(sel("allowsMultipleSelection")) != 0)
	directories := parameters.Send(sel("respondsToSelector:"), sel("allowsDirectories")) != 0 && parameters.Send(sel("allowsDirectories")) != 0
	panel.Send(sel("setCanChooseDirectories:"), directories)
	parent.Send(sel("retain"))
	finished := false
	finish := func(response int64) {
		if finished {
			return
		}
		finished = true
		delete(webUploadCancels, web)
		delete(nativeDialogOwners, parent)
		panel.Send(sel("orderOut:"), objc.ID(0))
		var urls objc.ID
		if response == 1 {
			urls = panel.Send(sel("URLs"))
		}
		finishHandler(urls)
		parent.Send(sel("release"))
		panel.Send(sel("release"))
	}
	cancel := func() {
		if finished {
			return
		}
		panel.Send(sel("retain"))
		defer panel.Send(sel("release"))
		parent.Send(sel("endSheet:returnCode:"), panel, int64(0))
		finish(0)
	}
	webUploadCancels[web] = cancel
	nativeDialogOwners[parent] = cancel
	block := objc.NewBlock(func(_ objc.Block, response int64) { finish(response) })
	panel.Send(sel("beginSheetModalForWindow:completionHandler:"), parent, block)
	block.Release()
}
