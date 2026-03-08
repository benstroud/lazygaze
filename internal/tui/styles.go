package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	diffAdd    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	diffRemove = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	diffHunk   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	diffMeta   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	activeBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("35"))

	inactiveBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("135")).
			PaddingLeft(1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("135")).
			PaddingLeft(1)

	streamingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Italic(true).
				PaddingLeft(1).
				PaddingTop(1)

	libraryCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("63")).
				PaddingLeft(1)

	librarySelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("63"))

	libraryItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				PaddingLeft(1)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			PaddingLeft(1).
			PaddingTop(1)

	personaNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Underline(true)

	scrollPctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// colorizeDiff applies syntax highlighting to a raw diff string by applying
// different styles based on the prefix of each line. Meta lines (+++,
// ---, diff, index) receive diffMeta styling, hunk headers (@@) receive
// diffHunk styling, additions (+) receive diffAdd styling, and deletions (-)
// receive diffRemove styling. The colorized result is returned as a single
// joined string.
func colorizeDiff(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"),
			strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
			lines[i] = diffMeta.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = diffHunk.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffAdd.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffRemove.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
