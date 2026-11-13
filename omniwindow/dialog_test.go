package omniwindow

import "testing"

func TestDialogValidationAndSnapshot(t *testing.T) {
	original := FileDialogOptions{Filters: []FileFilter{{Extensions: []string{".png"}, MIMETypes: []string{"image/*"}}}}
	snapshot, err := PrepareFileDialog(original, false)
	if err != nil {
		t.Fatal(err)
	}
	original.Filters[0].Extensions[0] = "exe"
	if snapshot.Filters[0].Extensions[0] != "png" {
		t.Fatal("caller state leaked into snapshot")
	}
	for _, o := range []FileDialogOptions{{Title: "bad\x00"}, {Filters: []FileFilter{{Extensions: []string{"../txt"}}}}, {Filters: []FileFilter{{MIMETypes: []string{"invalid"}}}}} {
		if _, err := PrepareFileDialog(o, false); err == nil {
			t.Fatal("accepted invalid dialog", o)
		}
	}
	if _, err := PrepareFileDialog(FileDialogOptions{Multiple: true}, true); err == nil {
		t.Fatal("save multiple accepted")
	}
	message, err := PrepareMessageDialog(MessageDialogOptions{})
	if err != nil || len(message.Buttons) != 1 {
		t.Fatal(message, err)
	}
	for _, o := range []MessageDialogOptions{{Kind: "invalid"}, {DefaultButton: 1}, {CancelButton: -1}, {DestructiveButtons: []int{2}}, {Buttons: []string{""}}} {
		if _, err := PrepareMessageDialog(o); err == nil {
			t.Fatal("accepted invalid message", o)
		}
	}
}
