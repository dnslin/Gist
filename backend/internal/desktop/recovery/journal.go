package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	JournalFilename       = "journal.json"
	MaxRecordSize         = 64 * 1024
	PhasePrepared         = "prepared"
	PhaseApplied          = "applied"
	PhaseCommitted        = "committed"
	PhaseRollbackRequired = "rollback_required"
	PhaseRolledBack       = "rolled_back"
)

var (
	ErrCorrupt     = errors.New("recovery_corrupt")
	ErrUnsupported = errors.New("recovery_unsupported")
	ErrFailed      = errors.New("recovery_failed")
)

type Record struct {
	SchemaVersion int             `json:"schemaVersion"`
	TransactionID string          `json:"transactionId"`
	Operation     string          `json:"operation"`
	Phase         string          `json:"phase"`
	Metadata      json.RawMessage `json:"metadata"`
}

type Decision uint8

const (
	DecisionFinish Decision = iota + 1
	DecisionRollback
)

type Handler interface {
	Recover(context.Context, Record) (Decision, error)
}

type Store interface {
	Load(context.Context) ([]byte, error)
	Replace(context.Context, []byte) error
	Remove(context.Context) error
}

type Journal struct {
	store    Store
	handlers map[string]Handler
}

func NewJournal(store Store, handlers map[string]Handler) *Journal {
	ownedHandlers := make(map[string]Handler, len(handlers))
	for operation, handler := range handlers {
		ownedHandlers[operation] = handler
	}
	return &Journal{store: store, handlers: ownedHandlers}
}

func (j *Journal) Save(ctx context.Context, record Record) error {
	if j == nil || j.store == nil {
		return fmt.Errorf("%w: missing store", ErrFailed)
	}
	if err := validate(record); err != nil {
		return err
	}
	if len(record.Metadata) > MaxRecordSize {
		return ErrCorrupt
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("%w: encode", ErrCorrupt)
	}
	if len(data) > MaxRecordSize {
		return ErrCorrupt
	}
	if err := j.store.Replace(ctx, data); err != nil {
		return fmt.Errorf("%w: durable replace: %w", ErrFailed, err)
	}
	return nil
}

func (j *Journal) Replay(ctx context.Context) error {
	if j == nil || j.store == nil {
		return fmt.Errorf("%w: missing store", ErrFailed)
	}
	data, err := j.store.Load(ctx)
	if errors.Is(err, ErrAbsent) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: load: %w", ErrFailed, err)
	}
	if len(data) == 0 || len(data) > MaxRecordSize || !utf8.Valid(data) {
		return ErrCorrupt
	}
	var record Record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return ErrCorrupt
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrCorrupt
	}
	if err := validate(record); err != nil {
		return err
	}
	handler := j.handlers[record.Operation]
	if handler == nil {
		return ErrUnsupported
	}
	if record.Phase == PhaseCommitted || record.Phase == PhaseRolledBack {
		if err := j.store.Remove(ctx); err != nil {
			return fmt.Errorf("%w: durable cleanup: %w", ErrFailed, err)
		}
		return nil
	}
	decision, err := handler.Recover(ctx, record)
	if err != nil {
		return fmt.Errorf("%w: operation %s: %w", ErrFailed, record.Operation, err)
	}
	if record.Phase == PhaseRollbackRequired && decision != DecisionRollback {
		return fmt.Errorf("%w: rollback-required record was not rolled back", ErrFailed)
	}
	switch decision {
	case DecisionFinish:
		record.Phase = PhaseCommitted
	case DecisionRollback:
		record.Phase = PhaseRolledBack
	default:
		return ErrFailed
	}
	if err := j.Save(ctx, record); err != nil {
		return err
	}
	if err := j.store.Remove(ctx); err != nil {
		return fmt.Errorf("%w: durable cleanup: %w", ErrFailed, err)
	}
	return nil
}

func validate(record Record) error {
	if record.SchemaVersion != 1 {
		return ErrUnsupported
	}
	if !validIdentifier(record.TransactionID) || !validIdentifier(record.Operation) {
		return ErrCorrupt
	}
	switch record.Phase {
	case PhasePrepared, PhaseApplied, PhaseCommitted, PhaseRollbackRequired, PhaseRolledBack:
	default:
		return ErrUnsupported
	}
	if len(record.Metadata) == 0 || len(record.Metadata) > MaxRecordSize || !utf8.Valid(record.Metadata) || !json.Valid(record.Metadata) || bytes.Equal(bytes.TrimSpace(record.Metadata), []byte("null")) {
		return ErrCorrupt
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}
