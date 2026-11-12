package clipboard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CF_HTML offsets are byte offsets, including for non-ASCII fragments.
func htmlClipboard(fragment []byte) []byte {
	const prefix = "<html><body><!--StartFragment-->"
	const suffix = "<!--EndFragment--></body></html>"
	header := "Version:1.0\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	start := len(fmt.Sprintf(header, 0, 0, 0, 0))
	content := prefix + string(fragment) + suffix
	return []byte(fmt.Sprintf(header, start, start+len(content), start+len(prefix), start+len(prefix)+len(fragment)) + content)
}
func htmlFragment(data []byte) ([]byte, error) {
	offset := func(key string) (int, error) {
		i := bytes.Index(data, []byte(key+":"))
		if i < 0 {
			return 0, errors.New("clipboard: missing HTML offset")
		}
		s := string(data[i+len(key)+1:])
		if at := strings.IndexByte(s, '\n'); at >= 0 {
			s = s[:at]
		}
		return strconv.Atoi(strings.TrimSpace(s))
	}
	start, err := offset("StartFragment")
	if err != nil {
		return nil, err
	}
	end, err := offset("EndFragment")
	if err != nil {
		return nil, err
	}
	if start < 0 || end < start || end > len(data) {
		return nil, errors.New("clipboard: invalid HTML bounds")
	}
	return bytes.Clone(data[start:end]), nil
}
func pngBytes(b []byte) ([]byte, error) {
	if len(b) < 8 || !bytes.Equal(b[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) {
		return nil, errors.New("clipboard: invalid PNG")
	}
	for at := 8; at+12 <= len(b); {
		n := uint64(binary.BigEndian.Uint32(b[at:]))
		end := uint64(at) + 12 + n
		if end > uint64(len(b)) {
			break
		}
		if string(b[at+4:at+8]) == "IEND" {
			if n != 0 {
				break
			}
			return bytes.Clone(b[:int(end)]), nil
		}
		at = int(end)
	}
	return nil, errors.New("clipboard: truncated PNG")
}
