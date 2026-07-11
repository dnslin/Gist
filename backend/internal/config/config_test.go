package config_test

import (
	"gist/backend/internal/config"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGistUserAgent_DerivesAppVersion(t *testing.T) {
	require.Equal(
		t,
		"Mozilla/5.0 (compatible; Gist/"+config.AppVersion+"; +"+config.AppRepo+")",
		config.GistUserAgent,
	)
	require.Equal(t, config.GistUserAgent, config.DefaultUserAgent)
}
func TestLoad(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("GIST_ADDR", ":9999")
	t.Setenv("GIST_DATA_DIR", dataDir)
	t.Setenv("GIST_LOG_LEVEL", "debug")
	t.Setenv("GIST_PPROF_ADDR", "127.0.0.1:6060")

	cfg := config.Load()
	require.Equal(t, ":9999", cfg.Addr)
	require.Equal(t, filepath.Clean(dataDir), cfg.DataDir)
	require.Equal(t, filepath.Join(dataDir, "gist.db"), cfg.DBPath)
	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, "127.0.0.1:6060", cfg.PprofAddr)
}

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("GIST_ADDR")
	os.Unsetenv("GIST_DATA_DIR")
	os.Unsetenv("GIST_DB_PATH")
	os.Unsetenv("GIST_LOG_LEVEL")
	os.Unsetenv("GIST_PPROF_ADDR")
	os.Unsetenv("GIST_ENABLE_PPROF")

	cfg := config.Load()
	require.Equal(t, ":8080", cfg.Addr)
	require.Equal(t, "data", cfg.DataDir)
	require.Contains(t, cfg.DBPath, "gist.db")
	require.Equal(t, "info", cfg.LogLevel)
	require.Empty(t, cfg.PprofAddr)
}
