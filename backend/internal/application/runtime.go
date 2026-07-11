package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"gist/backend/internal/db"
	"gist/backend/internal/handler"
	transport "gist/backend/internal/http"
	"gist/backend/internal/repository"
	"gist/backend/internal/scheduler"
	"gist/backend/internal/service"
	"gist/backend/internal/service/ai"
	"gist/backend/internal/service/anubis"
	"gist/backend/pkg/logger"
	"gist/backend/pkg/network"
	"gist/backend/pkg/snowflake"
)

const defaultSchedulerInterval = 15 * time.Minute

var (
	ErrMissingDBPath      = errors.New("runtime database path is required")
	ErrMissingIDGenerator = errors.New("runtime snowflake generator is required")
	ErrInvalidInterval    = errors.New("runtime scheduler interval must be positive")
)

// RuntimeOptions contains host-provided values only. Runtime never reads process
// environment or starts host listeners.
type RuntimeOptions struct {
	DataDir           string
	DBPath            string
	StaticDir         string
	EnableSwagger     bool
	SchedulerInterval time.Duration
	StartScheduler    bool
	IDGenerator       snowflake.Generator
}

type runtimeState uint8

const (
	runtimeOpen runtimeState = iota + 1
	runtimeQuiescing
	runtimeQuiesced
	runtimeClosing
	runtimeClosed
)

// Runtime is the platform-independent application composition root.
type Runtime struct {
	Router      *echo.Echo
	Auth        service.AuthService
	AI          service.AIService
	OPML        service.OPMLService
	ImportTasks service.ImportTaskService
	Writers     *WriterRegistry

	db          *sql.DB
	scheduler   *scheduler.Scheduler
	readability service.ReadabilityService
	proxy       service.ProxyService
	rootCancel  context.CancelFunc

	mu                sync.Mutex
	state             runtimeState
	stopSchedulerOnce sync.Once
	schedulerDone     chan struct{}
	closeOnce         sync.Once
	closeDone         chan struct{}
	closeErr          error
}
type runtimeBuildStage string

const (
	buildDatabase     runtimeBuildStage = "database"
	buildRepositories runtimeBuildStage = "repositories"
	buildServices     runtimeBuildStage = "services"
	buildRouter       runtimeBuildStage = "router"
	buildScheduler    runtimeBuildStage = "scheduler"
	buildBackfill     runtimeBuildStage = "backfill"
)

type runtimeBuilder struct {
	checkpoint       func(runtimeBuildStage) error
	cleanupObserved  func(string)
	activateObserved func(string)
}

func (b runtimeBuilder) reached(stage runtimeBuildStage) error {
	if b.checkpoint == nil {
		return nil
	}
	return b.checkpoint(stage)
}

func (b runtimeBuilder) cleaned(resource string) {
	if b.cleanupObserved != nil {
		b.cleanupObserved(resource)
	}
}

func (b runtimeBuilder) activated(name string) {
	if b.activateObserved != nil {
		b.activateObserved(name)
	}
}

// NewRuntime builds the complete application graph before activating any
// asynchronous writer. A build failure therefore cannot leave a worker behind.
func NewRuntime(ctx context.Context, options RuntimeOptions) (*Runtime, error) {
	return (runtimeBuilder{}).Build(ctx, options)
}

