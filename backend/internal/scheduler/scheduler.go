package scheduler

import (
	"context"
	"sync"
	"time"

	"gist/backend/internal/service"
	"gist/backend/pkg/logger"
)

type Scheduler struct {
	refreshService service.RefreshService
	interval       time.Duration
	writerLauncher service.WriterLauncher
	ctx            context.Context
	cancel         context.CancelFunc
	stopCh         chan struct{}
	stopOnce       sync.Once
	wg             sync.WaitGroup
}

func New(refreshService service.RefreshService, interval time.Duration, writerLauncher service.WriterLauncher) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		refreshService: refreshService,
		interval:       interval,
		writerLauncher: writerLauncher,
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

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.refresh()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) refresh() {
	ctx, cancel := context.WithTimeout(s.ctx, s.interval)
	defer cancel()

	done := make(chan struct{})
	err := s.writerLauncher.LaunchWriter(ctx, service.WriterBackground, func(writerCtx context.Context) {
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
	if err != nil {
		logger.Warn("scheduled refresh admission rejected", "module", "scheduler", "action", "refresh", "resource", "feed", "result", "rejected", "error", err)
		return
	}
	<-done
}
