package hooks

import (
	"fmt"
	"os"

	"github.com/cushycush/store/v2/internal/config"

	corehooks "github.com/cushycush/store-core/hooks"
)

// RunGlobal executes a global hook script at .store/hooks/<hookName>. See
// store-core/hooks for the full dispatch rules.
func RunGlobal(root, hookName, action string) error {
	return corehooks.RunGlobal(root, hookName, action)
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
	cmd.Env = corehooks.Env(root, action,
		"STORE_NAME="+name,
		"STORE_TARGET="+target,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s (%s) failed: %w", phase, name, err)
	}
	return nil
}
