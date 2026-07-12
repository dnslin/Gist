package recovery_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/desktop/recovery"
)

type handlerFunc func(context.Context, recovery.Record) (recovery.Decision, error)

func (f handlerFunc) Recover(ctx context.Context, record recovery.Record) (recovery.Decision, error) {
	return f(ctx, record)
}

type memoryStore struct {
	data     []byte
	replaced [][]byte
	removed  bool
}

func (s *memoryStore) Load(context.Context) ([]byte, error) {
	if s.data == nil {
		return nil, recovery.ErrAbsent
	}
	return append([]byte(nil), s.data...), nil
}

func (s *memoryStore) Replace(_ context.Context, data []byte) error {
	s.data = append([]byte(nil), data...)
	s.replaced = append(s.replaced, append([]byte(nil), data...))
	return nil
}

func (s *memoryStore) Remove(context.Context) error {
	s.data = nil
	s.removed = true
	return nil
}

func TestJournalStateMachineTransitionsAndCleanup(t *testing.T) {
	tests := []struct {
		name          string
		phase         string
		decision      recovery.Decision
		expectedPhase string
		handlerCalls  int
	}{
		{name: "prepared finish", phase: recovery.PhasePrepared, decision: recovery.DecisionFinish, expectedPhase: recovery.PhaseCommitted, handlerCalls: 1},
		{name: "applied finish", phase: recovery.PhaseApplied, decision: recovery.DecisionFinish, expectedPhase: recovery.PhaseCommitted, handlerCalls: 1},
		{name: "prepared rollback", phase: recovery.PhasePrepared, decision: recovery.DecisionRollback, expectedPhase: recovery.PhaseRolledBack, handlerCalls: 1},
		{name: "applied rollback", phase: recovery.PhaseApplied, decision: recovery.DecisionRollback, expectedPhase: recovery.PhaseRolledBack, handlerCalls: 1},
		{name: "rollback required", phase: recovery.PhaseRollbackRequired, decision: recovery.DecisionRollback, expectedPhase: recovery.PhaseRolledBack, handlerCalls: 1},
		{name: "committed cleanup", phase: recovery.PhaseCommitted, expectedPhase: recovery.PhaseCommitted},
		{name: "rolled back cleanup", phase: recovery.PhaseRolledBack, expectedPhase: recovery.PhaseRolledBack},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := recovery.Record{SchemaVersion: 1, TransactionID: "tx-1", Operation: "fixture", Phase: test.phase, Metadata: json.RawMessage(`{"fixture":true}`)}
			data, err := json.Marshal(record)
			require.NoError(t, err)
			store := &memoryStore{data: data}
			calls := 0
			journal := recovery.NewJournal(store, map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
				calls++
				return test.decision, nil
			})})
			require.NoError(t, journal.Replay(context.Background()))
			require.True(t, store.removed)
			require.Equal(t, test.handlerCalls, calls)
			if test.handlerCalls != 0 {
				var saved recovery.Record
				require.NoError(t, json.Unmarshal(store.replaced[len(store.replaced)-1], &saved))
				require.Equal(t, test.expectedPhase, saved.Phase)
			} else {
				require.Empty(t, store.replaced)
			}
			require.NoError(t, journal.Replay(context.Background()))
		})
	}
}

func TestRollbackRequiredCannotBeCommitted(t *testing.T) {
	record := recovery.Record{SchemaVersion: 1, TransactionID: "tx-1", Operation: "fixture", Phase: recovery.PhaseRollbackRequired, Metadata: json.RawMessage(`{}`)}
	data, err := json.Marshal(record)
	require.NoError(t, err)
	store := &memoryStore{data: data}
	journal := recovery.NewJournal(store, map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
		return recovery.DecisionFinish, nil
	})})
	err = journal.Replay(context.Background())
	require.ErrorIs(t, err, recovery.ErrFailed)
	require.False(t, store.removed)
	require.Equal(t, data, store.data)
}

func TestJournalDurableTraceAndReplayIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	var trace []string
	store := recovery.NewFileStore(dir, func(step string) error {
		trace = append(trace, step)
		return nil
	})
	journal := recovery.NewJournal(store, map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
		return recovery.DecisionFinish, nil
	})})
	record := recovery.Record{SchemaVersion: 1, TransactionID: "tx-1", Operation: "fixture", Phase: recovery.PhasePrepared, Metadata: json.RawMessage(`{"fixture":true}`)}
	require.NoError(t, journal.Save(context.Background(), record))
	require.Equal(t, []string{"temp_create", "temp_write", "file_sync", "atomic_replace", "directory_sync"}, trace)
	trace = nil
	require.NoError(t, journal.Replay(context.Background()))
	_, err := os.Stat(filepath.Join(dir, recovery.JournalFilename))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Contains(t, trace, "directory_sync")
	require.NoError(t, journal.Replay(context.Background()))
}

