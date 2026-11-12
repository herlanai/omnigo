// Package download stages hash-verified artifacts and serializes cache fills
// across processes. It does not install or execute downloaded code.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type File struct {
	URL, SHA256 string
	Size        int64       // Zero means unknown. The hash is always required.
	MaxBytes    int64       // Defaults to 64 MiB; at most 4 GiB.
	Mode        os.FileMode // Defaults to 0600. Set 0755 explicitly for native executables.
	Client      *http.Client
	Header      http.Header
	Progress    func(received, total int64) // Per attempt; runs on the caller's goroutine.
	Attempts    int                         // Defaults to four; maximum eight.
	wait        func(context.Context, time.Duration) error
}
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("download: HTTP status %d", e.Code) }
func validURL(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}
func (f File) normalized() (File, error) {
	digest, err := hex.DecodeString(f.SHA256)
	if err != nil || len(digest) != 32 || !validURL(f.URL) {
		return f, errors.New("download: invalid URL or SHA256")
	}
	f.SHA256 = strings.ToLower(f.SHA256)
	if f.MaxBytes == 0 {
		f.MaxBytes = 64 << 20
	}
	if f.Size < 0 || f.MaxBytes <= 0 || f.MaxBytes > 4<<30 || f.Size > f.MaxBytes {
		return f, errors.New("download: invalid size budget")
	}
	if f.Attempts == 0 {
		f.Attempts = 4
	}
	if f.Attempts < 1 || f.Attempts > 8 {
		return f, errors.New("download: invalid attempts")
	}
	if f.Mode == 0 {
		f.Mode = 0600
	}
	if f.Mode&^0777 != 0 {
		return f, errors.New("download: invalid file mode")
	}
	if f.wait == nil {
		f.wait = waitContext
	}
	f.Header = f.Header.Clone()
	return f, nil
}
func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("download: not a regular file")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func Check(path, digest string) bool {
	got, err := FileSHA256(path)
	return err == nil && strings.EqualFold(got, digest)
}
func Get(ctx context.Context, file File, path string) error {
	if ctx == nil {
		return errors.New("download: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := file.normalized()
	if err != nil {
		return err
	}
	return get(ctx, f, path)
}

// get accepts normalized metadata from Get, Stage or the locked cache path.
func get(ctx context.Context, f File, path string) error {
	for attempt := 0; attempt < f.Attempts; attempt++ {
		retry, err := attemptGet(ctx, f, path)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retry || attempt == f.Attempts-1 {
			return err
		}
		if err = f.wait(ctx, time.Duration(1<<attempt)*time.Second); err != nil {
			return err
		}
	}
	return errors.New("download: exhausted attempts")
}
func attemptGet(ctx context.Context, f File, path string) (retry bool, err error) {
	client := http.Client{Timeout: 10 * time.Minute}
	if f.Client != nil {
		client = *f.Client
	}
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !validURL(req.URL.String()) || req.URL.Scheme != via[0].URL.Scheme {
			return errors.New("download: redirect denied")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return false, err
	}
	req.Header = f.Header.Clone()
	response, err := client.Do(req)
	if err != nil {
		return retryNetwork(err), err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return response.StatusCode == 408 || response.StatusCode == 429 || response.StatusCode >= 500 && response.StatusCode <= 599, &StatusError{response.StatusCode}
	}
	if response.ContentLength > f.MaxBytes || f.Size > 0 && response.ContentLength >= 0 && response.ContentLength != f.Size {
		return false, errors.New("download: unexpected response size")
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".omnigo-download-*")
	if err != nil {
		return false, err
	}
	name := temp.Name()
	defer func() { temp.Close(); os.Remove(name) }()
	hash := sha256.New()
	buf := make([]byte, 64<<10)
	var received int64
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		n, readErr := response.Body.Read(buf)
		if n > 0 {
			received += int64(n)
			if received > f.MaxBytes || f.Size > 0 && received > f.Size {
				return false, errors.New("download: artifact exceeds size limit")
			}
			if _, err = temp.Write(buf[:n]); err != nil {
				return false, err
			}
			hash.Write(buf[:n])
			if f.Progress != nil {
				f.Progress(received, response.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			var network net.Error
			retry = errors.Is(readErr, io.ErrUnexpectedEOF) || errors.As(readErr, &network) && !network.Timeout()
			return retry, readErr
		}
	}
	if f.Size > 0 && received != f.Size {
		return false, errors.New("download: truncated artifact")
	}
	if hex.EncodeToString(hash.Sum(nil)) != f.SHA256 {
		return false, errors.New("download: SHA256 mismatch")
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	if err = temp.Chmod(f.Mode); err != nil {
		return false, err
	}
	if err = temp.Sync(); err != nil {
		return false, err
	}
	if err = temp.Close(); err != nil {
		return false, err
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	return false, os.Rename(name, path)
}
func retryNetwork(err error) bool {
	var request *url.Error
	if errors.As(err, &request) {
		err = request.Err
	}
	var dns *net.DNSError
	if errors.As(err, &dns) && !dns.IsTemporary {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var network net.Error
	return errors.As(err, &network) && !network.Timeout()
}

// Cached uses an OS advisory lock that is released if a process exits. The lock
// file is intentionally retained, so waiters always synchronize on one inode.
func Cached(ctx context.Context, file File, path string) error {
	if ctx == nil {
		return errors.New("download: nil context")
	}
	f, err := file.normalized()
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		acquired, err := tryLock(lock)
		if err != nil {
			return err
		}
		if acquired {
			break
		}
		if err = f.wait(ctx, 20*time.Millisecond); err != nil {
			return err
		}
	}
	defer unlock(lock)
	if err = ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	return cacheAfterLock(ctx, f, path, info, err)
}
func cacheAfterLock(ctx context.Context, f File, path string, info os.FileInfo, statErr error) error {
	if statErr == nil && info.Mode().IsRegular() && info.Size() <= f.MaxBytes && (f.Size == 0 || info.Size() == f.Size) && Check(path, f.SHA256) {
		if err := ctx.Err(); err != nil {
			return err
		}
		return os.Chmod(path, f.Mode)
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	return get(ctx, f, path)
}
