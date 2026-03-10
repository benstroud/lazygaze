package tui

import (
	"fmt"
	"math"
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

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			PaddingLeft(1).
			PaddingTop(1)

	personaNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Underline(true)

	scrollPctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	zoomHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("63")).
			Bold(true)
)

// pulseCycleTicks is the number of 80ms spinner ticks per full breath cycle.
// 40 ticks × 80ms = 3.2s per cycle.
const pulseCycleTicks = 40

func activeBorderPulsed(idx int) lipgloss.Style {
	// sine oscillates -1..1; shift to 0..1 for a smooth inhale/exhale
	t := (1 + math.Sin(2*math.Pi*float64(idx)/float64(pulseCycleTicks))) / 2
	// interpolate green channel: dim #005f00 (95) → peak #009b00 (155)
	green := 0x5f + int(t*float64(0x9b-0x5f))
	color := lipgloss.Color(fmt.Sprintf("#00%02x00", green))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color)
}

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
