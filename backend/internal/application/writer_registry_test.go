package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"gist/backend/internal/application"

	"github.com/stretchr/testify/require"
)

const testTimeout = time.Second

func TestWriterRegistryAdmissionAndExactlyOnceCompletion(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), application.WriterRequestBound)
	require.NoError(t, err)
	require.NotNil(t, token)

	var callers sync.WaitGroup
	for range 32 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			token.Complete()
		}()
	}
	callers.Wait()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))

	_, err = registry.Register(context.Background(), application.WriterRequestBound)
	require.ErrorIs(t, err, application.ErrWriterAdmissionClosed)
	_, err = registry.Register(context.Background(), application.WriterClass(255))
	require.ErrorIs(t, err, application.ErrInvalidWriterClass)
}

func TestWriterRegistryRequestCancellationIsLinked(t *testing.T) {
	type contextKey struct{}
	initiating, cancelInitiating := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-value"))
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(initiating, application.WriterRequestBound)
	require.NoError(t, err)
	require.Equal(t, "request-value", token.Context().Value(contextKey{}))
	defer token.Complete()

	cancelInitiating()
	requireContextCancelled(t, token.Context())
}

func TestWriterRegistryTaskCancellationIsLinked(t *testing.T) {
	initiating, cancelTask := context.WithCancel(context.Background())
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(initiating, application.WriterRequestBound)
	require.NoError(t, err)
	defer token.Complete()

	cancelTask()
	requireContextCancelled(t, token.Context())
}

func TestWriterRegistryRuntimeRootCancellationIsLinked(t *testing.T) {
	root, cancelRoot := context.WithCancel(context.Background())
	registry := application.NewWriterRegistry(root)
	token, err := registry.Register(context.Background(), application.WriterRequestBound)
	require.NoError(t, err)
	defer token.Complete()

	cancelRoot()
	requireContextCancelled(t, token.Context())
}

func TestWriterRegistryQuiesceCancelsBackgroundImmediately(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), application.WriterBackground)
	require.NoError(t, err)

	exited := make(chan struct{})
	go func() {
		defer close(exited)
		defer token.Complete()
		<-token.Context().Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))
	requireClosed(t, exited)
}

func TestWriterRegistryQuiesceGracefullyDrainsBoundWriter(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), application.WriterRequestBound)
	require.NoError(t, err)

	started := make(chan struct{})
	finish := make(chan struct{})
	go func() {
		close(started)
		<-finish
		token.Complete()
	}()
	<-started

	result := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	go func() { result <- registry.Quiesce(ctx) }()

	select {
	case <-token.Context().Done():
		t.Fatal("bound writer was cancelled during graceful drain")
	case <-time.After(20 * time.Millisecond):
	}
	close(finish)
	require.NoError(t, <-result)
}

func TestWriterRegistryDeadlineForceCancelsAndLaterCallContinuesWaiting(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), application.WriterRequestBound)
	require.NoError(t, err)

	releaseCompletion := make(chan struct{})
	go func() {
		<-token.Context().Done()
		<-releaseCompletion
		token.Complete()
	}()

	firstCtx, firstCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer firstCancel()
	err = registry.Quiesce(firstCtx)
	require.ErrorIs(t, err, application.ErrWriterQuiesceDeadline)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	requireContextCancelled(t, token.Context())

	secondResult := make(chan error, 1)
	secondCtx, secondCancel := context.WithTimeout(context.Background(), testTimeout)
	defer secondCancel()
	go func() { secondResult <- registry.Quiesce(secondCtx) }()
	select {
	case err := <-secondResult:
		t.Fatalf("retry returned before writer completion: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(releaseCompletion)
	require.NoError(t, <-secondResult)
}

func TestWriterRegistryConcurrentRegisterCompleteAndQuiesce(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	start := make(chan struct{})
	var workers sync.WaitGroup
	unexpectedErrors := make(chan error, 200)

	for i := range 200 {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			class := application.WriterRequestBound
			if i%2 == 0 {
				class = application.WriterBackground
			}
			token, err := registry.Register(context.Background(), class)
			if errors.Is(err, application.ErrWriterAdmissionClosed) {
				return
			}
			if err != nil {
				unexpectedErrors <- err
				return
			}
			if class == application.WriterBackground {
				select {
				case <-token.Context().Done():
				case <-time.After(time.Millisecond):
				}
			}
			token.Complete()
			token.Complete()
		}(i)
	}

	close(start)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))
	workers.Wait()
	close(unexpectedErrors)
	for err := range unexpectedErrors {
		require.NoError(t, err)
	}

	_, err := registry.Register(context.Background(), application.WriterBackground)
	require.ErrorIs(t, err, application.ErrWriterAdmissionClosed)
}

func requireContextCancelled(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	case <-time.After(testTimeout):
		t.Fatal("context was not cancelled")
	}
}

func requireClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(testTimeout):
		t.Fatal("channel was not closed")
	}
}
