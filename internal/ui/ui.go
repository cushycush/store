package ui

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

var enabled = true

func init() {
	if os.Getenv("NO_COLOR") != "" {
		enabled = false
		return
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		enabled = false
	}
}

func wrap(code, s string) string {
	if !enabled {
		return s
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", code, s)
}

func Bold(s string) string   { return wrap("1", s) }
func Dim(s string) string    { return wrap("2", s) }
func Italic(s string) string { return wrap("3", s) }
func Red(s string) string    { return wrap("31", s) }
func Green(s string) string  { return wrap("32", s) }
func Yellow(s string) string { return wrap("33", s) }
func Cyan(s string) string   { return wrap("36", s) }

func BoldRed(s string) string    { return wrap("1;31", s) }
func BoldGreen(s string) string  { return wrap("1;32", s) }
func BoldYellow(s string) string { return wrap("1;33", s) }
func BoldCyan(s string) string   { return wrap("1;36", s) }

func Success(s string) string { return Green("✓ " + s) }
func Warning(s string) string { return Yellow("⚠ " + s) }
func Error(s string) string   { return BoldRed("✗ " + s) }

func Arrow() string {
	if !enabled {
		return "->"
	}
	return Dim("→")
}

func StatusLinked() string   { return Green("[linked]") }
func StatusMissing() string  { return Cyan("[missing]") }
func StatusConflict() string { return Red("[conflict]") }
func StatusBroken() string   { return Yellow("[broken]") }

func DiffOK() string       { return Green("[ok]") }
func DiffCreate() string   { return Cyan("[create]") }
func DiffConflict() string { return Red("[conflict]") }
func DiffReplace() string  { return Yellow("[replace]") }
func DiffError() string    { return BoldRed("[error]") }

func DoctorOK() string    { return Green("[ok]") }
func DoctorWarn() string  { return Yellow("[warn]") }
func DoctorError() string { return Red("[error]") }
func DoctorInfo() string  { return Cyan("[info]") }

func StoreName(s string) string  { return Bold(s) }
func TargetPath(s string) string { return Dim(s) }
func FileName(s string) string   { return s }

func Prompt(question string) string {
	return Bold(question) + " " + Cyan("?") + " "
}
