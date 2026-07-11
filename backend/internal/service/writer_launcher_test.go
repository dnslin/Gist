package service_test

import (
	"context"

	"gist/backend/internal/service"
)

type testWriterLauncher struct{}

func (testWriterLauncher) ReserveWriter(ctx context.Context, _ service.WriterClass) (service.WriterReservation, error) {
	return &testWriterReservation{ctx: ctx}, nil
}

type testWriterReservation struct {
	ctx context.Context
}

func (r *testWriterReservation) Context() context.Context { return r.ctx }
func (r *testWriterReservation) Publish()                 {}
func (r *testWriterReservation) Launch(run func(context.Context)) {
	go run(r.ctx)
}
func (r *testWriterReservation) Release() {}

type rejectingWriterLauncher struct {
	err error
}

func (l rejectingWriterLauncher) ReserveWriter(context.Context, service.WriterClass) (service.WriterReservation, error) {
	return nil, l.err
}
