//go:build windows

package system

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shellExecute = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")
var ole = windows.NewLazySystemDLL("ole32.dll")
var initializeCOM = ole.NewProc("CoInitializeEx")
var uninitializeCOM = ole.NewProc("CoUninitialize")

func nativeOpenWindows(target string) error {
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Shell handlers may use COM. Balance only successful initialization calls.
	hr, _, _ := initializeCOM.Call(0, 2)
	if uint32(hr) == 0 || uint32(hr) == 1 {
		defer uninitializeCOM.Call()
	} else if uint32(hr) != 0x80010106 {
		return fmt.Errorf("COM initialization failed: 0x%x", uint32(hr))
	}
	result, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	runtime.KeepAlive(file)
	runtime.KeepAlive(verb)
	if result <= 32 {
		return fmt.Errorf("ShellExecuteW failed: %d", result)
	}
	return nil
}
