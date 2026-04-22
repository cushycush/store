// Package ui wraps store-core's CLI styling with store-specific chips
// (StoreName, StatusLinked, DiffCreate, etc.) while re-exporting the generic
// primitives under their original names so existing call sites are
// unchanged.
package ui

import coreui "github.com/cushycush/store-core/ui"

// Generic styling primitives — re-exported from store-core/ui so call sites
// in cmd/store/ keep their existing import path.
var (
	Bold    = coreui.Bold
	Dim     = coreui.Dim
	Italic  = coreui.Italic
	Red     = coreui.Red
	Green   = coreui.Green
	Yellow  = coreui.Yellow
	Cyan    = coreui.Cyan
	BoldRed    = coreui.BoldRed
	BoldGreen  = coreui.BoldGreen
	BoldYellow = coreui.BoldYellow
	BoldCyan   = coreui.BoldCyan

	Success = coreui.Success
	Warning = coreui.Warning
	Error   = coreui.Error

	Arrow  = coreui.Arrow
	Prompt = coreui.Prompt

	DoctorOK    = coreui.DoctorOK
	DoctorWarn  = coreui.DoctorWarn
	DoctorError = coreui.DoctorError
	DoctorInfo  = coreui.DoctorInfo
)

// Store-ledger chips. These are store-specific because they describe the
// state of a symlink target — stock doesn't have an equivalent concept.
func StatusLinked() string   { return Green("[linked]") }
func StatusMissing() string  { return Cyan("[missing]") }
func StatusConflict() string { return Red("[conflict]") }
func StatusBroken() string   { return Yellow("[broken]") }
func StatusDrift() string    { return Yellow("[drift]") }

// Diff chips for `store diff` output.
func DiffOK() string       { return Green("[ok]") }
func DiffCreate() string   { return Cyan("[create]") }
func DiffConflict() string { return Red("[conflict]") }
func DiffReplace() string  { return Yellow("[replace]") }
func DiffError() string    { return BoldRed("[error]") }

// Typographic helpers for store entities.
func StoreName(s string) string  { return Bold(s) }
func TargetPath(s string) string { return Dim(s) }
func FileName(s string) string   { return s }
