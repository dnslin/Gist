package scheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gist/backend/internal/scheduler"
	"gist/backend/internal/service"
	"gist/backend/internal/service/mock"
)

type recordingLauncher struct {
	mu      sync.Mutex
	classes []service.WriterClass
}

func (l *recordingLauncher) LaunchWriter(ctx context.Context, class service.WriterClass, run func(context.Context)) error {
	l.mu.Lock()
	l.classes = append(l.classes, class)
	l.mu.Unlock()
	go run(ctx)
	return nil
}

func (l *recordingLauncher) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.classes)
}

func TestScheduler_ImmediateAndPeriodicRefreshRegistered(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRefresh := mock.NewMockRefreshService(ctrl)
	calls := make(chan struct{}, 4)
	mockRefresh.EXPECT().RefreshAll(gomock.Any()).DoAndReturn(func(context.Context) error {
		calls <- struct{}{}
		return nil
	}).AnyTimes()

	launcher := &recordingLauncher{}
	s := scheduler.New(mockRefresh, 20*time.Millisecond, launcher)
	s.Start()
	defer s.Stop()

	for range 2 {
		select {
		case <-calls:
		case <-time.After(300 * time.Millisecond):
			t.Fatal("expected immediate and periodic refresh")
		}
	}

	require.GreaterOrEqual(t, launcher.count(), 2)
	launcher.mu.Lock()
	defer launcher.mu.Unlock()
	for _, class := range launcher.classes {
		require.Equal(t, service.WriterBackground, class)
	}
}

func TestScheduler_StopCancelsAndWaitsForRefresh(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRefresh := mock.NewMockRefreshService(ctrl)
	started := make(chan struct{})
	finished := make(chan struct{})
	mockRefresh.EXPECT().RefreshAll(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(finished)
		return ctx.Err()
	})

	s := scheduler.New(mockRefresh, time.Hour, &recordingLauncher{})
	s.Start()
	<-started
	s.Stop()

	select {
	case <-finished:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("Stop returned before refresh observed cancellation")
	}
}
