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

// The shell owns the PIDL returned by ILCreateFromPathW; release it on every path.
func nativeRevealWindows(target string) error {
	path, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := initializeCOM.Call(0, 2)
	if uint32(hr) == 0 || uint32(hr) == 1 {
		defer uninitializeCOM.Call()
	} else if uint32(hr) != 0x80010106 {
		return fmt.Errorf("COM initialization failed: 0x%x", uint32(hr))
	}
	shell := windows.NewLazySystemDLL("shell32.dll")
	pidl, _, _ := shell.NewProc("ILCreateFromPathW").Call(uintptr(unsafe.Pointer(path)))
	runtime.KeepAlive(path)
	if pidl == 0 {
		return fmt.Errorf("reveal: cannot resolve shell item")
	}
	defer shell.NewProc("ILFree").Call(pidl)
	// With cidl zero the absolute PIDL identifies the item to select.
	result, _, _ := shell.NewProc("SHOpenFolderAndSelectItems").Call(pidl, 0, 0, 0)
	if int32(result) < 0 {
		return fmt.Errorf("SHOpenFolderAndSelectItems failed: 0x%x", uint32(result))
	}
	return nil
}
