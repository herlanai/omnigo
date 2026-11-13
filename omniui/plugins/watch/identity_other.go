//go:build !darwin && !linux

package watch

import "io/fs"

// Other platforms retain conservative comparison until stable native identity
// capture is available. In particular Windows Lstat identity can be deferred.
func fileIdentity(fs.FileInfo) (identity, bool) { return identity{}, false }
