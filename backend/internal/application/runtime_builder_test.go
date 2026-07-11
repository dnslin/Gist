package application

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/pkg/snowflake"
)

func TestRuntimeBuilderFailureCleansAcquiredResourcesInReverseWithoutActivation(t *testing.T) {
	generator, err := snowflake.NewGenerator(1)
	require.NoError(t, err)
	fault := errors.New("injected build fault")
	tests := []struct {
		stage   runtimeBuildStage
		cleanup []string
	}{
		{stage: buildDatabase, cleanup: []string{"database", "root"}},
		{stage: buildRepositories, cleanup: []string{"database", "root"}},
		{stage: buildServices, cleanup: []string{"proxy", "readability", "database", "root"}},
		{stage: buildRouter, cleanup: []string{"proxy", "readability", "database", "root"}},
		{stage: buildScheduler, cleanup: []string{"proxy", "readability", "database", "root"}},
		{stage: buildBackfill, cleanup: []string{"backfill", "proxy", "readability", "database", "root"}},
	}
	for _, test := range tests {
		t.Run(string(test.stage), func(t *testing.T) {
			var cleanup []string
			var activation []string
			builder := runtimeBuilder{
				checkpoint: func(stage runtimeBuildStage) error {
					if stage == test.stage {
						return fault
					}
					return nil
				},
				cleanupObserved:  func(resource string) { cleanup = append(cleanup, resource) },
				activateObserved: func(resource string) { activation = append(activation, resource) },
			}
			runtime, buildErr := builder.Build(context.Background(), RuntimeOptions{
				DataDir:        filepath.Join(t.TempDir(), "data"),
				DBPath:         filepath.Join(t.TempDir(), "gist.db"),
				IDGenerator:    generator,
				StartScheduler: true,
			})
			require.ErrorIs(t, buildErr, fault)
			require.Nil(t, runtime)
			require.Equal(t, test.cleanup, cleanup)
			require.Empty(t, activation)
		})
	}
}
