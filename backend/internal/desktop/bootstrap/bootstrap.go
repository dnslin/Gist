package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"gist/backend/internal/application"
	"gist/backend/internal/desktop/ownership"
	"gist/backend/internal/desktop/paths"
	"gist/backend/pkg/snowflake"
)

var ErrBootstrapFailed = errors.New("bootstrap_failed")

type Closer interface {
	Close() error
}

type Stage func(context.Context, paths.Paths) (Closer, error)
type ActivationServerFactory func(context.Context, ownership.Identity) (Closer, error)

type RecoveryRunner interface {
	Replay(context.Context) error
}

type Runtime interface {
	Close(context.Context) error
}

type RuntimeFactory interface {
	NewRuntime(context.Context, application.RuntimeOptions) (Runtime, error)
}

type Dependencies struct {
	ResolvePaths    func() (paths.Paths, error)
	DeriveIdentity  func(paths.Paths) (ownership.Identity, error)
	AcquireOwner    ownership.Acquirer
	ActivateOwner   ownership.ActivationClient
	StartActivation ActivationServerFactory
	OpenLogs        Stage
	Recover         RecoveryRunner
	LoadConfig      Stage
	OpenCredentials Stage
	NewGenerator    func(context.Context) (snowflake.Generator, Closer, error)
	NewRuntime      RuntimeFactory
	Checkpoint      func(string) error
}

type Host struct {
	Runtime  Runtime
	mu       sync.Mutex
	cleanups []func(context.Context) error
	closed   bool
}

func Start(ctx context.Context, deps Dependencies) (*Host, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, err
	}
	p, err := deps.ResolvePaths()
	if err != nil {
		return nil, fmt.Errorf("%w: paths: %w", ErrBootstrapFailed, err)
	}
	identity, err := deps.DeriveIdentity(p)
	if err != nil {
		return nil, fmt.Errorf("%w: identity: %w", ErrBootstrapFailed, err)
	}
	acquired, err := deps.AcquireOwner.Acquire(ctx, identity)
	if err != nil {
		return nil, fmt.Errorf("%w: ownership: %w", ErrBootstrapFailed, err)
	}
	switch acquired.Outcome {
	case ownership.OutcomeOwnedSameSession, ownership.OutcomeOwnedOtherSession:
		_, routeErr := ownership.RouteContender(ctx, identity, acquired, deps.ActivateOwner)
		return nil, routeErr
	case ownership.OutcomeOwnedUnreachable:
		return nil, ownership.ErrOwnerUnreachable
	case ownership.OutcomeAcquired, ownership.OutcomeAcquiredAbandoned:
	default:
		return nil, fmt.Errorf("%w: invalid ownership outcome", ErrBootstrapFailed)
	}
	if acquired.Lease == nil {
		return nil, fmt.Errorf("%w: acquired ownership without lease", ErrBootstrapFailed)
	}

	host := &Host{}
	host.cleanups = append(host.cleanups, func(context.Context) error { return acquired.Lease.Close() })
	fail := func(stage string, cause error) (*Host, error) {
		cleanupErr := host.closeAll(ctx)
		return nil, errors.Join(fmt.Errorf("%w: %s: %w", ErrBootstrapFailed, stage, cause), cleanupErr)
	}
	checkpoint := func(stage string) error {
		if deps.Checkpoint != nil {
			return deps.Checkpoint(stage)
		}
		return nil
	}
	addCloser := func(closer Closer) {
		if closer != nil {
			host.cleanups = append(host.cleanups, func(context.Context) error { return closer.Close() })
		}
	}
	addStage := func(name string, stage Stage) error {
		if err := checkpoint(name); err != nil {
			return err
		}
		closer, err := stage(ctx, p)
		if err != nil {
			if closer != nil {
				return errors.Join(err, closer.Close())
			}
			return err
		}
		addCloser(closer)
		return nil
	}

	if err := checkpoint("activation"); err != nil {
		return fail("activation", err)
	}
	activation, err := deps.StartActivation(ctx, identity)
	if err != nil {
		if activation != nil {
			err = errors.Join(err, activation.Close())
		}
		return fail("activation", err)
	}
	addCloser(activation)
	if err := addStage("logs", deps.OpenLogs); err != nil {
		return fail("logs", err)
	}
	if err := checkpoint("recovery"); err != nil {
		return fail("recovery", err)
	}
	if err := deps.Recover.Replay(ctx); err != nil {
		return fail("recovery", err)
	}
	if err := addStage("config", deps.LoadConfig); err != nil {
		return fail("config", err)
	}
	if err := addStage("credentials", deps.OpenCredentials); err != nil {
		return fail("credentials", err)
	}
	if err := checkpoint("generator"); err != nil {
		return fail("generator", err)
	}
	generator, generatorCloser, err := deps.NewGenerator(ctx)
	if err != nil {
		if generatorCloser != nil {
			err = errors.Join(err, generatorCloser.Close())
		}
		return fail("generator", err)
	}
	if generator == nil {
		var closeErr error
		if generatorCloser != nil {
			closeErr = generatorCloser.Close()
		}
		return fail("generator", errors.Join(errors.New("nil generator"), closeErr))
	}
	addCloser(generatorCloser)
	if err := checkpoint("runtime"); err != nil {
		return fail("runtime", err)
	}
	runtime, err := deps.NewRuntime.NewRuntime(ctx, application.RuntimeOptions{
		DataDir:        p.DataDir,
		DBPath:         p.DBPath,
		IDGenerator:    generator,
		StartScheduler: true,
	})
	if err != nil {
		if runtime != nil {
			err = errors.Join(err, runtime.Close(ctx))
		}
		return fail("runtime", err)
	}
	if runtime == nil {
		return fail("runtime", errors.New("nil runtime"))
	}
	host.Runtime = runtime
	host.cleanups = append(host.cleanups, func(closeCtx context.Context) error { return runtime.Close(closeCtx) })
	return host, nil
}

func (h *Host) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	for len(h.cleanups) > 0 {
		last := len(h.cleanups) - 1
		if err := h.cleanups[last](ctx); err != nil {
			return err
		}
		h.cleanups = h.cleanups[:last]
	}
	h.closed = true
	return nil
}

func (h *Host) closeAll(ctx context.Context) error {
	if h.closed {
		return nil
	}
	var result error
	for i := len(h.cleanups) - 1; i >= 0; i-- {
		result = errors.Join(result, h.cleanups[i](ctx))
	}
	h.cleanups = nil
	h.closed = true
	return result
}

func validateDependencies(d Dependencies) error {
	if d.ResolvePaths == nil || d.DeriveIdentity == nil || d.AcquireOwner == nil || d.ActivateOwner == nil || d.StartActivation == nil || d.OpenLogs == nil || d.Recover == nil || d.LoadConfig == nil || d.OpenCredentials == nil || d.NewGenerator == nil || d.NewRuntime == nil {
		return fmt.Errorf("%w: incomplete dependencies", ErrBootstrapFailed)
	}
	return nil
}
