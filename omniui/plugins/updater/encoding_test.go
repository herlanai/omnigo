package updater

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestSigningFileEncodings(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	manifest := []byte(`{"version":"2","url":"https://example.com/app","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1}`)
	signature := ed25519.Sign(private, manifest)
	for _, encoded := range []bool{false, true} {
		p, s := []byte(public), signature
		if encoded {
			p = []byte(base64.StdEncoding.EncodeToString(p) + "\r\n")
			s = []byte(base64.StdEncoding.EncodeToString(s) + "\n")
		}
		key, err := ParsePublicKey(p)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := ParseSignature(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Verify(manifest, sig, key); err != nil {
			t.Fatal(err)
		}
		sig[0] ^= 1
		if _, err = Verify(manifest, sig, key); err == nil {
			t.Fatal("tampered signature accepted")
		}
	}
	raw := bytes.Repeat([]byte{' '}, 32)
	key, err := ParsePublicKey(raw)
	if err != nil || !bytes.Equal(key, raw) {
		t.Fatal("raw bytes trimmed")
	}
	raw[0] = 'x'
	if key[0] == 'x' {
		t.Fatal("key aliases file buffer")
	}
	for _, bad := range [][]byte{nil, []byte("invalid"), []byte(base64.StdEncoding.EncodeToString(make([]byte, 31))), []byte(base64.StdEncoding.EncodeToString(private))} {
		if _, err = ParsePublicKey(bad); err == nil {
			t.Fatal("bad key accepted")
		}
	}
}
