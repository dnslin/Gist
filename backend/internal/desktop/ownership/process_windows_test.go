//go:build windows

package ownership_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"gist/backend/internal/desktop/ownership"
)

func TestProcessOwnerHelper(t *testing.T) {
	if os.Getenv("GIST_OWNERSHIP_HELPER") != "1" {
		return
	}
	identity, err := ownership.CurrentIdentity(os.Getenv("GIST_OWNERSHIP_ROOT"))
	require.NoError(t, err)
	acquisition, err := (ownership.WindowsAcquirer{}).Acquire(context.Background(), identity)
	require.NoError(t, err)
	require.Equal(t, ownership.OutcomeAcquired, acquisition.Outcome)
	fmt.Println("READY")
	select {}
}

func startOwnerProcess(t *testing.T, root string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestProcessOwnerHelper")
	cmd.Env = append(os.Environ(), "GIST_OWNERSHIP_HELPER=1", "GIST_OWNERSHIP_ROOT="+root)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "READY", strings.TrimSpace(line))
	return cmd
}

func TestProcessContentionDACLAndOwnerKillReacquire(t *testing.T) {
	root := t.TempDir()
	cmd := startOwnerProcess(t, root)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	identity, err := ownership.CurrentIdentity(root)
	require.NoError(t, err)
	loser, err := (ownership.WindowsAcquirer{}).Acquire(context.Background(), identity)
	require.NoError(t, err)
	require.Equal(t, ownership.OutcomeOwnedSameSession, loser.Outcome)
	require.NotZero(t, loser.Owner.PID)
	require.NotEqual(t, [16]byte{}, loser.Owner.Nonce)

	mutexName, err := windows.UTF16PtrFromString(identity.MutexName)
	require.NoError(t, err)
	mutex, err := windows.OpenMutex(windows.READ_CONTROL, false, mutexName)
	require.NoError(t, err)
	mutexSD, err := windows.GetSecurityInfo(mutex, windows.SE_KERNEL_OBJECT, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	require.NoError(t, windows.CloseHandle(mutex))
	require.Contains(t, mutexSD.String(), "D:P")
	require.Contains(t, mutexSD.String(), ";;;SY)")
	require.Contains(t, mutexSD.String(), identity.UserSID)

	metadataKey, err := registry.OpenKey(registry.CURRENT_USER, identity.MetadataName, registry.READ)
	require.NoError(t, err)
	metadataSD, err := windows.GetSecurityInfo(windows.Handle(metadataKey), windows.SE_REGISTRY_KEY, windows.DACL_SECURITY_INFORMATION)
	require.NoError(t, err)
	require.NoError(t, metadataKey.Close())
	require.Contains(t, metadataSD.String(), "D:P")
	require.Contains(t, metadataSD.String(), ";;;SY)")
	require.Contains(t, metadataSD.String(), identity.UserSID)

	require.NoError(t, cmd.Process.Kill())
	_, _ = cmd.Process.Wait()
	var winner ownership.Acquisition
	require.Eventually(t, func() bool {
		winner, err = (ownership.WindowsAcquirer{}).Acquire(context.Background(), identity)
		return err == nil && winner.Outcome == ownership.OutcomeAcquired
	}, 3*time.Second, 25*time.Millisecond)
	require.NoError(t, winner.Lease.Close())
	require.NoError(t, winner.Lease.Close())
}

func TestAbandonedMutexIsAcquiredAndDiagnosed(t *testing.T) {
	root := t.TempDir()
	cmd := startOwnerProcess(t, root)
	identity, err := ownership.CurrentIdentity(root)
	require.NoError(t, err)
	name, err := windows.UTF16PtrFromString(identity.MutexName)
	require.NoError(t, err)
	spectator, err := windows.OpenMutex(windows.SYNCHRONIZE|windows.MUTEX_MODIFY_STATE, false, name)
	require.NoError(t, err)
	defer windows.CloseHandle(spectator)

	require.NoError(t, cmd.Process.Kill())
	_, _ = cmd.Process.Wait()
	winner, err := (ownership.WindowsAcquirer{}).Acquire(context.Background(), identity)
	require.NoError(t, err)
	require.Equal(t, ownership.OutcomeAcquiredAbandoned, winner.Outcome)
	require.NoError(t, winner.Lease.Close())
}

func TestActivationNamedPipeAcceptsOnlyOneActivateFrame(t *testing.T) {
	identity, err := ownership.CurrentIdentity(t.TempDir())
	require.NoError(t, err)
	var called atomic.Int32
	server, err := ownership.StartActivationServer(identity, ownership.ActivationSinkFunc(func(ctx context.Context) error {
		_, hasDeadline := ctx.Deadline()
		require.True(t, hasDeadline)
		called.Add(1)
		return nil
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	response, err := (ownership.WindowsActivationClient{}).Activate(ctx, identity)
	require.NoError(t, err)
	require.Equal(t, ownership.ResultAccepted, response.Result)
	require.EqualValues(t, 1, called.Load())

	request, err := ownership.EncodeRequest()
	require.NoError(t, err)
	var handle windows.Handle
	require.Eventually(t, func() bool {
		name, nameErr := windows.UTF16PtrFromString(identity.PipeName)
		if nameErr != nil {
			return false
		}
		handle, err = windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	mode := uint32(windows.PIPE_READMODE_MESSAGE)
	require.NoError(t, windows.SetNamedPipeHandleState(handle, &mode, nil, nil))
	payload := append(request, request...)
	written, err := windows.Write(handle, payload)
	require.NoError(t, err)
	require.Equal(t, len(payload), written)
	buffer := make([]byte, 128)
	_, err = windows.Read(handle, buffer)
	require.Error(t, err)
	require.NoError(t, windows.CloseHandle(handle))
	require.EqualValues(t, 1, called.Load())
}
