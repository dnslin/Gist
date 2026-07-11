package service_test

import (
	"context"

	"gist/backend/internal/service"
)

type testWriterLauncher struct{}

func (testWriterLauncher) LaunchWriter(ctx context.Context, _ service.WriterClass, run func(context.Context)) error {
	go run(ctx)
	return nil
}

type rejectingWriterLauncher struct {
	err error
}

func (l rejectingWriterLauncher) LaunchWriter(context.Context, service.WriterClass, func(context.Context)) error {
	return l.err
}
