//go:build windows

package ownership

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

const activationTimeout = 2 * time.Second

type ActivationServer struct {
	identity  Identity
	sink      ActivationSink
	stop      chan struct{}
	done      chan struct{}
	ready     chan error
	readyOnce sync.Once
	closeOnce sync.Once
}

func StartActivationServer(identity Identity, sink ActivationSink) (*ActivationServer, error) {
	if sink == nil || identity.PipeName == "" || identity.UserSID == "" {
		return nil, ErrActivationProtocolInvalid
	}
	s := &ActivationServer{
		identity: identity,
		sink:     sink,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		ready:    make(chan error, 1),
	}
	go s.serve()
	if err := <-s.ready; err != nil {
		<-s.done
		return nil, err
	}
	return s, nil
}

func (s *ActivationServer) serve() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		sa, err := securityAttributes(s.identity.UserSID)
		if err != nil {
			s.signalReady(err)
			return
		}
		name, err := windows.UTF16PtrFromString(s.identity.PipeName)
		if err != nil {
			s.signalReady(err)
			return
		}
		handle, err := windows.CreateNamedPipe(
			name,
			windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE|windows.FILE_FLAG_OVERLAPPED,
			windows.PIPE_TYPE_MESSAGE|windows.PIPE_READMODE_MESSAGE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
			1,
			MaxFrameSize+4,
			MaxFrameSize+4,
			uint32(activationTimeout/time.Millisecond),
			sa,
		)
		if err != nil {
			s.signalReady(err)
			return
		}
		s.signalReady(nil)
		deadline := time.Now().Add(activationTimeout)
		if err := connectPipe(handle, deadline); err != nil {
			_ = windows.CloseHandle(handle)
			select {
			case <-s.stop:
				return
			default:
				continue
			}
		}
		frame, readErr := readPipeMessage(handle, deadline)
		if readErr == nil {
			extra, peekErr := pipeHasData(handle)
			if peekErr == nil && !extra {
				requestCtx, cancel := context.WithDeadline(context.Background(), deadline)
				response, handleErr := HandleFrame(requestCtx, frame, s.sink)
				cancel()
				if handleErr == nil {
					encoded, encodeErr := EncodeResponse(response.Result)
					if encodeErr == nil {
						_ = writePipeMessage(handle, encoded, deadline)
					}
				}
			}
		}
		_ = windows.DisconnectNamedPipe(handle)
		_ = windows.CloseHandle(handle)
	}
}

func (s *ActivationServer) signalReady(err error) {
	s.readyOnce.Do(func() { s.ready <- err })
}

func (s *ActivationServer) Close() error {
	s.closeOnce.Do(func() {
		close(s.stop)
		select {
		case <-s.done:
			return
		default:
		}
		name, err := windows.UTF16PtrFromString(s.identity.PipeName)
		if err == nil {
			handle, openErr := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
			if openErr == nil {
				var zero [4]byte
				_, _ = windows.Write(handle, zero[:])
				_ = windows.CloseHandle(handle)
			}
		}
		<-s.done
	})
	return nil
}

type WindowsActivationClient struct{}

func (WindowsActivationClient) Activate(ctx context.Context, identity Identity) (Response, error) {
	deadline := time.Now().Add(activationTimeout)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return Response{}, context.DeadlineExceeded
	}
	name, err := windows.UTF16PtrFromString(identity.PipeName)
	if err != nil {
		return Response{}, err
	}
	wait := uint32(remaining / time.Millisecond)
	if wait == 0 {
		wait = 1
	}
	if err := waitNamedPipe(name, wait); err != nil {
		return Response{}, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return Response{}, err
	}
	defer windows.CloseHandle(handle)
	mode := uint32(windows.PIPE_READMODE_MESSAGE)
	if err := windows.SetNamedPipeHandleState(handle, &mode, nil, nil); err != nil {
		return Response{}, err
	}
	request, err := EncodeRequest()
	if err != nil {
		return Response{}, err
	}
	if err := writePipeMessage(handle, request, deadline); err != nil {
		return Response{}, err
	}
	frame, err := readPipeMessage(handle, deadline)
	if err != nil {
		return Response{}, err
	}
	return DecodeResponse(frame)
}

func connectPipe(handle windows.Handle, deadline time.Time) error {
	overlapped, closeEvent, err := newOverlapped()
	if err != nil {
		return err
	}
	defer closeEvent()
	err = windows.ConnectNamedPipe(handle, overlapped)
	if err == nil || errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		return nil
	}
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		return err
	}
	return awaitOverlapped(handle, overlapped, deadline, nil)
}

func readPipeMessage(handle windows.Handle, deadline time.Time) ([]byte, error) {
	buffer := make([]byte, MaxFrameSize+5)
	overlapped, closeEvent, err := newOverlapped()
	if err != nil {
		return nil, err
	}
	defer closeEvent()
	var done uint32
	err = windows.ReadFile(handle, buffer, &done, overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		err = awaitOverlapped(handle, overlapped, deadline, &done)
	}
	if err != nil {
		return nil, err
	}
	if done < 4 || done > MaxFrameSize+4 {
		return nil, ErrActivationProtocolInvalid
	}
	return buffer[:done], nil
}

func writePipeMessage(handle windows.Handle, message []byte, deadline time.Time) error {
	overlapped, closeEvent, err := newOverlapped()
	if err != nil {
		return err
	}
	defer closeEvent()
	var done uint32
	err = windows.WriteFile(handle, message, &done, overlapped)
	if errors.Is(err, windows.ERROR_IO_PENDING) {
		err = awaitOverlapped(handle, overlapped, deadline, &done)
	}
	if err != nil {
		return err
	}
	if int(done) != len(message) {
		return io.ErrShortWrite
	}
	return nil
}

func newOverlapped() (*windows.Overlapped, func(), error) {
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, nil, err
	}
	return &windows.Overlapped{HEvent: event}, func() { _ = windows.CloseHandle(event) }, nil
}

func awaitOverlapped(handle windows.Handle, overlapped *windows.Overlapped, deadline time.Time, done *uint32) error {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		_ = windows.CancelIoEx(handle, overlapped)
		return context.DeadlineExceeded
	}
	wait := uint32((remaining + time.Millisecond - 1) / time.Millisecond)
	waitResult, waitErr := windows.WaitForSingleObject(overlapped.HEvent, wait)
	if waitErr != nil {
		_ = windows.CancelIoEx(handle, overlapped)
		return waitErr
	}
	if waitResult == waitTimeout {
		_ = windows.CancelIoEx(handle, overlapped)
		var ignored uint32
		if done == nil {
			done = &ignored
		}
		_ = windows.GetOverlappedResult(handle, overlapped, done, true)
		return context.DeadlineExceeded
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		_ = windows.CancelIoEx(handle, overlapped)
		return ErrActivationProtocolInvalid
	}
	var ignored uint32
	if done == nil {
		done = &ignored
	}
	return windows.GetOverlappedResult(handle, overlapped, done, false)
}
