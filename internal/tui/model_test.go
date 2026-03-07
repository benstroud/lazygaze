package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestResetForNewStream(t *testing.T) {
	cancelled := false
	m := NewEmpty("sonnet", nil)
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
	m := NewEmpty("sonnet", nil)
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
	m := NewEmpty("sonnet", nil)
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
	m := NewEmpty("sonnet", nil)
	m.mode = modeTilde
	m.tildeInput.SetValue("0")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err == nil {
		t.Fatal("expected error for n=0")
	}
}

func TestHandleTildeInput_NegativeN(t *testing.T) {
	m := NewEmpty("sonnet", nil)
	m.mode = modeTilde
	m.tildeInput.SetValue("-1")

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEnter})
	model := result.(Model)

	if model.err == nil {
		t.Fatal("expected error for n=-1")
	}
}

func TestHandleTildeInput_EmptyInput(t *testing.T) {
	m := NewEmpty("sonnet", nil)
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
	m := NewEmpty("sonnet", nil)
	m.mode = modeTilde

	result, _ := m.handleTildeInput(tea.KeyMsg{Type: tea.KeyEscape})
	model := result.(Model)

	if model.mode != modeNormal {
		t.Error("esc should return to normal mode")
	}
}

func TestStagedKeybinding(t *testing.T) {
	m := NewEmpty("sonnet", nil)
	// Set up a pre-existing stream cancel to verify it gets called.
	streamCancelled := false
	m.cancelStream = func() { streamCancelled = true }

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
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
	m := NewEmpty("opus", nil)
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
