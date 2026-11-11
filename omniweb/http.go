package omniweb

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"omnigo.dev/omnigo/runtime/rpc"
)

// HTTPOptions is host configuration, never deserialized from a client message.
// Host is the exact backend authority, including port. Origins are exact browser
// origins; no wildcards, credentials, paths or opaque/null origins are accepted.
type HTTPOptions struct {
	Host             string
	Origins          []string
	Grants           []string
	MaxRequestBytes  int64
	MaxResponseBytes int
	MaxBatch         int
}

type HTTP struct {
	router  *rpc.Router
	options HTTPOptions
	origins map[string]bool
}

func NewHTTP(router *rpc.Router, o HTTPOptions) (*HTTP, error) {
	if router == nil || o.Host == "" || len(o.Origins) == 0 {
		return nil, errors.New("omniweb: router, host and origins required")
	}
	u, err := url.Parse("http://" + o.Host)
	if err != nil || u.Host != o.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("omniweb: invalid host")
	}
	if o.MaxRequestBytes <= 0 {
		o.MaxRequestBytes = 1 << 20
	}
	if o.MaxResponseBytes <= 0 {
		o.MaxResponseBytes = 4 << 20
	}
	if o.MaxBatch <= 0 {
		o.MaxBatch = 64
	}
	o.Grants = append([]string(nil), o.Grants...)
	o.Origins = append([]string(nil), o.Origins...)
	h := &HTTP{router: router, options: o, origins: make(map[string]bool)}
	for _, origin := range o.Origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != origin {
			return nil, errors.New("omniweb: invalid origin")
		}
		h.origins[origin] = true
	}
	return h, nil
}

func (h *HTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	fail := func(status int, code string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(Fault{Code: code, Message: http.StatusText(status)})
	}
	if r.Host != h.options.Host {
		fail(http.StatusForbidden, "host")
		return
	}
	if r.URL.Path != Prefix+"call" && r.URL.Path != Prefix+"batch" {
		fail(http.StatusNotFound, "path")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		fail(http.StatusMethodNotAllowed, "method")
		return
	}
	if !h.origins[r.Header.Get("Origin")] {
		fail(http.StatusForbidden, "origin")
		return
	}
	// Requests use a same-origin host or dev proxy. Deliberately no permissive CORS.
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(http.StatusUnsupportedMediaType, "content_type")
		return
	}
	if r.Header.Get("Content-Encoding") != "" {
		fail(http.StatusUnsupportedMediaType, "content_encoding")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.options.MaxRequestBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			fail(http.StatusRequestEntityTooLarge, "too_large")
		} else {
			fail(http.StatusBadRequest, "body")
		}
		return
	}
	var requests []Request
	batch := r.URL.Path == Prefix+"batch"
	if batch {
		err = decode(raw, &requests)
	} else {
		var request Request
		err = decode(raw, &request)
		requests = []Request{request}
	}
	if err != nil || len(requests) == 0 || len(requests) > h.options.MaxBatch {
		fail(http.StatusBadRequest, "invalid_request")
		return
	}
	seen := make(map[string]bool, len(requests))
	// Validate the entire batch before any service side effect. Execution itself
	// is sequential and non-transactional, preserving caller order and limits.
	for _, request := range requests {
		if validate(request) != nil || seen[request.ID] {
			fail(http.StatusBadRequest, "invalid_request")
			return
		}
		seen[request.ID] = true
	}
	responses := make([]Response, 0, len(requests))
	for _, request := range requests {
		responses = append(responses, execute(r.Context(), h.router, request, h.options.Grants))
	}
	var value any = responses
	if !batch {
		value = responses[0]
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		fail(http.StatusInternalServerError, "encoding")
		return
	}
	if len(encoded) > h.options.MaxResponseBytes {
		fail(http.StatusInternalServerError, "response_too_large")
		return
	}
	_, _ = w.Write(encoded)
}
