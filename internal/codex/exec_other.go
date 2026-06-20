//go:build !unix

package codex

import "os/exec"

func prepareCommandForCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		return killProcess(cmd)
	}
}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
