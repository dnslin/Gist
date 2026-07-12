package bootstrap_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/application"
	"gist/backend/internal/desktop/bootstrap"
	"gist/backend/internal/desktop/ownership"
	"gist/backend/internal/desktop/paths"
	"gist/backend/pkg/snowflake"
)

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

type acquirerFunc func(context.Context, ownership.Identity) (ownership.Acquisition, error)

func (f acquirerFunc) Acquire(ctx context.Context, id ownership.Identity) (ownership.Acquisition, error) {
	return f(ctx, id)
}

type bootstrapClientFunc func(context.Context, ownership.Identity) (ownership.Response, error)

func (f bootstrapClientFunc) Activate(ctx context.Context, id ownership.Identity) (ownership.Response, error) {
	return f(ctx, id)
}

type runtimeFunc func(context.Context, application.RuntimeOptions) (bootstrap.Runtime, error)

func (f runtimeFunc) NewRuntime(ctx context.Context, options application.RuntimeOptions) (bootstrap.Runtime, error) {
	return f(ctx, options)
}

type runtimeStub struct{ close func() error }

func (r runtimeStub) Close(context.Context) error { return r.close() }

type runnerFunc func(context.Context) error

func (f runnerFunc) Replay(ctx context.Context) error { return f(ctx) }

func TestSameSessionLockLoserOnlyActivatesOwner(t *testing.T) {
	var dataCalls int
	deps := baseDeps(t, &dataCalls, nil)
	deps.AcquireOwner = acquirerFunc(func(context.Context, ownership.Identity) (ownership.Acquisition, error) {
		return ownership.Acquisition{Outcome: ownership.OutcomeOwnedSameSession}, nil
	})
	activationCalls := 0
	deps.ActivateOwner = bootstrapClientFunc(func(context.Context, ownership.Identity) (ownership.Response, error) {
		activationCalls++
		return ownership.Response{Version: 1, Result: ownership.ResultAccepted}, nil
	})
	host, err := bootstrap.Start(context.Background(), deps)
	require.ErrorIs(t, err, ownership.ErrOwnedSameSession)
	require.Nil(t, host)
	require.Equal(t, 1, activationCalls)
	require.Zero(t, dataCalls)
}

func TestOtherSessionLockLoserTouchesNoEndpointOrDataStage(t *testing.T) {
	var dataCalls int
	deps := baseDeps(t, &dataCalls, nil)
	deps.AcquireOwner = acquirerFunc(func(context.Context, ownership.Identity) (ownership.Acquisition, error) {
		return ownership.Acquisition{Outcome: ownership.OutcomeOwnedOtherSession}, nil
	})
	activationCalls := 0
	deps.ActivateOwner = bootstrapClientFunc(func(context.Context, ownership.Identity) (ownership.Response, error) {
		activationCalls++
		return ownership.Response{}, nil
	})
	host, err := bootstrap.Start(context.Background(), deps)
	require.ErrorIs(t, err, ownership.ErrOwnedOtherSession)
	require.Nil(t, host)
	require.Zero(t, activationCalls)
	require.Zero(t, dataCalls)
}

func TestRecoveryPrecedesRuntimeAndCleanupIsLIFOWithLockLast(t *testing.T) {
	var order []string
	var dataCalls int
	deps := baseDeps(t, &dataCalls, &order)
	host, err := bootstrap.Start(context.Background(), deps)
	require.NoError(t, err)
	require.Equal(t, []string{"activation", "logs", "recovery", "config", "credentials", "generator", "runtime"}, order)
	require.NoError(t, host.Close(context.Background()))
	require.Equal(t, []string{"activation", "logs", "recovery", "config", "credentials", "generator", "runtime", "runtime_close", "generator_close", "credentials_close", "config_close", "logs_close", "activation_close", "lock_close"}, order)
}

func TestEveryStageFaultPreventsLaterStagesAndReleasesLockLast(t *testing.T) {
	for _, fail := range []string{"activation", "logs", "recovery", "config", "credentials", "generator", "runtime"} {
		t.Run(fail, func(t *testing.T) {
			fault := errors.New("fault")
			var order []string
			var dataCalls int
			deps := baseDeps(t, &dataCalls, &order)
			deps.Checkpoint = func(stage string) error {
				if stage == fail {
					return fault
				}
				return nil
			}
			host, err := bootstrap.Start(context.Background(), deps)
			require.ErrorIs(t, err, fault)
			require.Nil(t, host)
			require.Equal(t, "lock_close", order[len(order)-1])
		})
	}
}

