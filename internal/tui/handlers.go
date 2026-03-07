package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleGitRangeInput processes keyboard input when the TUI is in git range input mode.
// It handles three cases: pressing Enter submits the range and fetches the diff, pressing
// Escape cancels input and returns to normal mode, and other keys update the input field.
// When a valid range is submitted, it transitions the model to fetch and display the diff.
func (m Model) handleGitRangeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.gitRangeInput.Value())
		m.mode = modeNormal
		m.gitRangeInput.Blur()
		if val == "" {
			return m, nil
		}
		var ctx context.Context
		m, ctx = resetForNewStream(m)
		m.diffSrc = diffSourceRange
		return m, fetchDiffCmd(ctx, val, m.diffFetchGen)
	case "esc":
		m.mode = modeNormal
		m.gitRangeInput.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.gitRangeInput, cmd = m.gitRangeInput.Update(msg)
		return m, cmd
	}
}

// handlePromptInput processes keyboard input when the TUI is in prompt input mode.
// It handles three cases: pressing Enter submits the prompt and initiates a review,
// pressing Escape cancels input and returns to normal mode, and other keys are
// forwarded to the prompt input component for text editing.
// If the prompt is empty or no diff content is loaded, an error is returned.
func (m Model) handlePromptInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.promptInput.Value())
		m.mode = modeNormal
		m.promptInput.Blur()
		if val == "" {
			return m, nil
		}
		if m.diffContent == "" {
			m.err = fmt.Errorf("no diff loaded — press : to set a git range first")
			return m, nil
		}
		m.prompt = val
		m.promptNoPersona = false
		m = resetForNewReview(m)
		return m, startStreamCmd(m.buildFullPrompt(), m.diffContent, m.modelName)
	case "esc":
		m.mode = modeNormal
		m.promptInput.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.promptInput, cmd = m.promptInput.Update(msg)
		return m, cmd
	}
}

// handleTildeInput processes keyboard input when the application is in tilde input mode.
// This mode allows users to enter a number (n) to view the last n commits as a diff.
//
// The function handles three cases:
//   - Enter: Validates the input as a positive integer and, if valid, sets up a git range
//     (HEAD~n..HEAD) and triggers a diff fetch for those commits.
//   - Escape: Cancels tilde input mode and returns to normal mode without fetching any diff.
//   - Other keys: Passes input to the tilde input field for text editing.
//
// Returns the updated model and any resulting command.
func (m Model) handleTildeInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.tildeInput.Value())
		m.mode = modeNormal
		m.tildeInput.Blur()
		if val == "" {
			return m, nil
		}
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 {
			m.err = fmt.Errorf("invalid number: %s (must be a positive integer)", val)
			return m, nil
		}
		m.gitRange = fmt.Sprintf("HEAD~%d..HEAD", n)
		var ctx context.Context
		m, ctx = resetForNewStream(m)
		m.diffSrc = diffSourceRange
		return m, fetchDiffCmd(ctx, m.gitRange, m.diffFetchGen)
	case "esc":
		m.mode = modeNormal
		m.tildeInput.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.tildeInput, cmd = m.tildeInput.Update(msg)
		return m, cmd
	}
}

// handleLibraryInput processes keyboard input events when the application is in
// library selection mode, allowing users to browse and select prompt templates.
// It handles navigation through the available prompts using up/down keys (or j/k),
// selection with Enter to apply a prompt, and cancellation with Escape to return
// to normal mode.
func (m Model) handleLibraryInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		entry := PromptLibrary[m.libraryIndex]
		m.prompt = entry.Prompt
		m.promptNoPersona = entry.NoPersona
		m.mode = modeNormal
		m = resetForNewReview(m)
		return m, startStreamCmd(m.buildFullPrompt(), m.diffContent, m.modelName)
	case "esc":
		m.mode = modeNormal
		m.reviewViewport.SetContent(m.renderMarkdown(m.reviewContent.String()))
		return m, nil
	case "j", "down":
		if m.libraryIndex < len(PromptLibrary)-1 {
			m.libraryIndex++
		}
		m.reviewViewport.SetContent(m.renderLibraryList())
		return m, nil
	case "k", "up":
		if m.libraryIndex > 0 {
			m.libraryIndex--
		}
		m.reviewViewport.SetContent(m.renderLibraryList())
		return m, nil
	default:
		return m, nil
	}
}

