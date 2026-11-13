package omniweb

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

func (h *HTTP) events(w http.ResponseWriter, r *http.Request, raw []byte) {
	fail := func(status int, code string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(Fault{Code: code, Message: http.StatusText(status)})
	}
	if h.options.Events == nil {
		fail(404, "not_found")
		return
	}
	var input struct {
		Version int      `json:"v"`
		Topics  []string `json:"topics"`
	}
	if decode(raw, &input) != nil || input.Version != Version {
		fail(400, "invalid_request")
		return
	}
	sub, unsubscribe, err := h.options.Events.subscribe(input.Topics, h.options.Grants)
	if err != nil {
		status, code := 400, "invalid_request"
		switch {
		case errors.Is(err, ErrForbidden):
			status, code = 403, "forbidden"
		case errors.Is(err, ErrNotFound):
			status, code = 404, "not_found"
		case errors.Is(err, ErrBusy):
			status, code = 503, "busy"
		}
		fail(status, code)
		return
	}
	defer unsubscribe()
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "application/x-ndjson")
	send := func(frame eventFrame) bool {
		if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return false
		}
		if err := json.NewEncoder(w).Encode(frame); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if !send(eventFrame{Version: Version, Kind: "ready"}) {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case code := <-sub.end:
			send(eventFrame{Version: Version, Kind: "error", Code: code})
			return
		default:
		}
		select {
		case <-r.Context().Done():
			return
		case code := <-sub.end:
			send(eventFrame{Version: Version, Kind: "error", Code: code})
			return
		case frame := <-sub.queue:
			if !send(frame) {
				return
			}
		}
	}
}
