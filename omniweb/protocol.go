// Package omniweb defines the versioned Web UI protocol shared by application hosts
// and browser transports. It reuses the same service router as native OmniUI.
package omniweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"

	"github.com/herlanai/omnigo/internal/runtime/rpc"
)

const Version = 1
const Prefix = "/omniweb/v1/"

type Request struct {
	Version int             `json:"v"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Fault struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	Version int             `json:"v"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Fault          `json:"error,omitempty"`
}

var methodName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{0,127}$`)

func validate(r Request) error {
	if r.Version != Version || r.ID == "" || len(r.ID) > 128 || !methodName.MatchString(r.Method) || len(r.Params) == 0 || !json.Valid(r.Params) {
		return errors.New("invalid OmniWeb request")
	}
	return nil
}

func decode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func execute(ctx context.Context, router *rpc.Router, request Request, grants []string) Response {
	out, err := router.Call(ctx, request.Method, request.Params, grants)
	response := Response{Version: Version, ID: request.ID, Result: out}
	if err == nil {
		return response
	}
	code, message := "internal", "The service could not complete the request"
	switch {
	case errors.Is(err, rpc.ErrInvalidRequest):
		code, message = "invalid_params", "Invalid method parameters"
	case errors.Is(err, rpc.ErrNotFound):
		code, message = "not_found", "Unknown method"
	case errors.Is(err, rpc.ErrForbidden):
		code, message = "forbidden", "Permission denied"
	case errors.Is(err, rpc.ErrBusy):
		code, message = "busy", "Service concurrency limit reached"
	case errors.Is(err, rpc.ErrTooLarge):
		code, message = "too_large", "Request exceeds limit"
	case errors.Is(err, context.Canceled):
		code, message = "canceled", "Request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "deadline", "Request deadline exceeded"
	}
	response.Result = nil
	response.Error = &Fault{Code: code, Message: message}
	return response
}
