package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Info holds detected platform information for the current machine.
type Info struct {
	OS            string // runtime.GOOS: linux, darwin, windows
	Arch          string // runtime.GOARCH: amd64, arm64, arm, etc.
	Distro        string // linux: /etc/os-release ID; darwin: "macos"; windows: "windows"
	DistroVersion string // linux: /etc/os-release VERSION_ID; darwin: SystemVersion plist
	Hostname      string // os.Hostname()
	WSL           bool   // true if running under Windows Subsystem for Linux
	Shell         string // basename of $SHELL (e.g. zsh, bash, fish, nu)
}

// Detect returns platform information for the current machine.
func Detect() Info {
	info := Info{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	info.Hostname, _ = os.Hostname()
	info.Shell = detectShell()

	switch runtime.GOOS {
	case "linux":
		info.Distro, info.DistroVersion = detectLinuxDistro()
		info.WSL = detectWSL()
	case "darwin":
		info.Distro = "macos"
		info.DistroVersion = detectMacOSVersion()
	case "windows":
		info.Distro = "windows"
	}

	return info
}

// EnvVars returns the platform info as environment variable key=value pairs
// suitable for passing to exec.Cmd.Env.
func (i Info) EnvVars() []string {
	wsl := "false"
	if i.WSL {
		wsl = "true"
	}
	return []string{
		"STORE_OS=" + i.OS,
		"STORE_ARCH=" + i.Arch,
		"STORE_DISTRO=" + i.Distro,
		"STORE_DISTRO_VERSION=" + i.DistroVersion,
		"STORE_HOSTNAME=" + i.Hostname,
		"STORE_WSL=" + wsl,
		"STORE_SHELL=" + i.Shell,
	}
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

func detectWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func detectLinuxDistro() (string, string) {
	return parseOSRelease("/etc/os-release")
}

// parseOSRelease extracts ID and VERSION_ID from an os-release file.
// Exported logic for testability via a path parameter.
func parseOSRelease(path string) (string, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}

	var id, versionID string
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "ID="):
			id = unquote(strings.TrimPrefix(line, "ID="))
		case strings.HasPrefix(line, "VERSION_ID="):
			versionID = unquote(strings.TrimPrefix(line, "VERSION_ID="))
		}
	}
	return id, versionID
}

func detectMacOSVersion() string {
	data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if strings.Contains(line, "ProductVersion") && i+1 < len(lines) {
			val := strings.TrimSpace(lines[i+1])
			val = strings.TrimPrefix(val, "<string>")
			val = strings.TrimSuffix(val, "</string>")
			return val
		}
	}
	return ""
}

// unquote strips surrounding double quotes if present.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
