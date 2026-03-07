package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	headerText := fmt.Sprintf("lazyreview [%s]", m.modelName)
	if m.diffLabel != "" {
		headerText = fmt.Sprintf("lazyreview: %s | %s [%s]", m.diffLabel, m.prompt, m.modelName)
	}
	header := headerStyle.Render(headerText)
	if m.persona != nil {
		header += headerStyle.Render(" as ") + personaNameStyle.Render(m.persona.Name)
	}

	// Pane dimensions
	borderWidth := 2
	paneWidth := (m.width / 2) - borderWidth
	if paneWidth < 1 {
		paneWidth = 1
	}

	// Diff pane
	diffStyle := inactiveBorder
	if m.focusedPane == 0 {
		diffStyle = activeBorder
	}
	diffView := m.diffViewport.View()
	if m.diffContent == "" {
		diffView = placeholderStyle.Render("Press : to set a git range")
	}
	paneHeight := m.diffViewport.Height
	diffPane := diffStyle.Width(paneWidth).Height(paneHeight).Render(diffView)

	// Review pane
	reviewStyle := inactiveBorder
	if m.focusedPane == 1 {
		reviewStyle = activeBorder
	}
	reviewView := m.reviewViewport.View()
	if m.err != nil {
		reviewView = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.reviewContent.Len() == 0 && !m.streaming && m.mode != modeConfirmLargeDiff {
		reviewView = placeholderStyle.Render("The LLM output will appear here")
	}
	reviewPane := reviewStyle.Width(paneWidth).Height(paneHeight).Render(reviewView)

	// Join panes
	panes := lipgloss.JoinHorizontal(lipgloss.Top, diffPane, reviewPane)

	// Footer
	var footer string
	wideFooter := footerStyle.Width(m.width)
	switch m.mode {
	case modeGitRange:
		footer = wideFooter.Render("git range: " + m.gitRangeInput.View())
	case modePrompt:
		footer = wideFooter.Render("prompt: " + m.promptInput.View())
	case modeTilde:
		footer = wideFooter.Render("HEAD~n..HEAD, n = " + m.tildeInput.View())
	case modeLibrary:
		footer = wideFooter.Render("[j/k] navigate | [enter] select | [esc] cancel")
	case modePersona:
		footer = wideFooter.Render("[j/k] navigate | [enter] select | [esc] cancel")
	case modeConfirmLargeDiff:
		footer = wideFooter.Render("[enter] continue review | [esc] cancel")
	default:
		status := ""
		if m.copied {
			status = "copied!"
		} else if m.streaming {
			status = streamingStyle.Render(spinnerFrames[m.spinnerIndex] + " reviewing...")
		} else if m.done {
			status = "done"
		} else if m.err != nil {
			status = "error"
		} else if m.statusMsg != "" {
			status = m.statusMsg
		}
		if status != "" {
			footer = wideFooter.Render(footerHints + " | " + status)
		} else {
			footer = wideFooter.Render(footerHints)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}
