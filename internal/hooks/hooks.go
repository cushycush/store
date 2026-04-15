package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/platform"
)

// RunGlobal executes a global hook script at .store/hooks/<hookName> if it
// exists. Returns nil if the hook does not exist.
//
// On POSIX the script must have the executable bit set and is run directly
// (relying on its shebang). On Windows the executable bit is ignored and
// execution is dispatched by file extension: .ps1 scripts run under
// PowerShell, .cmd/.bat/.exe run directly, and anything else falls back to
// `sh -c <path>` (useful under Git Bash or WSL).
func RunGlobal(root, hookName, action string) error {
	path := filepath.Join(root, ".store", "hooks", hookName)

	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("hook %q: %w", hookName, err)
	}

	cmd, ok := buildGlobalHookCmd(path, fi)
	if !ok {
		return nil
	}
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = hookEnv(root, action)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %q failed: %w", hookName, err)
	}
	return nil
}

// RunEntry executes a per-store hook command (pre or post) via the platform's
// default shell — `cmd.exe /C` on Windows and `sh -c` elsewhere. Returns nil
// if hooks is nil or the requested phase is empty.
func RunEntry(root, name, target, action, phase string, h *config.HookEntry) error {
	if h == nil {
		return nil
	}

	var hookCmd string
	switch phase {
	case "pre":
		hookCmd = h.Pre
	case "post":
		hookCmd = h.Post
	default:
		return fmt.Errorf("unknown hook phase %q", phase)
	}

	if hookCmd == "" {
		return nil
	}

	cmd := buildShellCmd(hookCmd)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = hookEnv(root, action,
		"STORE_NAME="+name,
		"STORE_TARGET="+target,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s (%s) failed: %w", phase, name, err)
	}
	return nil
}

// buildShellCmd constructs the exec.Cmd that runs a hook command string under
// the platform's default shell.
func buildShellCmd(hookCmd string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe", "/C", hookCmd)
	}
	return exec.Command("sh", "-c", hookCmd)
}

// buildGlobalHookCmd decides how to execute a global hook script file. It
// returns (cmd, true) if the script should run, or (nil, false) if it should
// be silently skipped (e.g. POSIX file without the executable bit).
func buildGlobalHookCmd(path string, fi os.FileInfo) (*exec.Cmd, bool) {
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ps1":
			return exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path), true
		case ".cmd", ".bat", ".exe":
			return exec.Command(path), true
		default:
			// Likely a POSIX-style script with a shebang. `sh` is available
			// under Git Bash / WSL / MSYS; if it's missing the command will
			// error at run time and bubble up to the caller.
			return exec.Command("sh", path), true
		}
	}

	if fi.Mode()&0o111 == 0 {
		return nil, false // not executable, skip
	}
	return exec.Command(path), true
}

func hookEnv(root, action string, extra ...string) []string {
	env := append(os.Environ(),
		"STORE_ROOT="+root,
		"STORE_ACTION="+action,
	)
	env = append(env, extra...)
	return append(env, platform.Detect().EnvVars()...)
}
