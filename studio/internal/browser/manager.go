// Package browser owns Studio's optional CEF process, separate from baseui demos.
package browser

import (
	"context"
	"errors"
	"omnigo.dev/omnigo/omniui/plugins/chromium"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Manager struct {
	mu      sync.Mutex
	ctx     context.Context
	browser *chromium.Browser
	closed  bool
}

func New(ctx context.Context) *Manager { return &Manager{ctx: ctx} }
func HostPath() (string, error) {
	if p := os.Getenv("OMNIGO_CHROMIUM_HOST"); p != "" {
		return filepath.Abs(p)
	}
	if exe, e := os.Executable(); e == nil {
		p := filepath.Join(filepath.Dir(exe), "../Resources/chromium/Omni Browser.app/Contents/MacOS/Omni Browser")
		if _, e = os.Stat(p); e == nil {
			return filepath.Clean(p), nil
		}
	}
	// Development checkout fallback; distributed Studio uses the bundled runtime.
	p := "build/chromium-runtime/Omni Browser.app/Contents/MacOS/Omni Browser"
	if _, e := os.Stat(p); e == nil {
		return filepath.Abs(p)
	}
	return chromium.DefaultHostPath()
}
func (m *Manager) Open(ctx context.Context, dark bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("browser manager closed")
	}
	if m.browser != nil {
		select {
		case <-m.browser.Done():
			m.browser = nil
		default:
			return m.browser.SelectTab(ctx, m.browser.State().BrowserID)
		}
	}
	path, e := HostPath()
	if e != nil {
		return e
	}
	if _, e = os.Stat(path); e != nil {
		return errors.New("CEF runtime missing: run pnpm setup:chromium, then rebuild Studio")
	}
	// Studio uses the normal secure persistent profile; ephemeral/mock-keychain
	// profiles are restricted to explicitly opted-in development E2E tests.
	opts := chromium.Options{HostPath: path, AppName: "Omni Browser"}
	if dark {
		opts.Theme = &chromium.Theme{Dark: true, Background: "18181bff", Surface: "27272aff", Muted: "3f3f46ff", Foreground: "fafafaff", MutedForeground: "a1a1aaff", Border: "3f3f46ff", Primary: "e4e4e7ff", Radius: 10, FontSize: 13}
	}
	m.browser, e = chromium.Launch(m.ctx, opts)
	return e
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.browser != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = m.browser.Close(ctx)
		m.browser = nil
	}
}
