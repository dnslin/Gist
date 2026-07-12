package recovery

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const removalMarkerFilename = ".journal.removed"

var ErrAbsent = errors.New("recovery journal absent")

type FileStore struct {
	dir        string
	checkpoint func(string) error
}

func NewFileStore(dir string, checkpoint func(string) error) *FileStore {
	return &FileStore{dir: filepath.Clean(dir), checkpoint: checkpoint}
}

func (s *FileStore) reached(step string) error {
	if s.checkpoint != nil {
		return s.checkpoint(step)
	}
	return nil
}

func (s *FileStore) path() string {
	return filepath.Join(s.dir, JournalFilename)
}

func (s *FileStore) removalMarkerPath() string {
	return filepath.Join(s.dir, removalMarkerFilename)
}

func (s *FileStore) valid() bool {
	return s != nil && s.dir != "" && s.dir != "." && filepath.IsAbs(s.dir)
}

func (s *FileStore) Load(_ context.Context) ([]byte, error) {
	if !s.valid() {
		return nil, os.ErrInvalid
	}
	file, err := os.Open(s.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrAbsent
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, MaxRecordSize+1))
}


func (s *FileStore) Replace(_ context.Context, data []byte) error {
	if !s.valid() {
		return os.ErrInvalid
	}
	if len(data) > MaxRecordSize {
		return ErrCorrupt
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := securePath(s.dir, 0o700); err != nil {
		return err
	}
	if err := s.reached("temp_create"); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.dir, ".journal-*.tmp")
	if err != nil {
		return err
	}
	temp := file.Name()
	keep := true
	defer func() {
		_ = file.Close()
		if keep {
			_ = os.Remove(temp)
		}
	}()
	if err := securePath(temp, 0o600); err != nil {
		return err
	}
	if err := s.reached("temp_write"); err != nil {
		return err
	}
	if _, err := io.Copy(file, bytesReader(data)); err != nil {
		return err
	}
	if err := s.reached("file_sync"); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := s.reached("atomic_replace"); err != nil {
		return err
	}
	if err := atomicReplace(temp, s.path()); err != nil {
		return err
	}
	keep = false
	if err := s.reached("directory_sync"); err != nil {
		return err
	}
	return syncDirectory(s.dir)
}

func (s *FileStore) Remove(_ context.Context) error {
	if !s.valid() {
		return os.ErrInvalid
	}
	if _, err := os.Stat(s.path()); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if err := atomicReplace(s.path(), s.removalMarkerPath()); err != nil {
		return err
	}
	if err := s.reached("directory_sync"); err != nil {
		return err
	}
	return syncDirectory(s.dir)
}

type byteReader struct {
	data   []byte
	offset int
}

func bytesReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}
