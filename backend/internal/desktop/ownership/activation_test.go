package ownership_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/desktop/ownership"
)

type clientFunc func(context.Context, ownership.Identity) (ownership.Response, error)

func (f clientFunc) Activate(ctx context.Context, id ownership.Identity) (ownership.Response, error) {
	return f(ctx, id)
}

func TestRouteContenderNeverContactsOtherSession(t *testing.T) {
	called := false
	response, err := ownership.RouteContender(
		context.Background(),
		ownership.Identity{},
		ownership.Acquisition{Outcome: ownership.OutcomeOwnedOtherSession},
		clientFunc(func(context.Context, ownership.Identity) (ownership.Response, error) {
			called = true
			return ownership.Response{}, nil
		}),
	)
	require.ErrorIs(t, err, ownership.ErrOwnedOtherSession)
	require.Equal(t, ownership.ResultOccupiedOtherSession, response.Result)
	require.False(t, called)
}

func TestRouteContenderActivatesSameSessionAndStillRejectsStartup(t *testing.T) {
	called := 0
	response, err := ownership.RouteContender(
		context.Background(),
		ownership.Identity{},
		ownership.Acquisition{Outcome: ownership.OutcomeOwnedSameSession},
		clientFunc(func(context.Context, ownership.Identity) (ownership.Response, error) {
			called++
			return ownership.Response{Version: 1, Result: ownership.ResultAccepted}, nil
		}),
	)
	require.ErrorIs(t, err, ownership.ErrOwnedSameSession)
	require.Equal(t, ownership.ResultAccepted, response.Result)
	require.Equal(t, 1, called)
}

func TestRouteContenderFailsClosedWhenSameSessionEndpointUnavailable(t *testing.T) {
	cases := []clientFunc{
		func(context.Context, ownership.Identity) (ownership.Response, error) {
			return ownership.Response{}, errors.New("unreachable")
		},
		func(context.Context, ownership.Identity) (ownership.Response, error) {
			return ownership.Response{Version: 1, Result: ownership.ResultOccupiedOtherSession}, nil
		},
	}
	for _, client := range cases {
		response, err := ownership.RouteContender(context.Background(), ownership.Identity{}, ownership.Acquisition{Outcome: ownership.OutcomeOwnedSameSession}, client)
		require.ErrorIs(t, err, ownership.ErrOwnerUnreachable)
		require.Equal(t, ownership.ResultOccupiedUnreachable, response.Result)
	}
}
