package omnigo

import (
	"fmt"
)

// Registration registers an application's typed RPC methods and routes before Run.
// Registration must not allocate live resources; use Services for those.
type Registration func(*Application) error

// Group groups registrations and returns the first configuration error. Like
// New, it performs no resource startup; a failed application should be discarded.
func Group(modules ...Registration) Registration {
	return func(app *Application) error {
		for _, module := range modules {
			if module == nil {
				return fmt.Errorf("omnigo: nil registration")
			}
			if err := module(app); err != nil {
				return err
			}
		}
		return nil
	}
}
