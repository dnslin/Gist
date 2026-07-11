package scheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/scheduler"
	"gist/backend/internal/service"
)

type fakeClock struct {
	created chan *fakeTimer
}

func newFakeClock() *fakeClock { return &fakeClock{created: make(chan *fakeTimer, 1)} }

func (c *fakeClock) NewTimer(time.Duration) scheduler.Timer {
	timer := &fakeTimer{ticks: make(chan time.Time, 1)}
	c.created <- timer
	return timer
}

type fakeTimer struct {
	ticks chan time.Time
}

func (t *fakeTimer) C() <-chan time.Time      { return t.ticks }
func (t *fakeTimer) Stop() bool               { return true }
func (t *fakeTimer) Reset(time.Duration) bool { return true }
func (t *fakeTimer) fire()                    { t.ticks <- time.Time{} }

type recordingRefresh struct {
	calls chan context.Context
	run   func(context.Context) error
}

func (r *recordingRefresh) RefreshAll(ctx context.Context) error {
	r.calls <- ctx
	if r.run != nil {
		return r.run(ctx)
	}
	return nil
}

func (r *recordingRefresh) RefreshFeed(context.Context, int64) error    { return nil }
func (r *recordingRefresh) RefreshFeeds(context.Context, []int64) error { return nil }
func (r *recordingRefresh) IsRefreshing() bool                          { return false }
func (r *recordingRefresh) GetRefreshStatus() service.RefreshStatus     { return service.RefreshStatus{} }

type recordingLauncher struct {
	mu      sync.Mutex
	classes []service.WriterClass
}

func (l *recordingLauncher) ReserveWriter(ctx context.Context, class service.WriterClass) (service.WriterReservation, error) {
	l.mu.Lock()
	l.classes = append(l.classes, class)
	l.mu.Unlock()
	return &recordingReservation{ctx: ctx}, nil
}

type recordingReservation struct{ ctx context.Context }

func (r *recordingReservation) Context() context.Context         { return r.ctx }
func (r *recordingReservation) Publish()                         {}
func (r *recordingReservation) Launch(run func(context.Context)) { go run(r.ctx) }
func (r *recordingReservation) Release()                         {}

func (l *recordingLauncher) snapshot() []service.WriterClass {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]service.WriterClass(nil), l.classes...)
}

func TestSchedulerRunsImmediatelyAndOnControlledPeriodicTicks(t *testing.T) {
	clock := newFakeClock()
	refresh := &recordingRefresh{calls: make(chan context.Context, 3)}
	launcher := &recordingLauncher{}
	s := scheduler.New(refresh, time.Minute, launcher, clock)
	s.Start()
	<-refresh.calls
	timer := <-clock.created
	timer.fire()
	<-refresh.calls
	timer.fire()
	<-refresh.calls
	s.Stop()
	require.Equal(t, []service.WriterClass{
		service.WriterBackground,
		service.WriterBackground,
		service.WriterBackground,
	}, launcher.snapshot())
}

func TestSchedulerStopCancelsAndWaitsForCurrentRefresh(t *testing.T) {
	clock := newFakeClock()
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	refresh := &recordingRefresh{
		calls: make(chan context.Context, 1),
		run: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			<-release
			close(finished)
			return ctx.Err()
		},
	}
	s := scheduler.New(refresh, time.Minute, &recordingLauncher{}, clock)
	s.Start()
	<-started

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
		t.Fatal("Stop returned while the current refresh was still running")
	case <-refresh.calls:
	}
	close(release)
	<-stopped
	<-finished
}
