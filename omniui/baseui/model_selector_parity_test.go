package baseui

import (
	"strings"
	"testing"
)

func TestModelProviderOrderDescriptionsAndFallbackNames(t *testing.T) {
	s := surface(t)
	state := &ModelSelectorState{Model: "a"}
	models := []ModelOption{{ID: "a", Provider: "First", Description: "First description"}, {ID: "b", Name: "Second", Provider: "Second"}, {ID: "c", Name: "Third", Provider: "First"}, {ID: "d", Provider: ""}}
	view := func() *Element { return ModelSelector("models", state, models, nil) }
	mount(t, s, view())
	click(t, s, "models/model/trigger")
	mount(t, s, view())
	positions := []float64{}
	for _, id := range []string{"models/model/option/a", "models/model/option/c", "models/model/option/b", "models/model/option/d"} {
		r, ok := s.Bounds(id)
		if !ok {
			t.Fatal(id)
		}
		positions = append(positions, r.Y)
	}
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatal(positions)
		}
	}
	a := s.entries[s.index["models/model/option/a"]]
	if a.element.label != "a" || a.element.detail != "First description" || a.bounds.H != 56 {
		t.Fatal(a)
	}
	if !s.Focus("models/model/option/c") {
		t.Fatal("item focus")
	}
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, view())
	if s.focus != "models/model/option/b" {
		t.Fatal("navigation not in visual group order", s.focus)
	}
}
func TestModelSelectionCallbackOnlyForActualChanges(t *testing.T) {
	s := surface(t)
	calls := 0
	state := &ModelSelectorState{Model: "a", Effort: "unsupported"}
	models := []ModelOption{{ID: "a", Name: "A", Efforts: []ModelEffort{{ID: "low", Name: "Low"}}}, {ID: "b", Name: "B", Efforts: []ModelEffort{{ID: "low"}, {ID: "high", Name: "High"}}}, {ID: "c", Name: "C"}}
	view := func() *Element { return ModelSelector("models", state, models, func(string, string) { calls++ }) }
	choose := func(trigger, row string) {
		mount(t, s, view())
		click(t, s, trigger)
		mount(t, s, view())
		click(t, s, row)
	}
	choose("models/model/trigger", "models/model/option/a")
	if calls != 0 || state.Effort != "unsupported" {
		t.Fatal("same model modified application state", calls, state)
	}
	choose("models/model/trigger", "models/model/option/b")
	if calls != 1 || state.Effort != "low" {
		t.Fatal(calls, state)
	}
	choose("models/effort/trigger", "models/effort/option/low")
	if calls != 1 {
		t.Fatal("same effort notified")
	}
	choose("models/effort/trigger", "models/effort/option/high")
	if calls != 2 || state.Effort != "high" {
		t.Fatal(calls, state)
	}
	choose("models/model/trigger", "models/model/option/c")
	mount(t, s, view())
	if calls != 3 || state.Effort != "" {
		t.Fatal(calls, state)
	}
	if _, ok := s.Bounds("models/effort"); ok {
		t.Fatal("effort control not hidden")
	}
}
func TestModelDynamicDisabledEmptyAndSnapshot(t *testing.T) {
	s := surface(t)
	state := &ModelSelectorState{Model: "a", Effort: "low"}
	models := []ModelOption{{ID: "a", Name: "Alpha", Efforts: []ModelEffort{{ID: "low", Name: "Low"}}}}
	view := func() *Element { return ModelSelector("models", state, models, nil) }
	mount(t, s, view())
	models[0].Name = "Mutated"
	if !strings.Contains(s.entries[s.index["models/model/trigger"]].element.label, "Alpha") {
		t.Fatal("mounted label mutated")
	}
	models[0].Disabled = true
	mount(t, s, view())
	if _, ok := s.Bounds("models/effort/trigger"); !ok {
		t.Fatal("disabled model effort disappeared")
	}
	if s.Focus("models/effort/trigger") {
		t.Fatal("disabled model effort editable")
	}
	models = nil
	mount(t, s, view())
	if s.Focus("models/model/trigger") {
		t.Fatal("empty model list opens")
	}
	if state.Model != "a" || state.Effort != "low" {
		t.Fatal("list refresh mutated app selection")
	}
}
func TestModelDescriptionsClipInBothThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(360, 300, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		state := &ModelSelectorState{}
		view := func() *Element {
			return ModelSelector("models", state, []ModelOption{{ID: "a", Name: "Long title", Provider: "Provider", Description: strings.Repeat("Unicode café ", 40)}}, nil)
		}
		mount(t, s, view())
		click(t, s, "models/model/trigger")
		mount(t, s, view())
		if _, err = s.RenderScaled(1.25, nil); err != nil {
			t.Fatal(err)
		}
		s.Close()
	}
}

func TestModelSelectorPlaceholderAndOptionsDisabled(t *testing.T) {
	s := surface(t)
	state := &ModelSelectorState{}
	models := []ModelOption{{ID: "a"}}
	mount(t, s, ModelSelector("models", state, models, nil, ModelSelectorOptions{Placeholder: "Choose a provider model", Disabled: true}))
	if s.Focus("models/model/trigger") {
		t.Fatal("options disabled ignored")
	}
	if got := s.entries[s.index["models/model/trigger"]].element.text; got != "Choose a provider model" {
		t.Fatal(got)
	}
}
