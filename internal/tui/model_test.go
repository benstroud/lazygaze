package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/benstroud/lazygaze/internal/claude"
	"github.com/benstroud/lazygaze/internal/harness"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestResetForNewStream(t *testing.T) {
	cancelled := false
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.cancelStream = func() { cancelled = true }
	m.err = fmt.Errorf("old error")
	m.done = true
	m.streaming = true
	m.reviewContent.WriteString("old content")
	oldGen := m.streamGen

	m, ctx := resetForNewStream(m)

	if ctx == nil {
		t.Error("returned context should not be nil")
	}
	if m.cancelDiffFetch == nil {
		t.Fatal("cancelDiffFetch should be set after reset")
	}
	// Verify calling cancel actually cancels the context.
	m.cancelDiffFetch()
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Error("calling cancelDiffFetch should cancel the returned context")
	}
	if !cancelled {
		t.Error("cancelStream should have been called")
	}
	if m.cancelStream != nil {
		t.Error("cancelStream should be nil after reset")
	}
	if m.streaming {
		t.Error("streaming should be false")
	}
	if m.err != nil {
		t.Error("err should be nil")
	}
	if m.done {
		t.Error("done should be false")
	}
	if !m.autoScroll {
		t.Error("autoScroll should be true")
	}
	if m.streamGen != oldGen+1 {
		t.Errorf("streamGen = %d, want %d", m.streamGen, oldGen+1)
	}
	if m.reviewContent.Len() != 0 {
		t.Error("reviewContent should be empty")
	}
}

func TestHandleTildeInput_ValidN(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde
	m.tildeInput.SetValue("3")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.gitRange != "HEAD~3..HEAD" {
		t.Errorf("gitRange = %q, want %q", model.gitRange, "HEAD~3..HEAD")
	}
	if model.mode != modeNormal {
		t.Error("mode should return to normal")
	}
}

func TestHandleTildeInput_InvalidN(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde
	m.tildeInput.SetValue("abc")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err == nil {
		t.Fatal("expected error for invalid input")
	}
	if model.mode != modeNormal {
		t.Error("mode should return to normal")
	}
}

func TestHandleTildeInput_ZeroN(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde
	m.tildeInput.SetValue("0")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err == nil {
		t.Fatal("expected error for n=0")
	}
}

func TestHandleTildeInput_NegativeN(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde
	m.tildeInput.SetValue("-1")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err == nil {
		t.Fatal("expected error for n=-1")
	}
}

func TestHandleTildeInput_EmptyInput(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde
	m.tildeInput.SetValue("")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err != nil {
		t.Errorf("empty input should not produce error, got %v", model.err)
	}
	if model.mode != modeNormal {
		t.Error("mode should return to normal")
	}
}

func TestHandleTildeInput_Escape(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.mode = modeTilde

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	if model.mode != modeNormal {
		t.Error("esc should return to normal mode")
	}
}

func TestStagedKeybinding(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	// Set up a pre-existing stream cancel to verify it gets called.
	streamCancelled := false
	m.cancelStream = func() { streamCancelled = true }

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	model := result.(Model)

	if model.diffSrc != diffSourceStaged {
		t.Errorf("diffSrc = %d, want diffSourceStaged (%d)", model.diffSrc, diffSourceStaged)
	}
	if cmd == nil {
		t.Error("expected a command to fetch staged diff")
	}
	if !streamCancelled {
		t.Error("pre-existing cancelStream should have been called")
	}
	if model.cancelStream != nil {
		t.Error("cancelStream should be nil after reset")
	}
	if model.cancelDiffFetch == nil {
		t.Error("cancelDiffFetch should be set")
	}
}

func TestNewEmpty_Defaults(t *testing.T) {
	m := NewEmpty("opus", nil, claude.New("opus"), []harness.Harness{claude.New("opus")})
	if m.modelName != "opus" {
		t.Errorf("modelName = %q, want %q", m.modelName, "opus")
	}
	if m.prompt == "" {
		t.Error("default prompt should not be empty")
	}
	if !m.autoScroll {
		t.Error("autoScroll should default to true")
	}
	if m.reviewContent == nil {
		t.Error("reviewContent should be initialized")
	}
}

func TestFooterHints_Adaptive(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet"), claude.New("opus")})
	m.width = 220
	m.diffContent = "diff"
	got := m.footerContent()

	required := []string{"[tab] switch pane", "[j/k] scroll", "[?/f1] help", "[:] git range"}
	for _, s := range required {
		if strings.Contains(got, s) {
			continue
		}
		t.Fatalf("expected footer to include %q, got: %s", s, got)
	}

	if !strings.Contains(got, "[q] quit") {
		t.Fatalf("expected footer to include quit hint")
	}

	m.width = 40
	got = m.footerContent()
	if lipgloss.Width(got) > m.width {
		t.Fatalf("footer should fit in available width (%d): %q", m.width, got)
	}

	m.mode = modeHelp
	got = m.footerContent()
	if !strings.Contains(got, "close") {
		t.Fatalf("expected help-mode footer to include close guidance, got: %q", got)
	}
}

func TestRenderHelp_Comprehensive(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet"), claude.New("opus")})
	help := m.renderHelp()

	required := []string{
		"[tab] switch focused pane",
		"[j/k/down/up] scroll focused pane",
		"[z] zoom",
		"[?/f1] open this cheat sheet",
		"[/] set a custom prompt",
		"[L] open prompt library",
		"[P] choose review persona",
		"[m] cycle review model",
		"[c] copy focused pane",
		"[H] choose review harness",
		"[:] set an arbitrary git range",
		"[~] open HEAD~n..HEAD picker",
		"[^] review last commit",
		"[S] review staged changes",
		"[D] review uncommitted changes",
		"[U] review upstream diff",
		"[r] refresh current diff",
		"git range (:): [enter] fetch | [esc] cancel",
		"library/persona/harness: [j/k/down/up] navigate | [enter] select | [esc] cancel",
		"help mode: [j/k/down/up] scroll | [esc/q/ctrl+c/?/f1] close",
		"[q] / [ctrl+c] quit",
	}
	for _, s := range required {
		if !strings.Contains(help, s) {
			t.Fatalf("help text missing %q\nhelp:\n%q", s, help)
		}
	}
}

func TestHelpKeybinding_OpensHelpMode(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.width = 120
	m.height = 40
	m.resizeViewports()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model := result.(Model)
	if model.mode != modeHelp {
		t.Fatalf("expected modeHelp after '?', got %v", model.mode)
	}
}

func TestHelpF1Keybinding_OpensHelpMode(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.width = 120
	m.height = 40
	m.resizeViewports()

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	model := result.(Model)
	if model.mode != modeHelp {
		t.Fatalf("expected modeHelp after F1, got %v", model.mode)
	}
}

func TestHelpMode_RendersLegendInsteadOfPlaceholder(t *testing.T) {
	m := NewEmpty("sonnet", nil, claude.New("sonnet"), []harness.Harness{claude.New("sonnet")})
	m.width = 120
	m.height = 40
	m.mode = modeHelp
	m.focusedPane = 1
	m.resizeViewports()
	m.reviewViewport.SetContent(m.renderHelp())

	view := m.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Fatalf("expected help legend in view, got: %q", view)
	}
	if strings.Contains(view, "The LLM output will appear here") {
		t.Fatalf("expected help mode to suppress empty review placeholder")
	}
}
