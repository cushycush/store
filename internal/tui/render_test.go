package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRuleHasExactWidth(t *testing.T) {
	got := Rule(50, "tmux", "missing", ColorPartial)
	if w := lipgloss.Width(got); w != 50 {
		t.Errorf("rule width = %d, want 50 (got %q)", w, got)
	}
}

func TestRuleWithoutTitleOrRight(t *testing.T) {
	got := Rule(30, "", "", "")
	if w := lipgloss.Width(got); w != 30 {
		t.Errorf("plain rule width = %d, want 30", w)
	}
	// Should be all dashes.
	if strings.ContainsAny(stripANSI(got), " ") {
		t.Errorf("plain rule should be dashes only; got %q", stripANSI(got))
	}
}

func TestClipTruncatesWithEllipsis(t *testing.T) {
	got := Clip("the quick brown fox", 10)
	if lipgloss.Width(got) > 10 {
		t.Errorf("Clip produced width %d > 10: %q", lipgloss.Width(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("Clip should end with ellipsis; got %q", got)
	}
}

func TestStripANSI(t *testing.T) {
	styled := StyleEmber.Render("hello")
	if stripANSI(styled) != "hello" {
		t.Errorf("stripANSI(%q) = %q, want \"hello\"", styled, stripANSI(styled))
	}
}
