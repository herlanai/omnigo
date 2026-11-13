//go:build windows

package system

import "golang.org/x/sys/windows"

func userFolder(name PathName) (string, error) {
	ids := map[PathName]*windows.KNOWNFOLDERID{PathDesktop: windows.FOLDERID_Desktop, PathDocuments: windows.FOLDERID_Documents, PathDownloads: windows.FOLDERID_Downloads, PathMusic: windows.FOLDERID_Music, PathPictures: windows.FOLDERID_Pictures, PathVideos: windows.FOLDERID_Videos}
	return windows.KnownFolderPath(ids[name], windows.KF_FLAG_DEFAULT)
}
