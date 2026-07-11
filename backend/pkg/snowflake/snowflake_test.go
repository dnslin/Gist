package snowflake_test

import (
	"errors"
	"sync"
	"testing"

	"gist/backend/pkg/snowflake"

	"github.com/stretchr/testify/require"
)

func TestNewGeneratorRejectsInvalidNode(t *testing.T) {
	t.Parallel()
	for _, nodeID := range []int64{-1, 1024} {
		generator, err := snowflake.NewGenerator(nodeID)
		require.Error(t, err)
		require.Nil(t, generator)
	}
}

func TestBootstrapOwnerInitializesOnce(t *testing.T) {
	t.Parallel()
	owner := snowflake.NewBootstrapOwner()
	first, err := owner.Init(1)
	require.NoError(t, err)
	require.NotNil(t, first)
	second, err := owner.Init(2)
	require.ErrorIs(t, err, snowflake.ErrBootstrapAlreadyInitialized)
	require.Nil(t, second)
}

func TestBootstrapOwnerCanRetryAfterInvalidNode(t *testing.T) {
	t.Parallel()
	owner := snowflake.NewBootstrapOwner()
	generator, err := owner.Init(-1)
	require.Error(t, err)
	require.Nil(t, generator)
	generator, err = owner.Init(0)
	require.NoError(t, err)
	require.NotNil(t, generator)
}

func TestBootstrapOwnerConcurrentInitializationHasOneWinner(t *testing.T) {
	t.Parallel()
	owner := snowflake.NewBootstrapOwner()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, nodeID := range []int64{1, 2} {
		wg.Add(1)
		go func(nodeID int64) {
			defer wg.Done()
			_, err := owner.Init(nodeID)
			errs <- err
		}(nodeID)
	}
	wg.Wait()
	close(errs)
	var successes, duplicates int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, snowflake.ErrBootstrapAlreadyInitialized):
			duplicates++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, duplicates)
}

func TestGeneratorsAreIndependent(t *testing.T) {
	t.Parallel()
	first, err := snowflake.NewGenerator(1)
	require.NoError(t, err)
	second, err := snowflake.NewGenerator(2)
	require.NoError(t, err)
	firstID := first.NextID()
	secondID := second.NextID()
	require.NotEqual(t, firstID, secondID)
	require.Equal(t, int64(1)<<12, firstID&(int64(1023)<<12))
	require.Equal(t, int64(2)<<12, secondID&(int64(1023)<<12))
}

func TestGeneratorProducesUniqueMonotonicIDs(t *testing.T) {
	t.Parallel()
	generator, err := snowflake.NewGenerator(0)
	require.NoError(t, err)
	const count = 10000
	ids := make(map[int64]struct{}, count)
	previous := generator.NextID()
	require.Positive(t, previous)
	ids[previous] = struct{}{}
	for i := 1; i < count; i++ {
		id := generator.NextID()
		require.Greater(t, id, previous)
		_, duplicate := ids[id]
		require.False(t, duplicate)
		ids[id] = struct{}{}
		previous = id
	}
	require.Len(t, ids, count)
}
