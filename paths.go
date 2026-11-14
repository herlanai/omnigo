//go:build !ios && !android

package omnigo

import "github.com/herlanai/omnigo/omniwindow/system"

type PathName = system.PathName

const (
	PathHome      = system.PathHome
	PathAppData   = system.PathAppData
	PathUserData  = system.PathUserData
	PathCache     = system.PathCache
	PathLogs      = system.PathLogs
	PathResources = system.PathResources
	PathTemp      = system.PathTemp
	PathExe       = system.PathExe
	PathDesktop   = system.PathDesktop
	PathDocuments = system.PathDocuments
	PathDownloads = system.PathDownloads
	PathMusic     = system.PathMusic
	PathPictures  = system.PathPictures
	PathVideos    = system.PathVideos
)

// Path resolves a standard application location. It works before Run and after
// shutdown; user-data, cache and log directories are created on demand.
func (a *Application) Path(name PathName) (string, error) { return a.paths.Get(name) }

// SetPath overrides a location without creating it. Empty restores the default.
func (a *Application) SetPath(name PathName, path string) error { return a.paths.Set(name, path) }

// Name is the stable application name used to scope its writable directories.
func (a *Application) Name() string { return a.options.Name }
