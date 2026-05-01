package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// checkWindowsShimCollision prints a one-time hint when this binary is being
// shadowed on PATH by the Microsoft Store CLI stub. The stub lives at
// %LocalAppData%\Microsoft\WindowsApps\store.exe and is auto-prepended to
// PATH, so typing `store` resolves to the stub instead of this binary even
// after `go install`. The hint points the user at the PowerShell wrapper
// documented in the README.
//
// The hint prints once per machine; a marker file under the user's config
// directory suppresses it on subsequent runs. Delete the marker to see it
// again. No-op on non-Windows.
func checkWindowsShimCollision() {
	if runtime.GOOS != "windows" {
		return
	}
	marker, err := collisionMarkerPath()
	if err != nil {
		return
	}
	if _, err := os.Stat(marker); err == nil {
		return
	}
	out, err := exec.Command("where.exe", "store").Output()
	if err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if !shimShadowsBinary(string(out), exe) {
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  note: `store` on your PATH resolves to the Microsoft Store CLI shim,")
	fmt.Fprintln(os.Stderr, "  not this binary. To make `store` always invoke this tool, add to your")
	fmt.Fprintln(os.Stderr, "  PowerShell $PROFILE:")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, `    function store { & "$HOME\go\bin\store.exe" @args }`)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Showing this once. See README install section for details.")
	fmt.Fprintln(os.Stderr)

	_ = os.MkdirAll(filepath.Dir(marker), 0o755)
	_ = os.WriteFile(marker, []byte("shown\n"), 0o644)
}

// shimShadowsBinary parses `where.exe store` output and returns true when
// the first PATH match is a WindowsApps stub that is not this binary. The
// parser is split out from the orchestration so it stays testable on every
// platform.
func shimShadowsBinary(whereOut, exePath string) bool {
	for _, line := range strings.Split(whereOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, exePath) {
			return false
		}
		return strings.Contains(strings.ToLower(line), `\windowsapps\`)
	}
	return false
}

func collisionMarkerPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "store", "windows-shim-hint-shown"), nil
}
