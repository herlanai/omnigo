package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"golang.org/x/text/language"
)

type Catalog struct {
	source   language.Tag
	messages map[string]map[string][]segment
	tags     []language.Tag
	matcher  language.Matcher
}

// Load reads JSON/ARB catalogs named by BCP47 tag. It rejects malformed messages
// before returning an immutable catalog, suitable for sharing between windows.
func Load(fsys fs.FS, dir, source string) (*Catalog, error) {
	files, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	raw := map[string]map[string]string{}
	for _, file := range files {
		if file.IsDir() || (!strings.HasSuffix(file.Name(), ".json") && !strings.HasSuffix(file.Name(), ".arb")) {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		if len(b) > 8<<20 {
			return nil, fmt.Errorf("i18n: catalog too large")
		}
		locale := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
		if _, ok := raw[locale]; ok {
			return nil, fmt.Errorf("i18n: duplicate catalog %s", locale)
		}
		msgs, err := decode(b, locale)
		if err != nil {
			return nil, err
		}
		raw[locale] = msgs
	}
	return New(source, raw)
}
func decode(data []byte, locale string) (map[string]string, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("i18n: catalog must be object")
	}
	out := map[string]string{}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key := token.(string)
		if seen[key] {
			return nil, fmt.Errorf("i18n: duplicate key %s", key)
		}
		seen[key] = true
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return nil, err
		}
		if strings.HasPrefix(key, "@") {
			if key == "@@locale" {
				var s string
				if err = json.Unmarshal(raw, &s); err != nil || s != locale {
					return nil, fmt.Errorf("i18n: ARB locale does not match file")
				}
			}
			continue
		}
		var value string
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("i18n: %s must be string", key)
		}
		out[key] = value
	}
	if _, err = d.Token(); err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("i18n: trailing catalog data")
	}
	return out, nil
}
func New(source string, raw map[string]map[string]string) (*Catalog, error) {
	tag, err := language.Parse(source)
	if err != nil {
		return nil, err
	}
	c := &Catalog{source: tag, messages: map[string]map[string][]segment{}}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t, err := language.Parse(name)
		if err != nil {
			return nil, err
		}
		key := t.String()
		if _, ok := c.messages[key]; ok {
			return nil, fmt.Errorf("i18n: duplicate normalized locale %s", key)
		}
		messages := map[string][]segment{}
		for id, text := range raw[name] {
			if id == "" || strings.HasPrefix(id, "@") {
				return nil, fmt.Errorf("i18n: invalid message id")
			}
			compiled, err := compile(text)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", key, id, err)
			}
			messages[id] = compiled
		}
		c.messages[key] = messages
	}
	if _, ok := c.messages[tag.String()]; !ok {
		return nil, fmt.Errorf("i18n: source catalog missing")
	}
	c.tags = append(c.tags, tag)
	for _, name := range names {
		t, _ := language.Parse(name)
		if t != tag {
			c.tags = append(c.tags, t)
		}
	}
	c.matcher = language.NewMatcher(c.tags)
	return c, nil
}
func (c *Catalog) Languages() []string {
	out := make([]string, len(c.tags))
	for i, t := range c.tags {
		out[i] = t.String()
	}
	return out
}
func (c *Catalog) Resolve(locale string) (string, error) {
	t, err := language.Parse(locale)
	if err != nil {
		return "", err
	}
	_, index, confidence := c.matcher.Match(t)
	if confidence == language.No {
		return c.source.String(), nil
	}
	return c.tags[index].String(), nil
}
func (c *Catalog) Format(locale, id string, args map[string]any) (string, error) {
	selected, err := c.Resolve(locale)
	if err != nil {
		return "", err
	}
	tag, _ := language.Parse(selected)
	for t := tag; ; t = t.Parent() {
		if nodes, ok := c.messages[t.String()][id]; ok {
			var out strings.Builder
			if err := render(nodes, t, args, nil, &out); err != nil {
				return "", err
			}
			return out.String(), nil
		}
		if t == language.Und || t == t.Parent() {
			break
		}
	}
	if nodes, ok := c.messages[c.source.String()][id]; ok {
		var out strings.Builder
		if err := render(nodes, c.source, args, nil, &out); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	return "", fmt.Errorf("i18n: unknown message %q", id)
}

// Translator belongs to one UI goroutine. Switch language through dispatch when
// called by a worker; listeners execute synchronously on the caller's UI thread.
type Translator struct {
	catalog   *Catalog
	locale    string
	listeners map[uint64]func()
	next      uint64
}

func (c *Catalog) Translator(locale string) (*Translator, error) {
	s, err := c.Resolve(locale)
	if err != nil {
		return nil, err
	}
	return &Translator{catalog: c, locale: s, listeners: map[uint64]func(){}}, nil
}
func (t *Translator) Language() string { return t.locale }
func (t *Translator) Text(id string, args map[string]any) (string, error) {
	return t.catalog.Format(t.locale, id, args)
}
func (t *Translator) SetLanguage(locale string) error {
	s, err := t.catalog.Resolve(locale)
	if err != nil {
		return err
	}
	if s == t.locale {
		return nil
	}
	t.locale = s
	ids := make([]uint64, 0, len(t.listeners))
	for id := range t.listeners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if fn := t.listeners[id]; fn != nil {
			fn()
		}
	}
	return nil
}
func (t *Translator) Subscribe(fn func()) func() {
	if fn == nil {
		return func() {}
	}
	t.next++
	id := t.next
	t.listeners[id] = fn
	return func() { delete(t.listeners, id) }
}
func (t *Translator) RTL() bool {
	tag, _ := language.Parse(t.locale)
	script, _ := tag.Script()
	switch script.String() {
	case "Arab", "Hebr", "Thaa", "Nkoo", "Adlm", "Syrc":
		return true
	}
	return false
}