func (b runtimeBuilder) Build(ctx context.Context, options RuntimeOptions) (*Runtime, error) {
	if options.IDGenerator == nil {
		return nil, ErrMissingIDGenerator
	}
	if options.DBPath == "" {
		return nil, ErrMissingDBPath
	}
	interval := options.SchedulerInterval
	if interval == 0 {
		interval = defaultSchedulerInterval
	}
	if interval < 0 {
		return nil, ErrInvalidInterval
	}
	if ctx == nil {
		ctx = context.Background()
	}

	rootCtx, rootCancel := context.WithCancel(ctx)
	cleanups := []func(){func() { rootCancel(); b.cleaned("root") }}
	cleanupBuild := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	fail := func(stage runtimeBuildStage, err error) (*Runtime, error) {
		cleanupBuild()
		return nil, fmt.Errorf("build runtime %s: %w", stage, err)
	}

	r := &Runtime{
		rootCancel:    rootCancel,
		state:         runtimeOpen,
		schedulerDone: make(chan struct{}),
		closeDone:     make(chan struct{}),
	}
	r.Writers = NewWriterRegistry(rootCtx)
	launcher := writerRegistryLauncher{registry: r.Writers}

	dbConn, err := db.Open(options.DBPath)
	if err != nil {
		rootCancel()
		return nil, fmt.Errorf("open runtime database: %w", err)
	}
	r.db = dbConn
	cleanups = append(cleanups, func() { _ = dbConn.Close(); b.cleaned("database") })
	if err := b.reached(buildDatabase); err != nil {
		return fail(buildDatabase, err)
	}

	folderRepo := repository.NewFolderRepository(dbConn, options.IDGenerator)
	feedRepo := repository.NewFeedRepository(dbConn, options.IDGenerator)
	entryRepo := repository.NewEntryRepository(dbConn, options.IDGenerator)
	settingsRepo := repository.NewSettingsRepository(dbConn)
	aiSummaryRepo := repository.NewAISummaryRepository(dbConn, options.IDGenerator)
	aiTranslationRepo := repository.NewAITranslationRepository(dbConn, options.IDGenerator)
	aiListTranslationRepo := repository.NewAIListTranslationRepository(dbConn, options.IDGenerator)
	domainRateLimitRepo := repository.NewDomainRateLimitRepository(dbConn, options.IDGenerator)
	if err := b.reached(buildRepositories); err != nil {
		return fail(buildRepositories, err)
	}

	initialRateLimit := ai.DefaultRateLimit
	if setting, getErr := settingsRepo.Get(rootCtx, "ai.rate_limit"); getErr == nil && setting != nil {
		var value int
		if _, scanErr := fmt.Sscanf(setting.Value, "%d", &value); scanErr == nil && value > 0 {
			initialRateLimit = value
		}
	}
	rateLimiter := ai.NewRateLimiter(initialRateLimit)
	settingsService := service.NewSettingsService(settingsRepo, rateLimiter)
	clientFactory := network.NewClientFactory(settingsService, settingsService)
	anubisStore := anubis.NewStore(settingsRepo)
	anubisSolver := anubis.NewSolver(clientFactory, anubisStore)
	iconService := service.NewIconService(options.DataDir, feedRepo, clientFactory, anubisSolver)
	folderService := service.NewFolderService(folderRepo, feedRepo)
	feedService := service.NewFeedService(feedRepo, folderRepo, entryRepo, iconService, settingsService, clientFactory, anubisSolver)
	entryService := service.NewEntryService(entryRepo, feedRepo, folderRepo)
	r.readability = service.NewReadabilityService(entryRepo, clientFactory, anubisSolver)
	domainRateLimitService := service.NewDomainRateLimitService(domainRateLimitRepo)
	refreshService := service.NewRefreshService(feedRepo, entryRepo, settingsService, iconService, clientFactory, anubisSolver, domainRateLimitService)
	r.OPML = service.NewOPMLService(folderService, feedService, refreshService, iconService, folderRepo, feedRepo)
	r.proxy = service.NewProxyService(clientFactory, anubisSolver)
	r.AI = service.NewAIServiceWithFeedContext(aiSummaryRepo, aiTranslationRepo, aiListTranslationRepo, settingsRepo, rateLimiter, entryRepo, feedRepo, launcher)
	r.Auth = service.NewAuthService(settingsRepo)
	r.ImportTasks = service.NewImportTaskService()
	cleanups = append(cleanups,
		func() { r.readability.Close(); b.cleaned("readability") },
		func() { r.proxy.Close(); b.cleaned("proxy") },
	)
	if err := b.reached(buildServices); err != nil {
		return fail(buildServices, err)
	}

	folderHandler := handler.NewFolderHandler(folderService)
	feedHandler := handler.NewFeedHandler(feedService, refreshService)
	entryHandler := handler.NewEntryHandler(entryService, r.readability)
	opmlHandler := handler.NewOPMLHandler(r.OPML, r.ImportTasks, launcher)
	iconHandler := handler.NewIconHandler(iconService)
	proxyHandler := handler.NewProxyHandler(r.proxy)
	settingsHandler := handler.NewSettingsHandler(settingsService, clientFactory)
	aiHandler := handler.NewAIHandler(r.AI)
	authHandler := handler.NewAuthHandler(r.Auth)
	domainRateLimitHandler := handler.NewDomainRateLimitHandler(domainRateLimitService)
	r.Router = transport.NewRouter(folderHandler, feedHandler, entryHandler, opmlHandler, iconHandler, proxyHandler, settingsHandler, aiHandler, authHandler, domainRateLimitHandler, r.Auth, options.StaticDir, options.EnableSwagger)
	if err := b.reached(buildRouter); err != nil {
		return fail(buildRouter, err)
	}

	if options.StartScheduler {
		r.scheduler = scheduler.New(refreshService, interval, launcher)
	}
	if r.scheduler == nil {
		close(r.schedulerDone)
	}
	if err := b.reached(buildScheduler); err != nil {
		return fail(buildScheduler, err)
	}

	backfillReservation, err := launcher.ReserveWriter(rootCtx, service.WriterBackground)
	if err != nil {
		return fail(buildBackfill, err)
	}
	cleanups = append(cleanups, func() { backfillReservation.Release(); b.cleaned("backfill") })
	if err := b.reached(buildBackfill); err != nil {
		return fail(buildBackfill, err)
	}

	backfillReservation.Publish()
	backfillReservation.Launch(func(writerCtx context.Context) {
		if backfillErr := iconService.BackfillIcons(writerCtx); backfillErr != nil && !errors.Is(backfillErr, context.Canceled) {
			logger.Warn("backfill icons", "module", "service", "action", "backfill", "resource", "icon", "result", "failed", "error", backfillErr)
		}
	})
	b.activated("backfill")
	if r.scheduler != nil {
		r.scheduler.Start()
		b.activated("scheduler")
	}
	return r, nil
}

