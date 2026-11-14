//go:build windows && (amd64 || arm64) && omnigo_webview2

package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow/nativewindow"
	"golang.org/x/sys/windows"
)

const Available = true

// This small ABI adapter uses the published, stable WebView2 base interfaces.
// The application must deploy Microsoft's matching WebView2Loader.dll beside its
// executable and install the Evergreen Runtime. No executable search-path loading.
var ole = windows.NewLazySystemDLL("ole32.dll")
var comObjects sync.Map

type comHandler struct {
	table  *[4]uintptr
	refs   atomic.Int32
	pin    runtime.Pinner
	invoke func(uintptr, uintptr) uintptr
}

func newCOMHandler(iid string, fn func(uintptr, uintptr) uintptr) *comHandler {
	own, _ := windows.GUIDFromString(iid)
	unknown, _ := windows.GUIDFromString("{00000000-0000-0000-C000-000000000046}")
	h := &comHandler{invoke: fn}
	h.refs.Store(1)
	h.table = &[4]uintptr{
		syscall.NewCallback(func(this, iid, out uintptr) uintptr {
			if out == 0 {
				return 0x80004003
			}
			*(*uintptr)(unsafe.Pointer(out)) = 0
			if iid == 0 {
				return 0x80004003
			}
			requested := *(*windows.GUID)(unsafe.Pointer(iid))
			if requested != own && requested != unknown {
				return 0x80004002
			}
			*(*uintptr)(unsafe.Pointer(out)) = this
			h.refs.Add(1)
			return 0
		}),
		syscall.NewCallback(func(uintptr) uintptr { return uintptr(h.refs.Add(1)) }),
		syscall.NewCallback(func(uintptr) uintptr { return h.release() }),
		syscall.NewCallback(func(_, a, b uintptr) uintptr { return h.invoke(a, b) }),
	}
	h.pin.Pin(h)
	h.pin.Pin(h.table)
	comObjects.Store(h, h)
	return h
}
func (h *comHandler) release() uintptr {
	n := h.refs.Add(-1)
	if n == 0 {
		comObjects.Delete(h)
		h.pin.Unpin()
	}
	return uintptr(n)
}

