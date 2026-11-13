//go:build darwin && (amd64 || arm64)

package system

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var foundationOnce sync.Once
var foundationError error

func nativeTrash(ctx context.Context, path string) error {
	_, err := trashMac(ctx, path)
	return err
}

// trashMac returns the OS-selected destination for restoration/verification.
func trashMac(ctx context.Context, path string) (string, error) {
	foundationOnce.Do(func() {
		_, foundationError = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	})
	if foundationError != nil {
		return "", foundationError
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	sel := objc.RegisterName
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	str := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), path)
	url := objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), str)
	manager := objc.ID(objc.GetClass("NSFileManager")).Send(sel("defaultManager"))
	var nativeErr, destination objc.ID
	if manager.Send(sel("trashItemAtURL:resultingItemURL:error:"), url, unsafe.Pointer(&destination), unsafe.Pointer(&nativeErr)) != 0 {
		if destination == 0 {
			return "", fmt.Errorf("trash: missing destination after successful move")
		}
		result := destination.Send(sel("path"))
		return objc.Send[string](result, sel("UTF8String")), nil
	}
	if nativeErr != 0 {
		description := nativeErr.Send(sel("localizedDescription"))
		message := objc.Send[string](description, sel("UTF8String"))
		return "", fmt.Errorf("trash: %s", message)
	}
	return "", fmt.Errorf("trash: NSFileManager rejected item")
}