func TestConstructionErrorsCloseReturnedResourcesBeforePriorStages(t *testing.T) {
	fault := errors.New("fault")
	t.Run("activation", func(t *testing.T) {
		var order []string
		var dataCalls int
		deps := baseDeps(t, &dataCalls, &order)
		deps.StartActivation = func(context.Context, ownership.Identity) (bootstrap.Closer, error) {
			return closerFunc(func() error { order = append(order, "activation_partial_close"); return nil }), fault
		}
		_, err := bootstrap.Start(context.Background(), deps)
		require.ErrorIs(t, err, fault)
		require.Equal(t, []string{"activation_partial_close", "lock_close"}, order)
	})

	t.Run("generator", func(t *testing.T) {
		var order []string
		var dataCalls int
		deps := baseDeps(t, &dataCalls, &order)
		deps.NewGenerator = func(context.Context) (snowflake.Generator, bootstrap.Closer, error) {
			return nil, closerFunc(func() error { order = append(order, "generator_partial_close"); return nil }), fault
		}
		_, err := bootstrap.Start(context.Background(), deps)
		require.ErrorIs(t, err, fault)
		require.Equal(t, "generator_partial_close", order[len(order)-6])
		require.Equal(t, "lock_close", order[len(order)-1])
	})

	t.Run("runtime", func(t *testing.T) {
		var order []string
		var dataCalls int
		deps := baseDeps(t, &dataCalls, &order)
		deps.NewRuntime = runtimeFunc(func(context.Context, application.RuntimeOptions) (bootstrap.Runtime, error) {
			return runtimeStub{close: func() error { order = append(order, "runtime_partial_close"); return nil }}, fault
		})
		_, err := bootstrap.Start(context.Background(), deps)
		require.ErrorIs(t, err, fault)
		require.Contains(t, order, "runtime_partial_close")
		require.Equal(t, "lock_close", order[len(order)-1])
	})
}

func baseDeps(t *testing.T, dataCalls *int, order *[]string) bootstrap.Dependencies {
	t.Helper()
	appendOrder := func(value string) {
		if order != nil {
			*order = append(*order, value)
		}
	}
	p := paths.Paths{Root: t.TempDir()}
	p.DataDir = filepath.Join(p.Root, "data")
	p.DBPath = filepath.Join(p.DataDir, "gist.db")
	id, err := ownership.DeriveIdentity(p.Root, "S-1-test", 1)
	require.NoError(t, err)
	stage := func(name string) bootstrap.Stage {
		return func(context.Context, paths.Paths) (bootstrap.Closer, error) {
			*dataCalls++
			appendOrder(name)
			return closerFunc(func() error { appendOrder(name + "_close"); return nil }), nil
		}
	}
	return bootstrap.Dependencies{
		ResolvePaths:   func() (paths.Paths, error) { return p, nil },
		DeriveIdentity: func(paths.Paths) (ownership.Identity, error) { return id, nil },
		AcquireOwner: acquirerFunc(func(context.Context, ownership.Identity) (ownership.Acquisition, error) {
			return ownership.Acquisition{Outcome: ownership.OutcomeAcquired, Lease: closerFunc(func() error { appendOrder("lock_close"); return nil })}, nil
		}),
		ActivateOwner: bootstrapClientFunc(func(context.Context, ownership.Identity) (ownership.Response, error) {
			return ownership.Response{Version: 1, Result: ownership.ResultAccepted}, nil
		}),
		StartActivation: func(_ context.Context, got ownership.Identity) (bootstrap.Closer, error) {
			require.Equal(t, id, got)
			*dataCalls++
			appendOrder("activation")
			return closerFunc(func() error { appendOrder("activation_close"); return nil }), nil
		},
		OpenLogs: stage("logs"),
		Recover: runnerFunc(func(context.Context) error {
			*dataCalls++
			appendOrder("recovery")
			return nil
		}),
		LoadConfig:      stage("config"),
		OpenCredentials: stage("credentials"),
		NewGenerator: func(context.Context) (snowflake.Generator, bootstrap.Closer, error) {
			*dataCalls++
			appendOrder("generator")
			generator, err := snowflake.NewGenerator(1)
			return generator, closerFunc(func() error { appendOrder("generator_close"); return nil }), err
		},
		NewRuntime: runtimeFunc(func(_ context.Context, options application.RuntimeOptions) (bootstrap.Runtime, error) {
			*dataCalls++
			appendOrder("runtime")
			require.Equal(t, p.DataDir, options.DataDir)
			require.Equal(t, p.DBPath, options.DBPath)
			require.NotNil(t, options.IDGenerator)
			return runtimeStub{close: func() error { appendOrder("runtime_close"); return nil }}, nil
		}),
	}
}