//go:uintptrescapes
func comCall(object uintptr, slot int, args ...uintptr) error {
	if object == 0 {
		return errors.New("host: null COM object")
	}
	table := *(*uintptr)(unsafe.Pointer(object))
	address := *(*uintptr)(unsafe.Pointer(table + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	hr, _, _ := syscall.SyscallN(address, append([]uintptr{object}, args...)...)
	if int32(hr) < 0 {
		return fmt.Errorf("host: WebView2 HRESULT 0x%08x", uint32(hr))
	}
	return nil
}
func comString(object uintptr, slot int) (string, error) {
	var ptr *uint16
	if err := comCall(object, slot, uintptr(unsafe.Pointer(&ptr))); err != nil {
		return "", err
	}
	if ptr == nil {
		return "", nil
	}
	defer ole.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(ptr)))
	return windows.UTF16PtrToString(ptr), nil
}
func comText(object uintptr, slot int, text string) error {
	p, e := windows.UTF16PtrFromString(text)
	if e != nil {
		return e
	}
	e = comCall(object, slot, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	return e
}

func runNative(parent context.Context, o Options) error {
	if o.Window.URL == "" || o.InitialWindow != "" || o.CreateWindow != nil {
		return errors.New("host: WebView2 supports a Web main window only")
	}
	if !started.CompareAndSwap(false, true) {
		return errors.New("host: Run is single-use per process")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dll, err := windows.LoadDLL(filepath.Join(filepath.Dir(exe), "WebView2Loader.dll"))
	if err != nil {
		return fmt.Errorf("host: deploy matching Microsoft WebView2Loader.dll beside executable: %w", err)
	}
	// Keep the loader mapped: asynchronous COM callbacks may outlive window teardown.
	create, err := dll.FindProc("CreateCoreWebView2EnvironmentWithOptions")
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	profile := filepath.Join(cache, "OmniGo", filepath.Base(exe), "WebView2")
	if err = os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	origin, _ := url.Parse(o.Window.URL)
	trusted := func(value string) bool {
		u, e := url.Parse(value)
		return e == nil && u.User == nil && u.Scheme == origin.Scheme && u.Host == origin.Host
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	var controller, web uintptr
	var hostWindow *nativewindow.Window
	var failure error
	var initialized, alive bool
	var handlers []*comHandler
	pending := map[string]context.CancelFunc{}
	h := &Controller{queue: dispatch.New(128), ctx: ctx}
	fail := func(e error) {
		if e != nil && failure == nil {
			failure = e
			cancel()
		}
	}
	handler := func(iid string, fn func(uintptr, uintptr) uintptr) *comHandler {
		v := newCOMHandler(iid, func(a, b uintptr) uintptr {
			if !alive {
				return 0
			}
			return fn(a, b)
		})
		handlers = append(handlers, v)
		return v
	}
	subscribe := func(slot int, fn func(uintptr, uintptr) uintptr) error {
		ids := map[int]string{7: "{9ADBE429-F36D-432B-9DDC-F8881FBD76E3}", 44: "{D4C185FE-C81C-4989-97AF-2D3FA7AB5651}", 23: "{15E1C6A3-C72A-4DF3-91D7-D097FBEC6BFD}", 25: "{79E0AEA4-990B-42D9-AA1D-0FCC2E5BC7F1}", 34: "{57213F19-00E6-49FA-8E07-898EA01ECBD2}"}
		v := handler(ids[slot], fn)
		var token int64
		return comCall(web, slot, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&token)))
	}
	resize := func() error {
		if controller == 0 {
			return nil
		}
		var rect [4]int32
		r, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("GetClientRect").Call(hostWindow.HWND(), uintptr(unsafe.Pointer(&rect)))
		if r == 0 {
			return errors.New("host: GetClientRect failed")
		}
		return comCall(controller, 6, uintptr(unsafe.Pointer(&rect)))
	}
	options := o.Window
	options.URL = ""
	window, err := nativewindow.New(nativewindow.Options{Window: options,
		Event: func(e nativewindow.Event) error {
			if e.Kind == "resize" {
				return resize()
			}
			return nil
		},
		Closing: func() {
			alive = false
			for _, stop := range pending {
				stop()
			}
			h.close()
			if controller != 0 {
				comCall(controller, 24)
				comCall(controller, 2)
			}
			if web != 0 {
				comCall(web, 2)
			}
			for _, v := range handlers {
				v.release()
			}
			if initialized {
				ole.NewProc("CoUninitialize").Call()
			}
		},
		Ready: func(w *nativewindow.Window) error {
			hostWindow = w
			alive = true
			hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
			if int32(hr) < 0 {
				return fmt.Errorf("host: COM STA initialization failed: %08x", hr)
			}
			initialized = true
			h.wake = func() { _, _ = w.Post(ctx, func() error { h.drain(); return nil }) }
			envHandler := handler("{4E8A3389-C9D8-4BD2-B6B5-124FEE6CC14D}", func(result, environment uintptr) uintptr {
				if int32(result) < 0 || environment == 0 {
					fail(fmt.Errorf("host: WebView2 environment creation failed: %08x", result))
					return 0
				}
				readyHandler := handler("{6C4819F3-C9B7-4260-8127-C9F5BDE7F68C}", func(result, value uintptr) uintptr {
					if int32(result) < 0 || value == 0 {
						fail(fmt.Errorf("host: WebView2 controller creation failed: %08x", result))
						return 0
					}
					controller = value
					comCall(controller, 1)
					if e := comCall(controller, 25, uintptr(unsafe.Pointer(&web))); e != nil {
						fail(e)
						return 0
					}
					fail(resize())
					fail(comCall(controller, 4, 1))
					fail(subscribe(7, func(_, args uintptr) uintptr {
						uri, e := comString(args, 3)
						if e != nil || !trusted(uri) {
							comCall(args, 8, 1)
						}
						return 0
					}))
					fail(subscribe(44, func(_, args uintptr) uintptr { comCall(args, 6, 1); return 0 }))
					fail(subscribe(23, func(_, args uintptr) uintptr { comCall(args, 7, 2); return 0 }))
					fail(subscribe(25, func(_, args uintptr) uintptr { fail(errors.New("host: WebView2 browser process failed")); return 0 }))
					if o.NativeExchange != nil {
						fail(subscribe(34, func(_, args uintptr) uintptr {
							source, e := comString(args, 3)
							if e != nil || !trusted(source) {
								return 0
							}
							message, e := comString(args, 5)
							if e != nil || len(message) > 1<<20 {
								return 0
							}
							var request struct {
								Kind, Token string
								Request     json.RawMessage
							}
							if json.Unmarshal([]byte(message), &request) != nil || len(request.Token) == 0 || len(request.Token) > 128 {
								return 0
							}
							if request.Kind == "cancel" {
								if stop := pending[request.Token]; stop != nil {
									stop()
								}
								return 0
							}
							if request.Kind != "call" || pending[request.Token] != nil || len(pending) >= 64 {
								return 0
							}
							callCtx, stop := context.WithTimeout(ctx, 30*time.Second)
							pending[request.Token] = stop
							go func() {
								result, e := o.NativeExchange(callCtx, request.Request)
								stop()
								_, _ = h.Post(ctx, func() error {
									delete(pending, request.Token)
									reply := map[string]any{"token": request.Token}
									if e != nil {
										reply["error"] = e.Error()
									} else {
										reply["result"] = string(result)
									}
									raw, _ := json.Marshal(reply)
									return comText(web, 32, string(raw))
								})
							}()
							return 0
						}))
					}
					fail(comText(web, 5, o.Window.URL))
					if failure == nil && o.OnReady != nil {
						o.OnReady(h)
					}
					return 0
				})
				fail(comCall(environment, 3, w.HWND(), uintptr(unsafe.Pointer(readyHandler))))
				return 0
			})
			p, e := windows.UTF16PtrFromString(profile)
			if e != nil {
				return e
			}
			hr, _, _ = create.Call(0, uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(envHandler)))
			runtime.KeepAlive(p)
			if int32(hr) < 0 {
				return fmt.Errorf("host: WebView2 Runtime initialization failed: %08x", hr)
			}
			return nil
		}})
	if err != nil {
		return err
	}
	err = window.Run(ctx)
	if failure != nil {
		return failure
	}
	if errors.Is(err, context.Canceled) && parent.Err() != nil {
		return nil
	}
	return err
}
