package omniweb

import "context"

// RequestSource is an opaque server-owned transport host identity. A native window
// with its own HTTP handler has its own source. Browser tabs sharing a handler
// share a source; this is not a browser-document or authenticated-user identity.
type RequestSource struct{ marker byte }
type sourceKey struct{}

// Source returns the host identity attached by HTTP or the native bridge, or nil for direct Go calls.
// Requests cannot select it through JSON, headers, cookies or query parameters.
func Source(ctx context.Context) *RequestSource {
	source, _ := ctx.Value(sourceKey{}).(*RequestSource)
	return source
}
