package omniweb

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Assets serves only a built client filesystem. HTML navigation falls back to
// index.html; missing scripts, RPC routes and traversal never receive the shell.
// Pass os.DirFS("ui/dist/client") or an embedded fs.Sub, never the source tree.
func Assets(files fs.FS) (http.Handler, error) {
	if files == nil {
		return nil, errors.New("omniweb: client filesystem required")
	}
	if info, err := fs.Stat(files, "index.html"); err != nil || info.IsDir() {
		return nil, errors.New("omniweb: build the SPA client first (index.html missing)")
	}
	server := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/omniweb/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			http.NotFound(w, r)
			return
		}
		for _, part := range strings.Split(name, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		info, err := fs.Stat(files, name)
		if err == nil && !info.IsDir() {
			server.ServeHTTP(w, r)
			return
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		if path.Ext(name) != "" || !strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.NotFound(w, r)
			return
		}
		// FileServer serves index.html for '/' without redirecting client routes.
		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		server.ServeHTTP(w, clone)
	}), nil
}