func TestJournalRejectsMalformedUnsupportedAndOversizedRecords(t *testing.T) {
	valid := `{"schemaVersion":1,"transactionId":"tx-1","operation":"fixture","phase":"prepared","metadata":{}}`
	tests := []struct {
		name     string
		data     []byte
		expected error
	}{
		{name: "empty", data: nil, expected: recovery.ErrCorrupt},
		{name: "truncated", data: []byte(`{"schemaVersion":1`), expected: recovery.ErrCorrupt},
		{name: "trailing json", data: []byte(valid + `{}`), expected: recovery.ErrCorrupt},
		{name: "invalid utf8", data: append([]byte(valid[:len(valid)-1]), 0xff), expected: recovery.ErrCorrupt},
		{name: "unknown field", data: []byte(`{"schemaVersion":1,"transactionId":"tx-1","operation":"fixture","phase":"prepared","metadata":{},"extra":true}`), expected: recovery.ErrCorrupt},
		{name: "unknown schema", data: []byte(`{"schemaVersion":2,"transactionId":"tx-1","operation":"fixture","phase":"prepared","metadata":{}}`), expected: recovery.ErrUnsupported},
		{name: "unknown phase", data: []byte(`{"schemaVersion":1,"transactionId":"tx-1","operation":"fixture","phase":"future","metadata":{}}`), expected: recovery.ErrUnsupported},
		{name: "unknown operation", data: []byte(`{"schemaVersion":1,"transactionId":"tx-1","operation":"future","phase":"prepared","metadata":{}}`), expected: recovery.ErrUnsupported},
		{name: "missing metadata", data: []byte(`{"schemaVersion":1,"transactionId":"tx-1","operation":"fixture","phase":"prepared","metadata":null}`), expected: recovery.ErrCorrupt},
		{name: "unstable identifier", data: []byte(`{"schemaVersion":1,"transactionId":" tx ","operation":"fixture","phase":"prepared","metadata":{}}`), expected: recovery.ErrCorrupt},
		{name: "oversized", data: make([]byte, recovery.MaxRecordSize+1), expected: recovery.ErrCorrupt},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, recovery.JournalFilename)
			require.NoError(t, os.WriteFile(path, test.data, 0o600))
			journal := recovery.NewJournal(recovery.NewFileStore(dir, nil), map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
				return recovery.DecisionFinish, nil
			})})
			err := journal.Replay(context.Background())
			require.ErrorIs(t, err, test.expected)
			_, statErr := os.Stat(path)
			require.NoError(t, statErr)
		})
	}
}

func TestHandlerFailurePreservesCanonicalEvidence(t *testing.T) {
	dir := t.TempDir()
	store := recovery.NewFileStore(dir, nil)
	journal := recovery.NewJournal(store, map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
		return 0, errors.New("handler failed")
	})})
	require.NoError(t, journal.Save(context.Background(), recovery.Record{SchemaVersion: 1, TransactionID: "tx-1", Operation: "fixture", Phase: recovery.PhaseApplied, Metadata: json.RawMessage(`{}`)}))
	err := journal.Replay(context.Background())
	require.ErrorIs(t, err, recovery.ErrFailed)
	_, statErr := os.Stat(filepath.Join(dir, recovery.JournalFilename))
	require.NoError(t, statErr)
}

func TestDurableStoreFaultsDoNotReportSuccess(t *testing.T) {
	steps := []string{"temp_create", "temp_write", "file_sync", "atomic_replace", "directory_sync"}
	for _, fail := range steps {
		t.Run(fail, func(t *testing.T) {
			injected := errors.New("fault")
			store := recovery.NewFileStore(t.TempDir(), func(step string) error {
				if step == fail {
					return injected
				}
				return nil
			})
			journal := recovery.NewJournal(store, nil)
			err := journal.Save(context.Background(), recovery.Record{SchemaVersion: 1, TransactionID: "tx", Operation: "fixture", Phase: recovery.PhasePrepared, Metadata: json.RawMessage(`{}`)})
			require.ErrorIs(t, err, recovery.ErrFailed)
			require.ErrorIs(t, err, injected)
		})
	}
}

func TestDurableRemoveFailureLeavesRecoveryEvidence(t *testing.T) {
	dir := t.TempDir()
	directorySyncs := 0
	injected := errors.New("remove sync fault")
	store := recovery.NewFileStore(dir, func(step string) error {
		if step == "directory_sync" {
			directorySyncs++
			if directorySyncs == 3 {
				return injected
			}
		}
		return nil
	})
	journal := recovery.NewJournal(store, map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
		return recovery.DecisionFinish, nil
	})})
	require.NoError(t, journal.Save(context.Background(), recovery.Record{SchemaVersion: 1, TransactionID: "tx", Operation: "fixture", Phase: recovery.PhasePrepared, Metadata: json.RawMessage(`{}`)}))
	err := journal.Replay(context.Background())
	require.ErrorIs(t, err, recovery.ErrFailed)
	require.ErrorIs(t, err, injected)
	matches, globErr := filepath.Glob(filepath.Join(dir, ".journal*"))
	require.NoError(t, globErr)
	require.NotEmpty(t, matches)
	require.NoError(t, journal.Replay(context.Background()))
}
