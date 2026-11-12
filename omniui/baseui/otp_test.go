package baseui

import "testing"

func TestOTPFilteredPasteSelectionUndoAndIME(t *testing.T) {
	s := surface(t)
	b := NewTextBuffer("")
	mount(t, s, InputOTP("otp", "Verification code", b, 6))
	s.Focus("otp")
	s.Paste("12 a３-456789")
	if b.String() != "124567" {
		t.Fatal("filter or length", b.String())
	}
	s.Key(KeyEvent{Key: "z", Command: true})
	if b.String() != "" {
		t.Fatal("paste was not one undo step")
	}
	s.Key(KeyEvent{Key: "Z", Command: true, Shift: true})
	if b.String() != "124567" {
		t.Fatal("redo failed")
	}
	b.Select(1, 3)
	s.Paste("x9y")
	if b.String() != "19567" {
		t.Fatal("selection paste", b.String())
	}
	b.Select(1, 2)
	s.TextInput("letters")
	if b.String() != "19567" || b.Selected() != "9" {
		t.Fatal("rejected input deleted selection")
	}
	s.SetComposition("8x")
	if b.String() != "19567" {
		t.Fatal("preedit committed early")
	}
	s.CommitComposition()
	if b.String() != "18567" {
		t.Fatal("composition not filtered", b.String())
	}
	b.Select(0, 2)
	s.CommitComposition()
	if b.String() != "18567" {
		t.Fatal("empty composition deleted selection")
	}
}
func TestOTPClipboardReadOnlyDisabledAndClick(t *testing.T) {
	s := surface(t)
	b := NewTextBuffer("1234")
	mount(t, s, InputOTP("otp", "Code", b, 6))
	s.Focus("otp")
	b.Select(1, 3)
	if text, ok := s.CopySelection(); !ok || text != "23" {
		t.Fatal(text, ok)
	}
	if text, ok, err := s.CutSelection(); err != nil || !ok || text != "23" || b.String() != "14" {
		t.Fatal(text, ok, err, b.String())
	}
	s.Key(KeyEvent{Key: "Z", Command: true})
	if b.String() != "1234" {
		t.Fatal("cut undo")
	}
	mount(t, s, InputOTP("otp", "Code", b, 6).ReadOnly(true))
	s.Focus("otp")
	b.Select(0, 4)
	if _, ok, err := s.CutSelection(); ok || err != nil {
		t.Fatal("read-only cut", ok, err)
	}
	s.Paste("999999")
	if b.String() != "1234" {
		t.Fatal("read-only paste")
	}
	mount(t, s, InputOTP("otp", "Code", b, 6))
	r, _ := s.Bounds("otp")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 85, Y: r.Y + 20})
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 85, Y: r.Y + 20})
	_, caret := b.Selection()
	if caret != 2 {
		t.Fatal("cell hit testing", caret)
	}
	mount(t, s, InputOTP("otp", "Code", b, 6).Disabled(true))
	s.Focus("otp")
	s.Paste("9")
	if b.String() != "1234" {
		t.Fatal("disabled paste")
	}
}
func TestOTPValidation(t *testing.T) {
	s := surface(t)
	mount(t, s, Button("keep", "Keep", nil))
	if err := s.Mount(InputOTP("bad", "Code", NewTextBuffer("abc"), 6)); err == nil {
		t.Fatal("invalid initial digits accepted")
	}
	if _, ok := s.Bounds("keep"); !ok {
		t.Fatal("invalid OTP destroyed mounted tree")
	}
}
