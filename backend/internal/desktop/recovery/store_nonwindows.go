//go:build !windows

package recovery

import "os"

func atomicReplace(source, target string) error {
	return os.Rename(source, target)
}

func syncDirectory(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func securePath(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}
