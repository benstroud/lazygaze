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

	// Footer
	footer := footerStyle.Width(m.width).Render(m.footerContent())

	// Pane dimensions — derived from actual header/footer heights
	borderWidth := 2
	borderHeight := 2
	paneWidth := (m.width / 2) - borderWidth
	if paneWidth < 1 {
		paneWidth = 1
	}
	paneHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - borderHeight
	if paneHeight < 1 {
		paneHeight = 1
	}
	m.diffViewport.Height = paneHeight
	m.reviewViewport.Height = paneHeight

	// Diff pane
	diffStyle := inactiveBorder
	if m.focusedPane == 0 {
		diffStyle = activeBorder
	}
	diffView := m.diffViewport.View()
	if m.diffContent == "" {
		diffView = placeholderStyle.Render("Press : to set a git range")
	}
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

	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}
