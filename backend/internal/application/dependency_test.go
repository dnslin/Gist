package application_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLinuxDependencyClosureExcludesDesktopAndWindowsHosts(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	for _, dependency := range strings.Split(string(output), "\n") {
		require.NotContains(t, dependency, "github.com/wailsapp/")
		require.NotContains(t, dependency, "gist/backend/internal/desktop")
		require.NotEqual(t, "golang.org/x/sys/windows", dependency)
	}
}
