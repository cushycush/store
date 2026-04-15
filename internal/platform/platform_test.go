package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	info := Detect()

	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
	if info.Hostname == "" {
		t.Error("Hostname is empty")
	}
}

func TestDetectDistro(t *testing.T) {
	info := Detect()

	switch runtime.GOOS {
	case "linux":
		if info.Distro == "" {
			t.Log("Distro is empty (no /etc/os-release?)")
		}
	case "darwin":
		if info.Distro != "macos" {
			t.Errorf("Distro = %q, want macos", info.Distro)
		}
	case "windows":
		if info.Distro != "windows" {
			t.Errorf("Distro = %q, want windows", info.Distro)
		}
	}
}

func TestDetectShell(t *testing.T) {
	t.Run("SHELL set", func(t *testing.T) {
		t.Setenv("SHELL", "/usr/bin/zsh")
		got := detectShell()
		if got != "zsh" {
			t.Errorf("detectShell() = %q, want zsh", got)
		}
	})

	t.Run("SHELL empty", func(t *testing.T) {
		t.Setenv("SHELL", "")
		got := detectShell()
		if runtime.GOOS != "windows" && got != "" {
			t.Errorf("detectShell() = %q, want empty", got)
		}
	})
}

func TestDetectShellFromEnv(t *testing.T) {
	tests := []struct {
		name string
		goos string
		env  map[string]string
		want string
	}{
		{
			name: "SHELL wins on windows",
			goos: "windows",
			env:  map[string]string{"SHELL": "/usr/bin/bash", "PSModulePath": "anything", "ComSpec": `C:\Windows\System32\cmd.exe`},
			want: "bash",
		},
		{
			name: "windows PSModulePath fallback",
			goos: "windows",
			env:  map[string]string{"PSModulePath": `C:\Users\x\Modules`, "ComSpec": `C:\Windows\System32\cmd.exe`},
			want: "powershell",
		},
		{
			name: "windows ComSpec fallback",
			goos: "windows",
			env:  map[string]string{"ComSpec": `C:\Windows\System32\cmd.exe`},
			want: "cmd",
		},
		{
			name: "windows all empty",
			goos: "windows",
			env:  map[string]string{},
			want: "",
		},
		{
			name: "linux SHELL unset stays empty",
			goos: "linux",
			env:  map[string]string{"PSModulePath": "ignored", "ComSpec": "ignored"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }
			if got := detectShellFromEnv(tt.goos, getenv); got != tt.want {
				t.Errorf("detectShellFromEnv(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
	info := Info{
		OS:            "linux",
		Arch:          "arm64",
		Distro:        "ubuntu",
		DistroVersion: "24.04",
		Hostname:      "myhost",
		WSL:           true,
		Shell:         "fish",
	}

	vars := info.EnvVars()
	expected := map[string]string{
		"STORE_OS":             "linux",
		"STORE_ARCH":           "arm64",
		"STORE_DISTRO":         "ubuntu",
		"STORE_DISTRO_VERSION": "24.04",
		"STORE_HOSTNAME":       "myhost",
		"STORE_WSL":            "true",
		"STORE_SHELL":          "fish",
	}

	got := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			got[parts[0]] = parts[1]
		}
	}

	for k, want := range expected {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}

func TestEnvVarsWSLFalse(t *testing.T) {
	info := Info{WSL: false}
	vars := info.EnvVars()
	for _, v := range vars {
		if strings.HasPrefix(v, "STORE_WSL=") {
			if v != "STORE_WSL=false" {
				t.Errorf("got %q, want STORE_WSL=false", v)
			}
			return
		}
	}
	t.Error("STORE_WSL not found in env vars")
}

func TestParseOSRelease(t *testing.T) {
	t.Run("standard format", func(t *testing.T) {
		content := `NAME="Ubuntu"
ID=ubuntu
VERSION_ID="24.04"
PRETTY_NAME="Ubuntu 24.04 LTS"
`
		path := filepath.Join(t.TempDir(), "os-release")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		id, version := parseOSRelease(path)
		if id != "ubuntu" {
			t.Errorf("ID = %q, want ubuntu", id)
		}
		if version != "24.04" {
			t.Errorf("VERSION_ID = %q, want 24.04", version)
		}
	})

	t.Run("unquoted values", func(t *testing.T) {
		content := `ID=arch
VERSION_ID=rolling
`
		path := filepath.Join(t.TempDir(), "os-release")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		id, version := parseOSRelease(path)
		if id != "arch" {
			t.Errorf("ID = %q, want arch", id)
		}
		if version != "rolling" {
			t.Errorf("VERSION_ID = %q, want rolling", version)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		id, version := parseOSRelease("/nonexistent/os-release")
		if id != "" || version != "" {
			t.Errorf("expected empty strings, got %q %q", id, version)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		content := `NAME="Something"
PRETTY_NAME="Something Nice"
`
		path := filepath.Join(t.TempDir(), "os-release")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		id, version := parseOSRelease(path)
		if id != "" || version != "" {
			t.Errorf("expected empty strings, got %q %q", id, version)
		}
	})
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`"ubuntu"`, "ubuntu"},
		{`ubuntu`, "ubuntu"},
		{`"24.04"`, "24.04"},
		{`""`, ""},
		{`"`, `"`},
		{"", ""},
	}
	for _, tt := range tests {
		if got := unquote(tt.in); got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
