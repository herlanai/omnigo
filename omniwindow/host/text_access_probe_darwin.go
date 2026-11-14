//go:build darwin && (amd64 || arm64)

package host

import (
	"context"
	"fmt"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"time"
)

// CheckTextAccessibility exercises real AppKit text-client and accessibility
// selectors. It does not enable VoiceOver or request system accessibility trust.
func CheckTextAccessibility() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := make(chan error, 1)
	buffer := ui.NewTextBuffer("A😀B")
	presses := 0
	err := Run(ctx, Options{InitialWindow: "input-check", CreateWindow: func(string, bool, WindowServices) (ContentWindowOptions, error) {
		return ContentWindowOptions{Title: "Native input and accessibility check", Width: 500, Height: 240, View: func() *ui.Element {
			return ui.Column("root", ui.Input("input", "Text input", buffer), ui.Button("button", "Apply", func() { presses++ }), ui.Button("disabled", "Unavailable", func() { presses += 100 }).Disabled(true))
		}}, nil
	}, OnReady: func(h *Controller) {
		go func() {
			defer cancel()
			result <- h.callUI(ctx, func() error {
				w := byName["input-check/false"]
				if w == nil {
					return fmt.Errorf("missing window")
				}
				w.surface.Focus("input")
				w.native.Send(sel("makeFirstResponder:"), w.view)
				if w.view.Send(sel("conformsToProtocol:"), objc.GetProtocol("NSTextInputClient")) == 0 {
					return fmt.Errorf("missing NSTextInputClient")
				}
				w.view.Send(sel("setMarkedText:selectedRange:replacementRange:"), stringID("拼😀"), nsRange{Location: 3}, nsRange{Location: nsNotFound})
				if buffer.String() != "A😀B" || w.view.Send(sel("hasMarkedText")) == 0 {
					return fmt.Errorf("marked text committed early")
				}
				marked := objc.Send[nsRange](w.view, sel("markedRange"))
				if marked.Location != 4 || marked.Length != 3 {
					return fmt.Errorf("wrong UTF16 marked range: %+v", marked)
				}
				selected := objc.Send[nsRange](w.view, sel("selectedRange"))
				if selected.Location != 7 {
					return fmt.Errorf("wrong UTF16 selection: %+v", selected)
				}
				var actual nsRange
				r := objc.Send[rect](w.view, sel("firstRectForCharacterRange:actualRange:"), marked, &actual)
				if r.S.H <= 0 || r.S.W <= 0 {
					return fmt.Errorf("missing candidate rectangle")
				}
				part := w.view.Send(sel("attributedSubstringForProposedRange:actualRange:"), nsRange{Location: 4, Length: 3}, &actual)
				if cocoaText(part) != "拼😀" {
					return fmt.Errorf("wrong marked substring")
				}
				w.view.Send(sel("insertText:replacementRange:"), stringID("拼音"), nsRange{Location: nsNotFound})
				if buffer.String() != "A😀B拼音" || w.view.Send(sel("hasMarkedText")) != 0 {
					return fmt.Errorf("native commit failed")
				}
				if !buffer.Undo() || buffer.String() != "A😀B" {
					return fmt.Errorf("native composition undo failed")
				}
				if err := paint(w, h); err != nil {
					return err
				}
				address, lookupErr := purego.Dlsym(purego.RTLD_DEFAULT, "NSAccessibilityUnignoredChildrenForOnlyChild")
				if lookupErr != nil {
					return lookupErr
				}
				var exposedChildren func(objc.ID) objc.ID
				purego.RegisterFunc(&exposedChildren, address)
				if exposedChildren(w.view).Send(sel("count")) == 0 {
					return fmt.Errorf("AppKit accessibility traversal exposed no controls")
				}
				button := w.access["button"]
				if button == 0 || cocoaText(button.Send(sel("accessibilityRole"))) != "AXButton" {
					return fmt.Errorf("missing native AX button")
				}
				if button.Send(sel("accessibilityPerformPress")) == 0 || presses != 1 {
					return fmt.Errorf("AX press failed")
				}
				if w.access["disabled"].Send(sel("accessibilityPerformPress")) != 0 || presses != 1 {
					return fmt.Errorf("disabled AX action accepted")
				}
				input := w.access["input"]
				input.Send(sel("setAccessibilityValue:"), stringID("A😀B"))
				input.Send(sel("setAccessibilitySelectedTextRange:"), nsRange{Location: 1, Length: 2})
				if cocoaText(input.Send(sel("accessibilitySelectedText"))) != "😀" {
					return fmt.Errorf("AX UTF16 selection failed")
				}
				if input.Send(sel("accessibilityNumberOfCharacters")) != 4 {
					return fmt.Errorf("AX character count failed")
				}
				input.Send(sel("setAccessibilityValue:"), stringID("读屏"))
				if buffer.String() != "读屏" {
					return fmt.Errorf("AX text edit failed")
				}
				if objc.Send[rect](input, sel("accessibilityFrame")).S.H <= 0 {
					return fmt.Errorf("missing AX bounds")
				}
				// Route an actual NSEvent through interpretKeyEvents. The active input
				// source may produce committed or marked text; either is acceptable.
				input.Send(sel("setAccessibilityValue:"), stringID(""))
				event := objc.ID(objc.GetClass("NSEvent")).Send(sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"), uint64(10), point{}, uint64(0), float64(0), int64(w.native.Send(sel("windowNumber"))), objc.ID(0), stringID("a"), stringID("a"), false, uint16(0))
				w.view.Send(sel("keyDown:"), event)
				typed, ok := w.surface.NativeText()
				if !ok || typed.Text == "" {
					return fmt.Errorf("native keyboard event did not reach text client (context=%x responder=%x view=%x key=%d event=%x)", w.view.Send(sel("inputContext")), w.native.Send(sel("firstResponder")), w.view, w.native.Send(sel("isKeyWindow")), event)
				}
				w.surface.CancelComposition()
				// Hold a native reference across destruction: stale callbacks must become inert.
				button.Send(sel("retain"))
				defer button.Send(sel("release"))
				w.native.Send(sel("close"))
				if w.textContext != 0 || len(w.access) != 0 {
					return fmt.Errorf("native input/accessibility resources retained after close")
				}
				if button.Send(sel("accessibilityPerformPress")) != 0 {
					return fmt.Errorf("stale AX action accepted")
				}
				return nil
			})
		}()
	}})
	if err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
	default:
		return fmt.Errorf("native input probe timed out: %v", ctx.Err())
	}
}
