package omnigo

import (
	"context"
	"errors"
	"fmt"

	"github.com/herlanai/omnigo/runtime/rpc"
)

// Bind registers a typed service before Run. Existing RPC validation, concurrency
// limits and permission checks are shared with OmniWeb.
func Bind[Request, Response any](app *Application, name, permission string, handler func(context.Context, Request) (Response, error)) error {
	if app == nil {
		return errors.New("omnigo: nil application")
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.used {
		return errors.New("omnigo: configure services before Run")
	}
	return rpc.Bind(app.router, name, permission, handler)
}

// Method binds one explicitly named, statically typed RPC method. No reflection
// or automatic export of an object's other methods is involved.
func Method[Request, Response any](name string, handler func(context.Context, Request) (Response, error), permission ...string) Registration {
	return func(app *Application) error {
		if len(permission) > 1 {
			return fmt.Errorf("omnigo: %s accepts one permission", name)
		}
		grant := ""
		if len(permission) == 1 {
			grant = permission[0]
		}
		return Bind(app, name, grant, handler)
	}
}

// Query binds a read operation with an empty JSON object request.
func Query[Response any](name string, read func() Response, permission ...string) Registration {
	return func(app *Application) error {
		if read == nil {
			return fmt.Errorf("omnigo: %s requires a reader", name)
		}
		return Method(name, func(_ context.Context, _ struct{}) (Response, error) { return read(), nil }, permission...)(app)
	}
}
