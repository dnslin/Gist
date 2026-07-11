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

	pprofServer := startPprofServer(cfg.PprofAddr)
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		<-sigCh
		logger.Info("shutting down...")

		overallCtx, cancelOverall := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelOverall()
		drainCtx, cancelDrain := context.WithTimeout(overallCtx, 8*time.Second)
		defer cancelDrain()

		var wg sync.WaitGroup
		errs := make(chan error, 3)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if shutdownErr := runtime.Router.Shutdown(drainCtx); shutdownErr != nil {
				errs <- shutdownErr
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if quiesceErr := runtime.Quiesce(drainCtx); quiesceErr != nil {
				errs <- quiesceErr
			}
		}()
		if pprofServer != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if shutdownErr := pprofServer.Shutdown(drainCtx); shutdownErr != nil {
					errs <- shutdownErr
				}
			}()
		}
		wg.Wait()
		close(errs)
		for shutdownErr := range errs {
			logger.Warn("graceful drain", "error", shutdownErr)
		}
		if closeErr := runtime.Close(overallCtx); closeErr != nil {
			logger.Error("application runtime close", "error", closeErr)
		}
	}()

	if err := runtime.Router.Start(cfg.Addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("start server", "error", err)
		_ = runtime.Close(context.Background())
		os.Exit(1)
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
