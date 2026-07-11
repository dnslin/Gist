package handler_test

import (
	"context"

	"gist/backend/internal/service"
)

type writerLauncherFunc func(context.Context, service.WriterClass) (service.WriterReservation, error)

func (f writerLauncherFunc) ReserveWriter(ctx context.Context, class service.WriterClass) (service.WriterReservation, error) {
	return f(ctx, class)
}

type handlerTestReservation struct {
	ctx context.Context
}

func (r *handlerTestReservation) Context() context.Context { return r.ctx }
func (r *handlerTestReservation) Publish()                 {}
func (r *handlerTestReservation) Launch(run func(context.Context)) {
	go run(r.ctx)
}
func (r *handlerTestReservation) Release() {}

func reserveTestWriter(ctx context.Context, _ service.WriterClass) (service.WriterReservation, error) {
	return &handlerTestReservation{ctx: ctx}, nil
}

func reserveAcceptedWriter(initiating context.Context, _ service.WriterClass) (service.WriterReservation, error) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(initiating))
	return &acceptedTestReservation{ctx: ctx, cancel: cancel, stopInitiating: context.AfterFunc(initiating, cancel)}, nil
}

type acceptedTestReservation struct {
	ctx            context.Context
	cancel         context.CancelFunc
	stopInitiating func() bool
	launched       bool
}

func (r *acceptedTestReservation) Context() context.Context { return r.ctx }
func (r *acceptedTestReservation) Publish()                 { r.stopInitiating() }
func (r *acceptedTestReservation) Launch(run func(context.Context)) {
	r.launched = true
	go func() { defer r.cancel(); run(r.ctx) }()
}
func (r *acceptedTestReservation) Release() {
	if !r.launched {
		r.cancel()
	}
}
