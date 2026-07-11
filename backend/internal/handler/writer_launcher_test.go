package handler_test

import (
	"context"

	"gist/backend/internal/service"
)

type writerLauncherFunc func(context.Context, service.WriterClass, func(context.Context)) error

func (f writerLauncherFunc) LaunchWriter(ctx context.Context, class service.WriterClass, run func(context.Context)) error {
	return f(ctx, class, run)
}

func launchTestWriter(ctx context.Context, _ service.WriterClass, run func(context.Context)) error {
	go run(ctx)
	return nil
}
