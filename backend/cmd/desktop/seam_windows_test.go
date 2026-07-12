//go:build windows

package desktop_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopHostHasNoWailsDependency(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	for _, dependency := range strings.Split(string(output), "\n") { require.NotContains(t, dependency, "github.com/wailsapp/") }
}
