//go:build unix

package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestThreadRunContextCancellationKillsProcessGroup(t *testing.T) {
	t.Parallel()

	childPIDFile := filepath.Join(t.TempDir(), "child.pid")
	client, _ := newTestClientWithEnv(t, "spawn-child-wait", map[string]string{
		"CODEX_TEST_CHILD_PID_FILE": childPIDFile,
	})
	thread := client.StartThread(ThreadOptions{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := thread.Run(ctx, TextInput("start child"))
		done <- err
	}()

	childPID := waitForChildPID(t, childPIDFile)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled run")
	}

	waitForProcessExit(t, childPID)
}

func waitForChildPID(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(content))) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
			if err != nil {
				t.Fatalf("strconv.Atoi() error = %v", err)
			}
			return pid
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for child pid file %s", path)
	return 0
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("process %d is still alive after cancellation", pid)
}
