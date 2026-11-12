package omnibrowser

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLocatorStrictActionsAndCancellation(t *testing.T) {
	p := New(Options{})
	defer p.Close()
	ctx := context.Background()
	if err := p.SetContent(ctx, `<style>.hidden{display:none}</style><input id="name"><button onclick="document.title=document.querySelector('#name').value">提交</button><p>甲</p><p>乙</p><input id="locked" readonly><button class="hidden">隐藏</button>`); err != nil {
		t.Fatal(err)
	}
	if err := p.Locator("#name").Fill(ctx, "中文"); err != nil {
		t.Fatal(err)
	}
	if err := p.Locator("button").FilterText("提交").Click(ctx); err != nil {
		t.Fatal(err)
	}
	s, _ := p.Snapshot(ctx)
	if s.Title != "中文" {
		t.Fatal(s.Title)
	}
	if _, err := p.Locator("p").TextContent(ctx); err == nil {
		t.Fatal("ambiguous selector accepted")
	}
	if got, err := p.Locator("p").Nth(1).TextContent(ctx); err != nil || got != "乙" {
		t.Fatal(got, err)
	}
	if err := p.Locator(".hidden").WaitFor(ctx, "hidden"); err != nil {
		t.Fatal(err)
	}
	timeout, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := p.Locator("#locked").Fill(timeout, "bad"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if err := p.SetContent(ctx, `<p>新文档</p>`); err != nil {
		t.Fatal(err)
	}
	if got, err := p.Locator("p").TextContent(ctx); err != nil || got != "新文档" {
		t.Fatal(got, err)
	}
}
func TestStreamingDocumentRevealsBoundary(t *testing.T) {
	p := New(Options{})
	defer p.Close()
	ctx := context.Background()
	err := p.SetContent(ctx, `<main><!--$?--><template id="slot"></template><!--/$--></main><div hidden id="content"><p>流式正文</p></div><script>requestAnimationFrame(()=>{let t=document.getElementById('slot'),s=document.getElementById('content');t.previousSibling.data='$';while(s.firstChild)t.parentNode.insertBefore(s.firstChild,t);t.parentNode.removeChild(t);s.parentNode.removeChild(s);});</script>`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := p.Snapshot(ctx)
	if s.Text != "流式正文" || len(s.ScriptErrors) > 0 {
		t.Fatal(s.Text, s.ScriptErrors)
	}
}
