//go:build darwin && (amd64 || arm64)

package host

import (
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
	"sync"
	"unsafe"
)

var systemObservers = struct {
	sync.Mutex
	callbacks map[objc.ID]func(omniwindow.SystemEventKind)
}{callbacks: make(map[objc.ID]func(omniwindow.SystemEventKind))}
var systemObserverClass struct {
	once  sync.Once
	class objc.Class
	err   error
}
var workspaceEvents = map[string]omniwindow.SystemEventKind{
	"NSWorkspaceWillSleepNotification":                            omniwindow.SystemWillSleep,
	"NSWorkspaceDidWakeNotification":                              omniwindow.SystemDidWake,
	"NSWorkspaceScreensDidSleepNotification":                      omniwindow.SystemScreensDidSleep,
	"NSWorkspaceScreensDidWakeNotification":                       omniwindow.SystemScreensDidWake,
	"NSWorkspaceSessionDidBecomeActiveNotification":               omniwindow.SystemSessionActive,
	"NSWorkspaceSessionDidResignActiveNotification":               omniwindow.SystemSessionInactive,
	"NSWorkspaceAccessibilityDisplayOptionsDidChangeNotification": omniwindow.SystemAppearanceChanged,
}

func deliverSystemNotification(observer objc.ID, kind omniwindow.SystemEventKind) {
	systemObservers.Lock()
	callback := systemObservers.callbacks[observer]
	systemObservers.Unlock()
	if callback != nil {
		callback(kind)
	}
}
func watchNativeSystem(app objc.ID, notify func(omniwindow.SystemEventKind)) (func(), error) {
	systemObserverClass.once.Do(func() {
		systemObserverClass.class, systemObserverClass.err = objc.RegisterClass("OmniGoSystemObserver", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("notification:"), Fn: func(observer objc.ID, _ objc.SEL, note objc.ID) {
				name := objc.Send[string](note.Send(sel("name")), sel("UTF8String"))
				kind := workspaceEvents[name]
				if name == "NSApplicationDidChangeScreenParametersNotification" {
					kind = omniwindow.SystemDisplaysChanged
				}
				if name == "NSSystemColorsDidChangeNotification" {
					kind = omniwindow.SystemAppearanceChanged
				}
				if kind != "" {
					deliverSystemNotification(observer, kind)
				}
			}},
			{Cmd: sel("observeValueForKeyPath:ofObject:change:context:"), Fn: func(observer objc.ID, _ objc.SEL, _ objc.ID, _ objc.ID, _ objc.ID, _ unsafe.Pointer) {
				deliverSystemNotification(observer, omniwindow.SystemAppearanceChanged)
			}},
		})
	})
	if systemObserverClass.err != nil {
		return nil, systemObserverClass.err
	}
	observer := objc.ID(systemObserverClass.class).Send(sel("new"))
	systemObservers.Lock()
	systemObservers.callbacks[observer] = notify
	systemObservers.Unlock()
	center := objc.ID(objc.GetClass("NSNotificationCenter")).Send(sel("defaultCenter"))
	workspace := objc.ID(objc.GetClass("NSWorkspace")).Send(sel("sharedWorkspace")).Send(sel("notificationCenter"))
	for _, name := range []string{"NSApplicationDidChangeScreenParametersNotification", "NSSystemColorsDidChangeNotification"} {
		center.Send(sel("addObserver:selector:name:object:"), observer, sel("notification:"), stringID(name), objc.ID(0))
	}
	for name := range workspaceEvents {
		workspace.Send(sel("addObserver:selector:name:object:"), observer, sel("notification:"), stringID(name), objc.ID(0))
	}
	app.Send(sel("addObserver:forKeyPath:options:context:"), observer, stringID("effectiveAppearance"), uint64(0), unsafe.Pointer(nil))
	return func() {
		systemObservers.Lock()
		delete(systemObservers.callbacks, observer)
		systemObservers.Unlock()
		app.Send(sel("removeObserver:forKeyPath:"), observer, stringID("effectiveAppearance"))
		center.Send(sel("removeObserver:"), observer)
		workspace.Send(sel("removeObserver:"), observer)
		observer.Send(sel("release"))
	}, nil
}
