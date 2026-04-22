package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// Git-style subcommand dispatch: when the first arg isn't one of store's own
// subcommands, try to execute it as an external binary. Two lookup paths:
//
//  1. `store-<sub>` on $PATH (strict git convention; anything installed under
//     this prefix is implicitly opting in to being a store subcommand).
//  2. For known companions — currently just `stock` — also accept the bare
//     binary name, since those tools ship under their own name. This means
//     `store stock doctor` works out of the box after
//     `go install …/stock/cmd/stock@latest`, without a symlink shim.
//
// Unknown args that match neither path fall through to cobra, which reports
// "unknown command" with its normal suggestion machinery.

// companions are tools known to ship under their own binary name. The map
// value is reserved for future metadata; right now the key set is all that
// matters.
var companions = map[string]struct{}{
	"stock": {},
}

var knownSubsOnce = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, c := range newRootCmd().Commands() {
		out[c.Name()] = true
		for _, a := range c.Aliases {
			out[a] = true
		}
	}
	return out
})

// resolveCompanion returns an absolute executable path if args[0] should be
// delegated to an external binary; otherwise ("", false) and cobra handles
// the args as normal.
func resolveCompanion(sub string) (string, bool) {
	if sub == "" || sub[0] == '-' {
		return "", false // flag or empty; cobra handles
	}
	if knownSubsOnce()[sub] {
		return "", false // cobra owns this one
	}
	if path, err := exec.LookPath("store-" + sub); err == nil {
		return path, true
	}
	if _, ok := companions[sub]; ok {
		if path, err := exec.LookPath(sub); err == nil {
			return path, true
		}
	}
	return "", false
}

// runCompanion execs path with args, piping stdio and propagating exit code.
// Never returns on success — it os.Exit()s with the child's exit code.
func runCompanion(path string, args []string) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "store: run %s: %s\n", path, err)
		os.Exit(1)
	}
	os.Exit(0)
}
