//go:build windows

package paths

import "golang.org/x/sys/windows"

type WindowsLocalAppDataResolver struct{}

func (WindowsLocalAppDataResolver) ResolveLocalAppData() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_LocalAppData, windows.KF_FLAG_DONT_VERIFY)
}
