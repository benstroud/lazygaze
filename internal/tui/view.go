package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	harnessName := m.activeHarness.Name()
	harnessLabel := harnessName + ":" + m.modelName
	if harnessName != "claude" {
		harnessLabel = harnessName
	}
	headerText := fmt.Sprintf("lazygaze [%s]", harnessLabel)
	if m.diffLabel != "" {
		headerText = fmt.Sprintf("lazygaze: %s | %s [%s]", m.diffLabel, m.prompt, harnessLabel)
	}
	header := headerStyle.Render(headerText)
	if m.persona != nil {
		header += headerStyle.Render("as ") + personaNameStyle.Render(m.persona.Name)
	}

	// Footer
	footer := footerStyle.Width(m.width).Render(m.footerContent())

	// Pane dimensions — derived from actual header/footer heights
	borderWidth := 2
	borderHeight := 2
	if m.zoomed {
		borderWidth = 0
		borderHeight = 0
	}
	var paneWidth int
	if m.zoomed {
		paneWidth = m.width - borderWidth
	} else {
		paneWidth = (m.width / 2) - borderWidth
	}
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
		if m.streaming {
			diffStyle = activeBorderPulsed(m.pulseIndex)
		} else {
			diffStyle = activeBorder
		}
	}
	if m.zoomed {
		diffStyle = lipgloss.NewStyle()
	}
	diffView := m.diffViewport.View()
	diffShowPct := m.diffContent != ""
	if m.diffContent == "" {
		diffView = placeholderStyle.Render("Press : to set a git range")
	}
	diffPane := diffStyle.Width(paneWidth).Height(paneHeight).Render(diffView)
	if diffShowPct && m.diffViewport.TotalLineCount() > m.diffViewport.Height {
		diffPane = overlayScrollPct(diffPane, m.diffViewport.ScrollPercent())
	}

	// Review pane
	reviewStyle := inactiveBorder
	if m.focusedPane == 1 {
		if m.streaming {
			reviewStyle = activeBorderPulsed(m.pulseIndex)
		} else {
			reviewStyle = activeBorder
		}
	}
	if m.zoomed {
		reviewStyle = lipgloss.NewStyle()
	}
	reviewView := m.reviewViewport.View()
	reviewShowPct := true
	if m.err != nil {
		reviewView = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
		reviewShowPct = false
	} else if m.reviewContent.Len() == 0 && !m.streaming &&
		m.mode != modeConfirmLargeDiff && m.mode != modeLibrary &&
		m.mode != modePersona && m.mode != modeHarness && m.mode != modeHelp {
		reviewView = placeholderStyle.Render("The LLM output will appear here")
		reviewShowPct = false
	}
	reviewPane := reviewStyle.Width(paneWidth).Height(paneHeight).Render(reviewView)
	if reviewShowPct && m.reviewViewport.TotalLineCount() > m.reviewViewport.Height {
		reviewPane = overlayScrollPct(reviewPane, m.reviewViewport.ScrollPercent())
	}

	// Join panes
	var panes string
	if m.zoomed {
		if m.focusedPane == 0 {
			panes = diffPane
		} else {
			panes = reviewPane
		}
	} else {
		panes = lipgloss.JoinHorizontal(lipgloss.Top, diffPane, reviewPane)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}

// overlayScrollPct splices a scroll percentage label onto the bottom border
// of a rendered pane, replacing border characters near the right edge.
func overlayScrollPct(rendered string, pct float64) string {
	label := scrollPctStyle.Render(fmt.Sprintf(" %d%% ", int(pct*100)))
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}
	last := lines[len(lines)-1]
	labelW := lipgloss.Width(label)
	lastW := lipgloss.Width(last)
	if labelW+2 > lastW {
		return rendered
	}
	// ANSI-aware: keep prefix up to where label starts, skip middle, keep corner
	prefix := ansi.Truncate(last, lastW-labelW-1, "")
	suffix := ansi.TruncateLeft(last, lastW-1, "")
	lines[len(lines)-1] = prefix + label + suffix
	return strings.Join(lines, "\n")
}
