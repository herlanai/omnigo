# Native WebView upload check

Run on macOS with `CGO_ENABLED=0 go run ./examples/webview-upload`. Add
`-embedded` to check a WebView inside a Go layout. This local fixture never uploads
file contents to a server.

Verify the visible file input and the custom avatar button both open a native
sheet. Select a disposable PNG/JPEG/WebP and confirm its name, size and readable
byte count appear. Cancel and reopen; try multiple files and directory selection.
Close the parent window with a chooser open and confirm clean shutdown. Repeat
with `-embedded`. Directory enumeration follows the installed WebKit version.

The framework returns selected NSURLs to WebKit, which owns the DOM FileList and
file-reading permissions. WKOpenPanelParameters supplies multiple/directory
selection but not HTML accept filters. Treat `accept` as a hint: validate image
type, size, and contents in the application (and again on the server if uploading).
This is a diagnostic example, not a distributable product bundle.
