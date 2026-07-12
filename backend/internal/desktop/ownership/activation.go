package ownership

import "context"

type ActivationClient interface {
	Activate(context.Context, Identity) (Response, error)
}

func RouteContender(ctx context.Context, identity Identity, acquisition Acquisition, client ActivationClient) (Response, error) {
	switch acquisition.Outcome {
	case OutcomeOwnedOtherSession:
		return Response{Version: 1, Result: ResultOccupiedOtherSession}, ErrOwnedOtherSession
	case OutcomeOwnedSameSession:
		if client == nil {
			return Response{Version: 1, Result: ResultOccupiedUnreachable}, ErrOwnerUnreachable
		}
		response, err := client.Activate(ctx, identity)
		if err != nil || response.Version != 1 || response.Result != ResultAccepted {
			return Response{Version: 1, Result: ResultOccupiedUnreachable}, ErrOwnerUnreachable
		}
		return response, ErrOwnedSameSession
	default:
		return Response{}, ErrInvalidIdentity
	}
}
