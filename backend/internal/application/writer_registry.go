package application

import (
	"context"
	"errors"
	"sync"
)

// WriterClass controls how a writer is treated while the application quiesces.
type WriterClass uint8

const (
	WriterBackground WriterClass = iota + 1
	WriterRequestBound
)

var (
	ErrWriterAdmissionClosed = errors.New("writer admission closed")
	ErrInvalidWriterClass    = errors.New("invalid writer class")
	ErrWriterQuiesceDeadline = errors.New("writer quiesce deadline exceeded")
)

// WriterRegistry admits asynchronous local-data writers and provides their quiet point.
type WriterRegistry struct {
	root context.Context

	mu        sync.Mutex
	accepting bool
	writers   map[*WriterToken]struct{}
	quiet     chan struct{}
	quietOnce sync.Once
}

// WriterToken is reserved before a writer is launched. Complete must be called
// when the writer exits; repeated calls are safe.
type WriterToken struct {
	registry *WriterRegistry
	class    WriterClass
	ctx      context.Context
	cancel   context.CancelFunc
	stopRoot func() bool
	once     sync.Once
}

func NewWriterRegistry(root context.Context) *WriterRegistry {
	if root == nil {
		root = context.Background()
	}
	return &WriterRegistry{
		root:      root,
		accepting: true,
		writers:   make(map[*WriterToken]struct{}),
		quiet:     make(chan struct{}),
	}
}

// Register reserves admission and returns the linked writer context. The
// initiating context supplies values and cancellation; Runtime root
// cancellation is linked independently.
func (r *WriterRegistry) Register(initiating context.Context, class WriterClass) (*WriterToken, error) {
	if class != WriterBackground && class != WriterRequestBound {
		return nil, ErrInvalidWriterClass
	}
	if initiating == nil {
		initiating = context.Background()
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.accepting {
		return nil, ErrWriterAdmissionClosed
	}

	ctx, cancel := context.WithCancel(initiating)
	token := &WriterToken{registry: r, class: class, ctx: ctx, cancel: cancel}
	token.stopRoot = context.AfterFunc(r.root, cancel)
	r.writers[token] = struct{}{}
	return token, nil
}

func (t *WriterToken) Context() context.Context { return t.ctx }

// Complete releases the reservation exactly once and cancels the linked
// context so any child operations owned by the writer are also released.
func (t *WriterToken) Complete() {
	if t == nil {
		return
	}
	t.once.Do(func() {
		if t.stopRoot != nil {
			t.stopRoot()
		}
		t.cancel()
		t.registry.complete(t)
	})
}

func (r *WriterRegistry) complete(token *WriterToken) {
	r.mu.Lock()
	delete(r.writers, token)
	if !r.accepting && len(r.writers) == 0 {
		r.quietOnce.Do(func() { close(r.quiet) })
	}
	r.mu.Unlock()
}

// Quiesce permanently closes admission, immediately cancels background
// writers, and waits for bound writers to complete. If waitCtx expires, all
// remaining bound writers are force-cancelled. A later call continues waiting
// for their completion.
func (r *WriterRegistry) Quiesce(waitCtx context.Context) error {
	if waitCtx == nil {
		waitCtx = context.Background()
	}

	r.mu.Lock()
	if r.accepting {
		r.accepting = false
		for writer := range r.writers {
			if writer.class == WriterBackground {
				writer.cancel()
			}
		}
		if len(r.writers) == 0 {
			r.quietOnce.Do(func() { close(r.quiet) })
		}
	}
	quiet := r.quiet
	r.mu.Unlock()

	select {
	case <-quiet:
		return nil
	default:
	}
	select {
	case <-quiet:
		return nil
	case <-waitCtx.Done():
		r.forceCancelBound()
		return errors.Join(ErrWriterQuiesceDeadline, waitCtx.Err())
	}
}

func (r *WriterRegistry) forceCancelBound() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for writer := range r.writers {
		if writer.class == WriterRequestBound {
			writer.cancel()
		}
	}
}
