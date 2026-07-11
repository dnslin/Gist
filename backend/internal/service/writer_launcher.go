package service

import "context"

// WriterClass controls how application lifecycle treats an admitted writer.
type WriterClass uint8

const (
	WriterBackground WriterClass = iota + 1
	WriterRequestBound
)

// WriterReservation owns one admitted writer slot. Publish transfers cancellation
// ownership away from the initiating HTTP request after synchronous task
// publication. Launch is non-failing and Release safely abandons an unlaunched
// reservation.
type WriterReservation interface {
	Context() context.Context
	Publish()
	Launch(run func(context.Context))
	Release()
}

// WriterLauncher reserves asynchronous local-data writer capacity before any
// task publication, goroutine launch, or successful response.
type WriterLauncher interface {
	ReserveWriter(initiating context.Context, class WriterClass) (WriterReservation, error)
}
