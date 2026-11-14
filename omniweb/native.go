package omniweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// NativeExchange serves unary/batch RPC through a trusted native window bridge.
// The host must validate the sending frame's origin before invoking it. Grants
// are host-owned and copied; clients cannot supply permissions in their payload.
func NativeExchange(router *Router, grants []string) func(context.Context, []byte) ([]byte, error) {
	return nativeExchange(router, grants, &RequestSource{})
}

// NativeExchange shares this HTTP host's grants and source identity with its native window.
func (h *HTTP) NativeExchange() func(context.Context, []byte) ([]byte, error) {
	return nativeExchange(h.router, h.options.Grants, h.source)
}

func nativeExchange(router *Router, grants []string, source *RequestSource) func(context.Context, []byte) ([]byte, error) {
	grants = append([]string(nil), grants...)
	return func(ctx context.Context, raw []byte) ([]byte, error) {
		if router == nil || ctx == nil {
			return nil, fmt.Errorf("omniweb: native host is not configured")
		}
		if len(raw) > 1<<20 {
			return nil, fmt.Errorf("omniweb: native request exceeds 1 MiB")
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			return nil, fmt.Errorf("omniweb: empty native request")
		}
		batch := raw[0] == '['
		var requests []Request
		if batch {
			if err := decode(raw, &requests); err != nil {
				return nil, err
			}
		} else {
			var r Request
			if err := decode(raw, &r); err != nil {
				return nil, err
			}
			requests = []Request{r}
		}
		if len(requests) == 0 || len(requests) > 64 {
			return nil, fmt.Errorf("omniweb: invalid native batch")
		}
		seen := map[string]bool{}
		for _, r := range requests {
			if validate(r) != nil || seen[r.ID] {
				return nil, fmt.Errorf("omniweb: invalid native request")
			}
			seen[r.ID] = true
		}
		ctx = context.WithValue(ctx, sourceKey{}, source)
		responses := make([]Response, 0, len(requests))
		for _, r := range requests {
			responses = append(responses, execute(ctx, router, r, grants))
		}
		var value any = responses
		if !batch {
			value = responses[0]
		}
		out, err := json.Marshal(value)
		if len(out) > 4<<20 {
			return nil, fmt.Errorf("omniweb: native response exceeds 4 MiB")
		}
		return out, err
	}
}
