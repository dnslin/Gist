package application_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/application"
	"gist/backend/internal/service"
	"gist/backend/pkg/snowflake"
)

func TestNewRuntimeBuildsRouterAndServices(t *testing.T) {
	runtime := newTestRuntime(t)
	t.Cleanup(func() { require.NoError(t, runtime.Close(context.Background())) })

	require.NotNil(t, runtime.Router)
	require.NotNil(t, runtime.Auth)
	require.NotNil(t, runtime.AI)
	require.NotNil(t, runtime.OPML)
	require.NotNil(t, runtime.ImportTasks)
	require.NotNil(t, runtime.Writers)

	request := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	response := httptest.NewRecorder()
	runtime.Router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
}

func TestNewRuntimeRejectsInvalidOptionsBeforeOpeningDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "must-not-exist", "gist.db")

	runtime, err := application.NewRuntime(context.Background(), application.RuntimeOptions{DBPath: dbPath})
	require.ErrorIs(t, err, application.ErrMissingIDGenerator)
	require.Nil(t, runtime)
	_, statErr := os.Stat(filepath.Dir(dbPath))
	require.ErrorIs(t, statErr, os.ErrNotExist)

	generator, err := snowflake.NewGenerator(1)
	require.NoError(t, err)
	runtime, err = application.NewRuntime(context.Background(), application.RuntimeOptions{
		DBPath:            dbPath,
		IDGenerator:       generator,
		SchedulerInterval: -time.Second,
	})
	require.ErrorIs(t, err, application.ErrInvalidInterval)
	require.Nil(t, runtime)
	_, statErr = os.Stat(filepath.Dir(dbPath))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestNewRuntimeDatabaseFailureReturnsNoRuntime(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(parentFile, []byte("file"), 0o600))
	generator, err := snowflake.NewGenerator(1)
	require.NoError(t, err)

	runtime, err := application.NewRuntime(context.Background(), application.RuntimeOptions{
		DataDir:     t.TempDir(),
		DBPath:      filepath.Join(parentFile, "gist.db"),
		IDGenerator: generator,
	})
	require.Error(t, err)
	require.Nil(t, runtime)
}

func TestRuntimeActivatesAndStopsScheduler(t *testing.T) {
	generator, err := snowflake.NewGenerator(1)
	require.NoError(t, err)
	runtime, err := application.NewRuntime(context.Background(), application.RuntimeOptions{
		DataDir:           filepath.Join(t.TempDir(), "data"),
		DBPath:            filepath.Join(t.TempDir(), "gist.db"),
		IDGenerator:       generator,
		StartScheduler:    true,
		SchedulerInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, runtime.Close(closeCtx))
}

func TestRuntimeCloseKeepsDatabaseOpenUntilWritersAreQuiet(t *testing.T) {
	runtime := newTestRuntime(t)

	token, err := runtime.Writers.Register(context.Background(), service.WriterBackground)
	require.NoError(t, err)
	writerCheckedDB := make(chan error, 1)
	go func() {
		<-token.Context().Done()
		_, checkErr := runtime.Auth.CheckUserExists(context.Background())
		writerCheckedDB <- checkErr
		token.Complete()
	}()

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, runtime.Close(closeCtx))
	require.NoError(t, <-writerCheckedDB, "database must remain usable until the final writer completes")

	_, err = runtime.Auth.CheckUserExists(context.Background())
	require.Error(t, err, "database must be closed after a successful Runtime.Close")
	require.NoError(t, runtime.Writers.Quiesce(context.Background()))
	_, err = runtime.Writers.Register(context.Background(), service.WriterBackground)
	require.ErrorIs(t, err, application.ErrWriterAdmissionClosed)
	require.NoError(t, runtime.Close(context.Background()))
}

func TestRuntimeCloseDeadlineCanBeRetried(t *testing.T) {
	runtime := newTestRuntime(t)

	token, err := runtime.Writers.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)

	closeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = runtime.Close(closeCtx)
	require.ErrorIs(t, err, application.ErrWriterQuiesceDeadline)
	require.ErrorIs(t, token.Context().Err(), context.Canceled)

	token.Complete()
	require.NoError(t, runtime.Close(context.Background()))
	cancelledCtx, cancelRetry := context.WithCancel(context.Background())
	cancelRetry()
	require.NoError(t, runtime.Quiesce(cancelledCtx))
	require.NoError(t, runtime.Close(cancelledCtx))
}

func TestRuntimeConcurrentQuiesceAndCloseAreIdempotent(t *testing.T) {
	runtime := newTestRuntime(t)
	token, err := runtime.Writers.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)

	const callers = 16
	start := make(chan struct{})
	results := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			results <- runtime.Quiesce(context.Background())
		}()
	}
	close(start)
	token.Complete()
	for range callers {
		require.NoError(t, <-results)
	}

	start = make(chan struct{})
	for range callers {
		go func() {
			<-start
			results <- runtime.Close(context.Background())
		}()
	}
	close(start)
	for range callers {
		require.NoError(t, <-results)
	}
	require.NoError(t, runtime.Quiesce(context.Background()))
	require.NoError(t, runtime.Close(context.Background()))
}

func TestNilRuntimeCloseAndQuiesceAreSafe(t *testing.T) {
	var runtime *application.Runtime
	require.NoError(t, runtime.Quiesce(context.Background()))
	require.NoError(t, runtime.Close(context.Background()))
}

func newTestRuntime(t *testing.T) *application.Runtime {
	t.Helper()
	generator, err := snowflake.NewGenerator(1)
	require.NoError(t, err)
	runtime, err := application.NewRuntime(context.Background(), application.RuntimeOptions{
		DataDir:     filepath.Join(t.TempDir(), "data"),
		DBPath:      filepath.Join(t.TempDir(), "gist.db"),
		IDGenerator: generator,
	})
	require.NoError(t, err)
	return runtime
}
