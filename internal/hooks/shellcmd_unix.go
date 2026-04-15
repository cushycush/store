//go:build !windows

package hooks

import "os/exec"

// buildShellCmd runs a per-store hook command under `sh -c`.
func buildShellCmd(hookCmd string) *exec.Cmd {
	return exec.Command("sh", "-c", hookCmd)
}
