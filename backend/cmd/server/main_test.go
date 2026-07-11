package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockingHTTPServer struct {
	closeCalled chan struct{}
	handlerDone chan struct{}
	closeOnce   sync.Once
}

func newBlockingHTTPServer() *blockingHTTPServer {
	return &blockingHTTPServer{
		closeCalled: make(chan struct{}),
		handlerDone: make(chan struct{}),
	}
}

func (s *blockingHTTPServer) Shutdown(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *blockingHTTPServer) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCalled)
		close(s.handlerDone)
	})
	return nil
}

func (s *blockingHTTPServer) Wait(ctx context.Context) error {
	select {
	case <-s.handlerDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type recordingRuntime struct {
	quiesceCalls int
	closeCalls   int
	handlerDone  <-chan struct{}
	closeErr     error
}

func (r *recordingRuntime) Quiesce(ctx context.Context) error {
	r.quiesceCalls++
	<-ctx.Done()
	return ctx.Err()
}

func (r *recordingRuntime) Close(context.Context) error {
	r.closeCalls++
	select {
	case <-r.handlerDone:
		return r.closeErr
	default:
		return errors.New("runtime close began before HTTP handler exited")
	}
}

type recordingPprofServer struct {
	shutdownCalls int
	closeCalls    int
}

func (s *recordingPprofServer) Shutdown(context.Context) error {
	s.shutdownCalls++
	return nil
}

func (s *recordingPprofServer) Close() error {
	s.closeCalls++
	return nil
}

func TestShutdownRunnerForcesHTTPHandlersBeforeRuntimeClose(t *testing.T) {
	httpServer := newBlockingHTTPServer()
	runtime := &recordingRuntime{handlerDone: httpServer.handlerDone}
	runner := shutdownRunner{
		http:           httpServer,
		runtime:        runtime,
		overallTimeout: 100 * time.Millisecond,
		drainTimeout:   20 * time.Millisecond,
	}

	err := runner.Run(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, runtime.closeCalls)
	requireClosed(t, httpServer.closeCalled, "HTTP server was not force-closed after drain timeout")
}

func TestSignalsUseSameShutdownPathWithPprofOnAndOff(t *testing.T) {
	testCases := []struct {
		name   string
		signal os.Signal
		pprof  bool
	}{
		{name: "SIGINT pprof off", signal: syscall.SIGINT},
		{name: "SIGTERM pprof off", signal: syscall.SIGTERM},
		{name: "SIGINT pprof on", signal: syscall.SIGINT, pprof: true},
		{name: "SIGTERM pprof on", signal: syscall.SIGTERM, pprof: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpServer := newBlockingHTTPServer()
			runtime := &recordingRuntime{handlerDone: httpServer.handlerDone}
			var pprof *recordingPprofServer
			var pprofServer shutdownPprofServer
			if tc.pprof {
				pprof = &recordingPprofServer{}
				pprofServer = pprof
			}
			runner := shutdownRunner{
				http:           httpServer,
				pprof:          pprofServer,
				runtime:        runtime,
				overallTimeout: 100 * time.Millisecond,
				drainTimeout:   20 * time.Millisecond,
			}
			signals := make(chan os.Signal, 1)
			signals <- tc.signal

			err := runOnSignal(signals, func() error { return runner.Run(context.Background()) })

			require.NoError(t, err)
			require.Equal(t, 1, runtime.quiesceCalls)
			require.Equal(t, 1, runtime.closeCalls)
			if tc.pprof {
				require.Equal(t, 1, pprof.shutdownCalls)
				require.Equal(t, 1, pprof.closeCalls)
			}
		})
	}
}

type stubListener struct {
	err error
}

func (s stubListener) Start(string) error { return s.err }

func TestStartListenerReturnsBindFailure(t *testing.T) {
	bindErr := errors.New("bind failed")
	require.ErrorIs(t, startListener(stubListener{err: bindErr}, ":8080"), bindErr)
	require.NoError(t, startListener(stubListener{err: http.ErrServerClosed}, ":8080"))
}

func TestServerDependenciesExcludeDesktopAndWindowsHosts(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	dependencies := strings.Split(string(output), "\n")
	for _, dependency := range dependencies {
		require.NotContains(t, dependency, "github.com/wailsapp/")
		require.NotContains(t, dependency, "gist/backend/internal/desktop")
		require.NotEqual(t, "golang.org/x/sys/windows", dependency)
	}
}

func requireClosed(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	default:
		require.Fail(t, message)
	}
}
