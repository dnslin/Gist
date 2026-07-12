package paths_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/desktop/paths"
)

type resolverFunc func() (string, error)

func (f resolverFunc) ResolveLocalAppData() (string, error) { return f() }

func TestResolveUsesOnlyLocalAppDataAndHasNoSideEffects(t *testing.T) {
	local := filepath.Join(t.TempDir(), "Local App Data")
	cwd := t.TempDir()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(cwd))
	t.Cleanup(func() { require.NoError(t, os.Chdir(old)) })
	t.Setenv("GIST_DATA_DIR", filepath.Join(t.TempDir(), "server-data"))
	t.Setenv("GIST_DB_PATH", filepath.Join(t.TempDir(), "server.db"))

	got, err := paths.Resolve(resolverFunc(func() (string, error) { return local, nil }))
	require.NoError(t, err)
	root, err := filepath.Abs(filepath.Join(local, "Gist"))
	require.NoError(t, err)
	require.Equal(t, paths.Paths{
		Root:        root,
		DataDir:     filepath.Join(root, "data"),
		DBPath:      filepath.Join(root, "data", "gist.db"),
		ConfigPath:  filepath.Join(root, "desktop.json"),
		RecoveryDir: filepath.Join(root, "recovery"),
		LogsDir:     filepath.Join(root, "logs"),
		UpdatesDir:  filepath.Join(root, "updates"),
		WebViewDir:  filepath.Join(root, "webview"),
	}, got)
	_, err = os.Stat(root)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestResolveRejectsInvalidRoots(t *testing.T) {
	volumeRoot := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	invalidRoots := []string{"", ".", "relative", string(filepath.Separator), volumeRoot, filepath.Join(t.TempDir(), "bad") + "\x00suffix"}
	for _, root := range invalidRoots {
		t.Run(root, func(t *testing.T) {
			_, err := paths.Resolve(resolverFunc(func() (string, error) { return root, nil }))
			require.ErrorIs(t, err, paths.ErrUnavailable)
		})
	}

	resolverErr := errors.New("known folder failed")
	_, err := paths.Resolve(resolverFunc(func() (string, error) { return "", resolverErr }))
	require.ErrorIs(t, err, paths.ErrUnavailable)
}
