//go:build windows

package recovery_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gist/backend/internal/desktop/recovery"
)

func TestCrashWriterHelper(t *testing.T) {
	if os.Getenv("GIST_RECOVERY_HELPER") != "1" {
		return
	}
	phase := os.Getenv("GIST_RECOVERY_PHASE")
	journal := recovery.NewJournal(recovery.NewFileStore(os.Getenv("GIST_RECOVERY_DIR"), nil), nil)
	require.NoError(t, journal.Save(context.Background(), recovery.Record{
		SchemaVersion: 1,
		TransactionID: "crash-tx",
		Operation:     "fixture",
		Phase:         phase,
		Metadata:      json.RawMessage(`{"material":"present"}`),
	}))
	fmt.Println("DURABLE")
	select {}
}

func TestCrashPhasesReplayBeforeDataFactory(t *testing.T) {
	tests := []struct {
		phase          string
		decision       recovery.Decision
		expectRecovery bool
	}{
		{phase: recovery.PhasePrepared, decision: recovery.DecisionFinish, expectRecovery: true},
		{phase: recovery.PhaseApplied, decision: recovery.DecisionFinish, expectRecovery: true},
		{phase: recovery.PhaseRollbackRequired, decision: recovery.DecisionRollback, expectRecovery: true},
		{phase: recovery.PhaseCommitted},
		{phase: recovery.PhaseRolledBack},
	}
	for _, test := range tests {
		t.Run(test.phase, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.Command(os.Args[0], "-test.run=TestCrashWriterHelper")
			cmd.Env = append(os.Environ(), "GIST_RECOVERY_HELPER=1", "GIST_RECOVERY_DIR="+dir, "GIST_RECOVERY_PHASE="+test.phase)
			stdout, err := cmd.StdoutPipe()
			require.NoError(t, err)
			require.NoError(t, cmd.Start())
			line, err := bufio.NewReader(stdout).ReadString('\n')
			require.NoError(t, err)
			require.Equal(t, "DURABLE", strings.TrimSpace(line))
			require.NoError(t, cmd.Process.Kill())
			_, _ = cmd.Process.Wait()

			var events []string
			journal := recovery.NewJournal(recovery.NewFileStore(dir, nil), map[string]recovery.Handler{"fixture": handlerFunc(func(context.Context, recovery.Record) (recovery.Decision, error) {
				events = append(events, "replay")
				return test.decision, nil
			})})
			require.NoError(t, journal.Replay(context.Background()))
			events = append(events, "db_factory")
			if test.expectRecovery {
				require.Equal(t, []string{"replay", "db_factory"}, events)
			} else {
				require.Equal(t, []string{"db_factory"}, events)
			}
			require.NoError(t, journal.Replay(context.Background()))
		})
	}
}
