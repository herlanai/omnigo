package system

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"syscall"
	"unsafe"
)

type shellCOM struct{ table *[23]uintptr }

func (o *shellCOM) call(index int, args ...uintptr) error {
	values := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	hr, _, _ := syscall.SyscallN(o.table[index], values...)
	runtime.KeepAlive(o)
	if int32(hr) < 0 {
		return fmt.Errorf("shell COM method %d: HRESULT 0x%x", index, uint32(hr))
	}
	return nil
}
func nativeTrash(ctx context.Context, path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := initializeCOM.Call(0, 2)
	// Do not run this STA-only operation inside a conflicting apartment.
	if uint32(hr) != 0 && uint32(hr) != 1 {
		return fmt.Errorf("trash COM initialization: 0x%x", uint32(hr))
	}
	defer uninitializeCOM.Call()
	clsid, _ := windows.GUIDFromString("{3AD05575-8857-4850-9277-11B85BDB8E09}")
	iid, _ := windows.GUIDFromString("{947AAB5F-0A5C-4C13-B4D6-4BF7836FC9F8}")
	var operation *shellCOM
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&operation)))
	if int32(hr) < 0 {
		return fmt.Errorf("trash CoCreateInstance: 0x%x", uint32(hr))
	}
	if operation == nil {
		return fmt.Errorf("trash: missing operation")
	}
	defer operation.call(2)
	// Windows 8+: recycle explicitly, preserve undo, fail instead of skipping errors.
	const flags = 0x00080000 | 0x20000000 | 0x00100000 | 0x0400 | 0x0004 | 0x4000 | 0x2000
	if err := operation.call(5, flags); err != nil {
		return err
	}
	itemIID, _ := windows.GUIDFromString("{43826D1E-E718-42EE-BC55-A1E261C37BFE}")
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item *shellCOM
	hr, _, _ = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(wide)), 0, uintptr(unsafe.Pointer(&itemIID)), uintptr(unsafe.Pointer(&item)))
	runtime.KeepAlive(wide)
	if int32(hr) < 0 {
		return fmt.Errorf("trash shell item: 0x%x", uint32(hr))
	}
	if item == nil {
		return fmt.Errorf("trash: missing shell item")
	}
	defer item.call(2)
	if err := operation.call(18, uintptr(unsafe.Pointer(item)), 0); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := operation.call(21); err != nil {
		return err
	}
	var aborted int32
	if err := operation.call(22, uintptr(unsafe.Pointer(&aborted))); err != nil {
		return err
	}
	if aborted != 0 {
		return fmt.Errorf("trash: operation aborted")
	}
	return nil
}