// renderLibraryList renders the prompt library view displaying all available
// prompts organized by category. It shows a header with navigation instructions
// (j/k for navigation, Enter to select, Esc to cancel), followed by categorized
// prompt entries. The currently selected prompt (determined by m.libraryIndex)
// is highlighted with a ">" prefix, while unselected items use a spaced prefix.
// Each category is displayed with a distinct header style.
func (m Model) renderLibraryList() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Render("Prompt Library"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("j/k navigate | Enter select | Esc cancel"))
	b.WriteString("\n")

	lastCategory := ""
	for i, entry := range PromptLibrary {
		if entry.Category != lastCategory {
			b.WriteString("\n")
			b.WriteString(libraryCategoryStyle.Render(entry.Category))
			b.WriteString("\n")
			lastCategory = entry.Category
		}
		if i == m.libraryIndex {
			b.WriteString(librarySelectedStyle.Render("> " + entry.Prompt))
		} else {
			b.WriteString(libraryItemStyle.Render("  " + entry.Prompt))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// handlePersonaInput processes keyboard input when the TUI is in persona selection mode.
// It enables navigation through a list of available review personas using arrow keys
// (j/k or up/down) and selection via Enter. The function handles three built-in options:
// "(None)" (no persona), "(Critical Only)" (critical issues only), and "(Terse)" (brief output),
// as well as any custom personas. When a persona is selected, it updates the model's persona
// field and either displays existing review content or initiates a new review stream if diff
// content is present. Pressing Escape cancels persona selection and returns to normal mode.
func (m Model) handlePersonaInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Index 0 = "(None)", 1 = "(Critical Only)", 2 = "(Terse)", 3..len(Personas)+2 = personas
	maxIndex := len(Personas) + 2
	switch msg.String() {
	case "enter":
		m.mode = modeNormal
		switch m.personaIndex {
		case 0:
			m.persona = nil
		case 1:
			m.persona = &Persona{Name: "(Critical Only)", Description: "Only report critical issues — bugs, security vulnerabilities, data loss risks, and correctness problems. Skip style, naming, and minor suggestions."}
		case 2:
			m.persona = &Persona{Name: "(Terse)", Description: "Extremely brief and concise, bullet points only, no fluff"}
		default:
			m.persona = &Personas[m.personaIndex-3]
		}
		saveCmd := saveProfileCmd(m)
		if m.diffContent == "" {
			m.reviewViewport.SetContent(m.renderMarkdown(m.reviewContent.String()))
			return m, saveCmd
		}
		m = resetForNewReview(m)
		return m, tea.Batch(saveCmd, startStreamCmd(m.buildFullPrompt(), m.diffContent, m.modelName))
	case "esc":
		m.mode = modeNormal
		m.reviewViewport.SetContent(m.renderMarkdown(m.reviewContent.String()))
		return m, nil
	case "j", "down":
		if m.personaIndex < maxIndex {
			m.personaIndex++
		}
		m.reviewViewport.SetContent(m.renderPersonaList())
		return m, nil
	case "k", "up":
		if m.personaIndex > 0 {
			m.personaIndex--
		}
		m.reviewViewport.SetContent(m.renderPersonaList())
		return m, nil
	default:
		return m, nil
	}
}

// renderPersonaList renders the persona selection list view for the TUI.
// It displays special filter options ("None", "Critical Only", "Terse") at the top,
// followed by all available personas with their names and descriptions.
// The currently selected item is highlighted using the selected style.
// Navigation hints are shown at the top of the list.
func (m Model) renderPersonaList() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Render("Persona"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("j/k navigate | Enter select | Esc cancel"))
	b.WriteString("\n\n")

	// Special options at the top
	specialOptions := []string{"(None)", "(Critical Only)", "(Terse)"}
	for i, label := range specialOptions {
		if i == m.personaIndex {
			b.WriteString(librarySelectedStyle.Render("> " + label))
		} else {
			b.WriteString(libraryItemStyle.Render("  " + label))
		}
		b.WriteString("\n")
	}

	offset := len(specialOptions)
	for i, p := range Personas {
		label := fmt.Sprintf("%s — %s", p.Name, p.Description)
		if i+offset == m.personaIndex {
			b.WriteString(librarySelectedStyle.Render("> " + label))
		} else {
			b.WriteString(libraryItemStyle.Render("  " + label))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// handleConfirmLargeDiff processes keyboard input when the user is confirming
// whether to proceed with reviewing a large diff. The "enter" key confirms and
// initiates the review process, while "esc" cancels and clears all pending diff
// state, returning to normal mode.
func (m Model) handleConfirmLargeDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.pendingDiff = nil
		m.mode = modeNormal
		m = resetForNewReview(m)
		return m, startStreamCmd(m.buildFullPrompt(), m.diffContent, m.modelName)
	case "esc":
		m.pendingDiff = nil
		m.mode = modeNormal
		m.diffContent = ""
		m.diffLabel = ""
		m.diffViewport.SetContent("")
		m.reviewContent.Reset()
		m.reviewViewport.SetContent("")
		m.statusMsg = ""
		return m, nil
	default:
		return m, nil
	}
}
