package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"gist/backend/internal/application"
	"gist/backend/internal/config"
	"gist/backend/pkg/logger"
	"gist/backend/pkg/snowflake"
)

// @title Gist API
// @version 1.2.0
// @description This is a modern RSS reader API.
// @BasePath /api
func main() {
	cfg := config.Load()
	logger.Init(logger.ParseLevel(cfg.LogLevel))

	generatorOwner := snowflake.NewBootstrapOwner()
	idGenerator, err := generatorOwner.Init(1)
	if err != nil {
		logger.Error("init snowflake", "error", err)
		os.Exit(1)
	}

	runtime, err := application.NewRuntime(context.Background(), application.RuntimeOptions{
		DataDir:           cfg.DataDir,
		DBPath:            cfg.DBPath,
		StaticDir:         cfg.StaticDir,
		EnableSwagger:     cfg.EnableSwagger,
		SchedulerInterval: 15 * time.Minute,
		StartScheduler:    true,
		IDGenerator:       idGenerator,
	})
	if err != nil {
		logger.Error("create application runtime", "error", err)
		os.Exit(1)
	}

	trackedRouter := newTrackedHTTPServer(runtime.Router)
	pprofServer := startPprofServer(cfg.PprofAddr)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	shutdown := shutdownRunner{
		http:           trackedRouter,
		pprof:          pprofServer,
		runtime:        runtime,
		overallTimeout: 10 * time.Second,
		drainTimeout:   8 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		if shutdownErr := runOnSignal(sigCh, func() error {
			logger.Info("shutting down...")
			return shutdown.Run(context.Background())
		}); shutdownErr != nil {
			logger.Error("server shutdown", "error", shutdownErr)
		}
	}()

	if err := startListener(trackedRouter, cfg.Addr); err != nil {
		logger.Error("start server", "error", err)
		_ = runtime.Close(context.Background())
		return
	}
	<-shutdownDone
	logger.Info("server stopped")
}

func startPprofServer(addr string) *http.Server {
	if addr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("pprof server started", "module", "server", "action", "start", "resource", "pprof", "result", "ok", "addr", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("pprof server failed", "module", "server", "action", "start", "resource", "pprof", "result", "failed", "addr", addr, "error", err)
		}
	}()

	return server
}

func runOnSignal(signals <-chan os.Signal, shutdown func() error) error {
	<-signals
	return shutdown()
}

type listenerServer interface {
	Start(string) error
}

type shutdownHTTPServer interface {
	Shutdown(context.Context) error
	Close() error
	Wait(context.Context) error
}

type shutdownPprofServer interface {
	Shutdown(context.Context) error
	Close() error
}

type shutdownRuntime interface {
	Quiesce(context.Context) error
	Close(context.Context) error
}

type shutdownRunner struct {
	http           shutdownHTTPServer
	pprof          shutdownPprofServer
	runtime        shutdownRuntime
	overallTimeout time.Duration
	drainTimeout   time.Duration
}

func (r shutdownRunner) Run(parent context.Context) error {
	overallCtx, cancelOverall := context.WithTimeout(parent, r.overallTimeout)
	defer cancelOverall()
	drainCtx, cancelDrain := context.WithTimeout(overallCtx, r.drainTimeout)
	defer cancelDrain()

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	runDrain := func(shutdown func(context.Context) error) {
		defer wg.Done()
		if err := shutdown(drainCtx); err != nil {
			errs <- err
		}
	}
	wg.Add(2)
	go runDrain(r.http.Shutdown)
	go runDrain(r.runtime.Quiesce)
	if r.pprof != nil {
		wg.Add(1)
		go runDrain(r.pprof.Shutdown)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		logger.Warn("graceful drain", "error", err)
	}
	if drainCtx.Err() != nil {
		if err := r.http.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		if r.pprof != nil {
			if err := r.pprof.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		}
		if err := r.http.Wait(overallCtx); err != nil {
			return err
		}
	}
	return r.runtime.Close(overallCtx)
}

func startListener(server listenerServer, addr string) error {
	err := server.Start(addr)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type trackedHTTPServer struct {
	server  *echo.Echo
	tracker requestTracker
}

func newTrackedHTTPServer(server *echo.Echo) *trackedHTTPServer {
	tracked := &trackedHTTPServer{server: server}
	server.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			done := tracked.tracker.Begin()
			defer done()
			return next(c)
		}
	})
	return tracked
}

func (s *trackedHTTPServer) Start(addr string) error { return s.server.Start(addr) }

func (s *trackedHTTPServer) Shutdown(ctx context.Context) error { return s.server.Shutdown(ctx) }

func (s *trackedHTTPServer) Close() error { return s.server.Close() }

func (s *trackedHTTPServer) Wait(ctx context.Context) error { return s.tracker.Wait(ctx) }

type requestTracker struct {
	mu     sync.Mutex
	active int
	idle   chan struct{}
}

func (t *requestTracker) Begin() func() {
	t.mu.Lock()
	if t.active == 0 {
		t.idle = make(chan struct{})
	}
	t.active++
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		t.active--
		if t.active == 0 {
			close(t.idle)
		}
		t.mu.Unlock()
	}
}

func (t *requestTracker) Wait(ctx context.Context) error {
	t.mu.Lock()
	if t.active == 0 {
		t.mu.Unlock()
		return nil
	}
	idle := t.idle
	t.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
