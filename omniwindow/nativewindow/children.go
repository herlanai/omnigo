package nativewindow

import (
	"errors"

	"github.com/herlanai/omnigo/omniwindow"
)

type childBackend interface {
	child(omniwindow.WindowOptions) (backend, error)
}
type pendingBackend interface{ pending() bool }

// NewChild must run in Ready/Event or a Post callback on the parent's UI thread.
// Children share the root event loop and are destroyed before their parent.
// They must not call Run. Native titlebar closure respects their ShouldClose.
func (w *Window) NewChild(o Options) (*Window, error) {
	if w.closed || w.driver == nil {
		return nil, errors.New("nativewindow: parent is not running")
	}
	root := w.root
	if root == nil {
		root = w
	}
	if len(root.children) >= 128 {
		return nil, errors.New("nativewindow: child window limit exceeded")
	}
	factory, ok := w.driver.(childBackend)
	if !ok {
		return nil, errors.New("nativewindow: this driver does not implement owned child windows")
	}
	child, err := New(o)
	if err != nil {
		return nil, err
	}
	b, err := factory.child(child.options.Window)
	if err != nil {
		child.queue.Close()
		return nil, err
	}
	child.parent = w
	child.root = root
	child.driver = b
	child.used.Store(true)
	child.root.children = append(child.root.children, child)
	if child.options.Ready != nil {
		if err = child.options.Ready(child); err != nil {
			child.root.destroyChild(child)
			return nil, err
		}
	}
	b.wake()
	return child, nil
}
func descendant(child, parent *Window) bool {
	for p := child; p != nil; p = p.parent {
		if p == parent {
			return true
		}
	}
	return false
}
func (w *Window) destroyChild(target *Window) {
	for i := len(w.children) - 1; i >= 0; i-- {
		child := w.children[i]
		if !descendant(child, target) {
			continue
		}
		child.mu.Lock()
		b := child.driver
		child.driver = nil
		child.closed = true
		child.queue.Close()
		child.mu.Unlock()
		if b != nil {
			b.close()
		}
		w.children = append(w.children[:i], w.children[i+1:]...)
	}
}
func (w *Window) pumpChildren() error {
	// Callbacks can create children, so iterate a stable snapshot.
	for _, child := range append([]*Window(nil), w.children...) {
		if child.driver == nil {
			continue
		}
		child.drainFrame()
		if child.closeRequested {
			w.destroyChild(child)
			continue
		}
		events, err := child.driver.next(0)
		if err != nil {
			return err
		}
		for _, e := range events {
			closeWindow, err := child.handleEvent(e)
			if err != nil {
				return err
			}
			if closeWindow {
				w.destroyChild(child)
				break
			}
		}
		if child.driver != nil {
			if err := child.renderFrame(w.now()); err != nil {
				return err
			}
		}
	}
	for _, child := range w.children {
		if p, ok := child.driver.(pendingBackend); ok && p.pending() {
			w.driver.wake()
			break
		}
	}
	return nil
}
func (w *Window) childTimeout(timeout int) int {
	now := w.now()
	for _, child := range w.children {
		wait := child.frameTimeout(now)
		if wait >= 0 && (timeout < 0 || wait < timeout) {
			timeout = wait
		}
	}
	return timeout
}
