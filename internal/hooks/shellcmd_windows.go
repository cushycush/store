//go:build windows

package hooks

import (
	"os/exec"
	"syscall"
)

// buildShellCmd runs a per-store hook command under cmd.exe on Windows.
// It bypasses Go's default Windows argument escaping via SysProcAttr.CmdLine
// so that quotes the user wrote in their hook command reach cmd.exe
// verbatim — otherwise `set > "%STORE_ROOT%\foo"` becomes unusable after Go
// rewrites the inner quotes as `\"`.
func buildShellCmd(hookCmd string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `/C ` + hookCmd}
	return cmd
}