// Quiesce rejects new writers, stops scheduler triggers, cancels background
// work, and waits for the registry quiet point. A timed-out call is retryable.
func (r *Runtime) Quiesce(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	if r.state >= runtimeQuiesced {
		r.mu.Unlock()
		return nil
	}
	if r.state == runtimeOpen {
		r.state = runtimeQuiescing
	}
	r.mu.Unlock()

	r.stopSchedulerOnce.Do(func() {
		if r.scheduler == nil {
			return
		}
		go func() {
			r.scheduler.Stop()
			close(r.schedulerDone)
		}()
	})
	if err := r.Writers.Quiesce(ctx); err != nil {
		return err
	}
	select {
	case <-r.schedulerDone:
	default:
		select {
		case <-r.schedulerDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	r.mu.Lock()
	if r.state < runtimeQuiesced {
		r.state = runtimeQuiesced
	}
	r.mu.Unlock()
	return nil
}

// Close is idempotent. Only a nil result guarantees all services are closed,
// all Runtime-owned writers are quiet, and SQLite has been closed last.
func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.Lock()
	state := r.state
	if state == runtimeClosed {
		err := r.closeErr
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	if state < runtimeClosing {
		if err := r.Quiesce(ctx); err != nil {
			return err
		}
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.state = runtimeClosing
		r.mu.Unlock()
		go r.closeResources()
	})
	select {
	case <-r.closeDone:
		return r.closeErr
	default:
	}
	select {
	case <-r.closeDone:
		return r.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) closeResources() {
	r.rootCancel()
	r.readability.Close()
	r.proxy.Close()
	// SQLite is deliberately the final owned resource closed.
	r.closeErr = r.db.Close()
	r.mu.Lock()
	r.state = runtimeClosed
	r.mu.Unlock()
	close(r.closeDone)
}

type writerRegistryLauncher struct {
	registry *WriterRegistry
}

func (l writerRegistryLauncher) ReserveWriter(initiating context.Context, class service.WriterClass) (service.WriterReservation, error) {
	token, err := l.registry.Register(initiating, class)
	if err != nil {
		return nil, err
	}
	return &writerReservation{token: token}, nil
}

type writerReservation struct {
	token    *WriterToken
	mu       sync.Mutex
	launched bool
	released bool
}

func (r *writerReservation) Context() context.Context { return r.token.Context() }
func (r *writerReservation) Publish()                 { r.token.Publish() }

func (r *writerReservation) Launch(run func(context.Context)) {
	r.mu.Lock()
	if r.launched || r.released {
		r.mu.Unlock()
		return
	}
	r.launched = true
	r.mu.Unlock()
	go func() {
		defer r.token.Complete()
		run(r.token.Context())
	}()
}

func (r *writerReservation) Release() {
	r.mu.Lock()
	if r.launched || r.released {
		r.mu.Unlock()
		return
	}
	r.released = true
	r.mu.Unlock()
	r.token.Complete()
}
