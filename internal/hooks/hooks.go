package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RunGlobal executes a global hook script at .store/hooks/<hookName> if it
// exists and is executable. Returns nil if the hook does not exist.
func RunGlobal(root, hookName, action string) error {
	path := filepath.Join(root, ".store", "hooks", hookName)

	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("hook %q: %w", hookName, err)
	}

	if fi.Mode()&0o111 == 0 {
		return nil // not executable, skip
	}

	cmd := exec.Command(path)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"STORE_ROOT="+root,
		"STORE_ACTION="+action,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %q failed: %w", hookName, err)
	}

	return nil
}

// RunEntry executes a per-store hook command (pre or post) via "sh -c"
// Returns nil if hooks is nil or the requested phase is empty.
func RunEntry(root, name, target, action, phase string, h *HookEntry) error {
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

	cmd := exec.Command("sh", "-c", hookCmd)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"STORE_ROOT="+root,
		"STORE_NAME="+name,
		"STORE_TARGET="+target,
		"STORE_ACTION="+action,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s (%s) failed: %w", phase, name, err)
	}

	return nil
}
