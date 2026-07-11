package scheduler

import (
	"context"
	"sync"
	"time"

	"gist/backend/internal/service"
	"gist/backend/pkg/logger"
)

type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(time.Duration) bool
}

type Clock interface {
	NewTimer(time.Duration) Timer
}

type realClock struct{}

func (realClock) NewTimer(duration time.Duration) Timer {
	return realTimer{Timer: time.NewTimer(duration)}
}

type realTimer struct{ *time.Timer }

func (t realTimer) C() <-chan time.Time { return t.Timer.C }

type Scheduler struct {
	refreshService service.RefreshService
	interval       time.Duration
	writerLauncher service.WriterLauncher
	clock          Clock
	ctx            context.Context
	cancel         context.CancelFunc
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
}

func New(refreshService service.RefreshService, interval time.Duration, writerLauncher service.WriterLauncher, clocks ...Clock) *Scheduler {
	clock := Clock(realClock{})
	if len(clocks) != 0 && clocks[0] != nil {
		clock = clocks[0]
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		refreshService: refreshService,
		interval:       interval,
		writerLauncher: writerLauncher,
		clock:          clock,
		ctx:            ctx,
		cancel:         cancel,
		stopCh:         make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.run()
	logger.Info("scheduler started", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "ok", "interval_ms", s.interval.Milliseconds())
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.cancel()
		close(s.stopCh)
	})
	s.wg.Wait()
	logger.Info("scheduler stopped", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "ok")
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	// Run immediately on start.
	s.refresh()

	timer := s.clock.NewTimer(s.interval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C():
			s.refresh()
			timer.Reset(s.interval)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) refresh() {
	ctx, cancel := context.WithTimeout(s.ctx, s.interval)
	defer cancel()

	done := make(chan struct{})
	reservation, err := s.writerLauncher.ReserveWriter(ctx, service.WriterBackground)
	if err != nil {
		logger.Warn("scheduled refresh admission rejected", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "rejected", "error", err)
		return
	}
	reservation.Launch(func(writerCtx context.Context) {
		defer close(done)
		logger.Info("scheduled feed refresh started", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "ok")
		if err := s.refreshService.RefreshAll(writerCtx); err != nil {
			if writerCtx.Err() != nil {
				logger.Warn("scheduled refresh cancelled", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "cancelled")
				return
			}
			logger.Error("scheduled refresh failed", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "failed", "error", err)
			return
		}
		logger.Info("scheduled feed refresh completed", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "ok")
	})
	<-done
}
