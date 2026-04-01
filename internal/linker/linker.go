package linker

import (
	"fmt"
	"os"
	"path/filepath"
)

// Status represents the state of a store's symlink.
type Status int

const (
	// StatusLinked means the symlink exists and points to the correct source.
	StatusLinked Status = iota
	// StatusMissing means no symlink exists at the target path.
	StatusMissing
	// StatusConflict means something exists at the target but it's not our symlink.
	StatusConflict
	// StatusBroken means a symlink exists but points to a nonexistent path.
	StatusBroken
)

func (s Status) String() string {
	switch s {
	case StatusLinked:
		return "linked"
	case StatusMissing:
		return "missing"
	case StatusConflict:
		return "conflict"
	case StatusBroken:
		return "broken"
	default:
		return "unknown"
	}
}

// Check examines the target path and returns the symlink status relative to the source.
func Check(source, target string) (Status, error) {
	fi, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return StatusMissing, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to stat %s: %w", target, err)
	}

	// Something exists at target - is it a symlink?
	if fi.Mode()&os.ModeSymlink == 0 {
		return StatusConflict, nil
	}

	// It's a symlink - does it point to our source?
	dest, err := os.Readlink(target)
	if err != nil {
		return 0, fmt.Errorf("failed to read symlink %s: %w", target, err)
	}

	// Resolve to absolute paths for comparison.
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve symlink destination: %w", err)
	}

	absSource, err := filepath.Abs(source)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve source path: %w", err)
	}

	if absDest != absSource {
		// Symlink exists but points elsewhere - check if it's broken.
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return StatusBroken, nil
		}
		return StatusConflict, nil
	}

	// Check if the source actually exists.
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return StatusBroken, nil
	}

	return StatusLinked, nil
}

// Link creates a symlink at target pointing to source.
// It creates parent directories as needed.
// Returns an error if something already exists at the target path.
func Link(source, target string) error {
	status, err := Check(source, target)
	if err != nil {
		return err
	}

	switch status {
	case StatusLinked:
		return nil // Already correctly linked.
	case StatusConflict:
		return fmt.Errorf("conflict: %s already exists and is not a symlink managed by store", target)
	case StatusBroken:
		// Remove the broken symlink and re-create.
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove broken symlink %s: %w", target, err)
		}
	case StatusMissing:
		// Good, proceed to create.
	}

	// Ensure parent directory exists.
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parent, err)
	}

	// Resolve source to absolute path for the symlink.
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}

	if err := os.Symlink(absSource, target); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %w", target, absSource, err)
	}

	return nil
}

// Unlink removes the symlink at target, but only if it points to source.
func Unlink(source, target string) error {
	status, err := Check(source, target)
	if err != nil {
		return err
	}

	switch status {
	case StatusMissing:
		return nil // Nothing to do.
	case StatusConflict:
		return fmt.Errorf("refusing to remove %s: not a symlink managed by store", target)
	case StatusBroken:
		// Still remove broken symlinks if they were ours (best effort).
	case StatusLinked:
		// Good, proceed.
	}

	if err := os.Remove(target); err != nil {
		return fmt.Errorf("failed to remove symlink %s: %w", target, err)
	}

	return nil
}
