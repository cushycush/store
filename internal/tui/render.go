package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Rule renders a horizontal rule with an embedded title and optional
// right-aligned status word. The title sits inside the rule itself, offset
// a few chars from the left:
//
//	─── tmux ─────────────────────────────────────────── missing ─
//
// The typographic move that gives the UI its signature look.
func Rule(width int, title, rightLabel string, rightColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}

	leftDashes := StyleDim.Render(strings.Repeat("─", 3))
	titleMid := ""
	if title != "" {
		titleMid = " " + StyleFg.Render(title) + " "
	}
	rightPart := ""
	if rightLabel != "" {
		c := StyleEmber
		if rightColor != "" {
			c = lipgloss.NewStyle().Foreground(rightColor)
		}
		rightPart = " " + c.Render(rightLabel) + " " + StyleDim.Render("─")
	}

	used := lipgloss.Width(leftDashes) + lipgloss.Width(titleMid) + lipgloss.Width(rightPart)
	mid := width - used
	if mid < 3 {
		mid = 3
	}
	middleDashes := StyleDim.Render(strings.Repeat("─", mid))
	return leftDashes + titleMid + middleDashes + rightPart
}

// PadRight right-pads s with spaces so its printable width equals width.
func PadRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// Clip truncates s to at most width printable columns, adding an ellipsis
// glyph if truncation occurred.
func Clip(s string, width int) string {
	w := lipgloss.Width(s)
	if w <= width || width < 1 {
		return s
	}
	if width < 2 {
		return string([]rune(s)[:1])
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// IndentBlock indents every line of s with n spaces.
func IndentBlock(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}
