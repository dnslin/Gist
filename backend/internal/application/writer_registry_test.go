package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/application"
	"gist/backend/internal/service"
)

const testTimeout = time.Second

func TestWriterRegistryAdmissionAndExactlyOnceCompletion(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)

	var callers sync.WaitGroup
	for range 32 {
		callers.Add(1)
		go func() { defer callers.Done(); token.Complete() }()
	}
	callers.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))
	_, err = registry.Register(context.Background(), service.WriterRequestBound)
	require.ErrorIs(t, err, application.ErrWriterAdmissionClosed)
	_, err = registry.Register(context.Background(), service.WriterClass(255))
	require.ErrorIs(t, err, application.ErrInvalidWriterClass)
}

func TestWriterRegistryInitiatingCancellationBeforePublicationIsLinked(t *testing.T) {
	type contextKey struct{}
	initiating, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request-value"))
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(initiating, service.WriterRequestBound)
	require.NoError(t, err)
	defer token.Complete()
	require.Equal(t, "request-value", token.Context().Value(contextKey{}))
	cancel()
	requireContextCancelled(t, token.Context())
}

func TestWriterRegistryPublishedTaskSurvivesInitiatingRequestCompletion(t *testing.T) {
	initiating, cancel := context.WithCancel(context.Background())
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(initiating, service.WriterRequestBound)
	require.NoError(t, err)
	defer token.Complete()
	token.Publish()
	cancel()
	select {
	case <-token.Context().Done():
		t.Fatal("published task was cancelled with completed HTTP request")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestWriterRegistryRuntimeRootCancellationIsLinkedAfterPublication(t *testing.T) {
	root, cancelRoot := context.WithCancel(context.Background())
	registry := application.NewWriterRegistry(root)
	token, err := registry.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)
	defer token.Complete()
	token.Publish()
	cancelRoot()
	requireContextCancelled(t, token.Context())
}

func TestWriterRegistryQuiesceCancelsBackgroundImmediately(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), service.WriterBackground)
	require.NoError(t, err)
	exited := make(chan struct{})
	go func() { defer close(exited); defer token.Complete(); <-token.Context().Done() }()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))
	requireClosed(t, exited)
}

func TestWriterRegistryQuiesceGracefullyDrainsBoundWriter(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	token, err := registry.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)
	finish := make(chan struct{})
	go func() { <-finish; token.Complete() }()
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
	token, err := registry.Register(context.Background(), service.WriterRequestBound)
	require.NoError(t, err)
	release := make(chan struct{})
	go func() { <-token.Context().Done(); <-release; token.Complete() }()
	firstCtx, firstCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer firstCancel()
	err = registry.Quiesce(firstCtx)
	require.ErrorIs(t, err, application.ErrWriterQuiesceDeadline)
	requireContextCancelled(t, token.Context())
	second := make(chan error, 1)
	secondCtx, secondCancel := context.WithTimeout(context.Background(), testTimeout)
	defer secondCancel()
	go func() { second <- registry.Quiesce(secondCtx) }()
	select {
	case err := <-second:
		t.Fatalf("retry returned before writer completion: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-second)
}

func TestWriterRegistryConcurrentRegisterCompleteAndQuiesce(t *testing.T) {
	registry := application.NewWriterRegistry(context.Background())
	start := make(chan struct{})
	var workers sync.WaitGroup
	unexpected := make(chan error, 200)
	for i := range 200 {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			class := service.WriterRequestBound
			if i%2 == 0 {
				class = service.WriterBackground
			}
			token, err := registry.Register(context.Background(), class)
			if errors.Is(err, application.ErrWriterAdmissionClosed) {
				return
			}
			if err != nil {
				unexpected <- err
				return
			}
			token.Complete()
		}(i)
	}
	close(start)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	require.NoError(t, registry.Quiesce(ctx))
	workers.Wait()
	close(unexpected)
	for err := range unexpected {
		require.NoError(t, err)
	}
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
