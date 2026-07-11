package service

import "context"

// WriterClass controls how application lifecycle treats an admitted writer.
type WriterClass uint8

const (
	WriterBackground WriterClass = iota + 1
	WriterRequestBound
)

// WriterLauncher admits and starts asynchronous local-data writers.
// Implementations must complete admission before returning and must not invoke run
// when admission is rejected.
type WriterLauncher interface {
	LaunchWriter(initiating context.Context, class WriterClass, run func(context.Context)) error
}
