package omnibrowser

import (
	"context"
	"testing"
)

func TestThemeBootstrapAndCurrentScript(t *testing.T) {
	p := New(Options{})
	defer p.Close()
	ctx := context.Background()
	if e := p.SetContent(ctx, `<div id="before"></div><script>document.currentScript.previousElementSibling.setAttribute('data-ran','yes');document.documentElement.classList.add(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light')</script>`); e != nil {
		t.Fatal(e)
	}
	v, e := p.Eval(ctx, `document.documentElement.classList.contains('light') && document.querySelector('#before').getAttribute('data-ran') === 'yes' && document.currentScript === null`)
	if e != nil || string(v) != "true" {
		t.Fatal(string(v), e)
	}
	v, e = p.Eval(ctx, `var c=document.documentElement.classList;c.toggle('light',false);c.add('a','b');c.replace('a','z');c.remove('b');c.value`)
	if e != nil || string(v) != `"z"` {
		t.Fatal(string(v), e)
	}
}
