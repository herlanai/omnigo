// Package omniui defines the shared component and extension model for OmniGo.
package omniui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
)

type Kind string

const (
	Control  Kind = "control"
	Service  Kind = "service"
	Renderer Kind = "renderer"
)

// Descriptor is copied on registration. IDs are stable names such as omniui.button.
// Requires names other extensions; Permissions declares host capabilities.
type Descriptor struct {
	ID          string
	Version     string
	Kind        Kind
	Requires    []string
	Permissions []string
}

// Stop must release resources even when ctx is canceled. Start must clean up its
// own partial work on failure; successfully started extensions stop in reverse order.
type Extension struct {
	Descriptor Descriptor
	Start      func(context.Context) error
	Stop       func(context.Context) error
}

type Registry struct {
	mu      sync.RWMutex
	entries map[string]Extension
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`)

func cloneDescriptor(d Descriptor) Descriptor {
	d.Requires = append([]string(nil), d.Requires...)
	d.Permissions = append([]string(nil), d.Permissions...)
	return d
}

func (r *Registry) Register(e Extension) error {
	d := e.Descriptor
	if !identifier.MatchString(d.ID) || d.Version == "" {
		return errors.New("omniui: extension needs a namespaced ID and version")
	}
	if d.Kind != Control && d.Kind != Service && d.Kind != Renderer {
		return fmt.Errorf("omniui: invalid extension kind %q", d.Kind)
	}
	e.Descriptor = cloneDescriptor(d)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = make(map[string]Extension)
	}
	if _, ok := r.entries[d.ID]; ok {
		return fmt.Errorf("omniui: duplicate extension %q", d.ID)
	}
	r.entries[d.ID] = e
	return nil
}

func (r *Registry) Catalog() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, cloneDescriptor(e.Descriptor))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Resolve returns a deterministic, independent dependency snapshot. Only selected
// extensions and their dependencies are linked into a session.
func (r *Registry) Resolve(ids []string, allowed []string) ([]Extension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	grants := make(map[string]bool, len(allowed))
	for _, p := range allowed {
		grants[p] = true
	}
	state := make(map[string]uint8)
	var out []Extension
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 2 {
			return nil
		}
		if state[id] == 1 {
			return fmt.Errorf("omniui: dependency cycle at %q", id)
		}
		e, ok := r.entries[id]
		if !ok {
			return fmt.Errorf("omniui: missing extension %q", id)
		}
		for _, p := range e.Descriptor.Permissions {
			if !grants[p] {
				return fmt.Errorf("omniui: %s requires permission %q", id, p)
			}
		}
		state[id] = 1
		e.Descriptor = cloneDescriptor(e.Descriptor)
		deps := append([]string(nil), e.Descriptor.Requires...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		out = append(out, e)
		return nil
	}
	roots := append([]string(nil), ids...)
	sort.Strings(roots)
	for _, id := range roots {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return out, nil
}
