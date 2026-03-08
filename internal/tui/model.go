package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/benstroud/lazyreview/internal/claude"
	"github.com/benstroud/lazyreview/internal/config"
	"github.com/benstroud/lazyreview/internal/git"
	"github.com/benstroud/lazyreview/internal/harness"
	"github.com/benstroud/lazyreview/internal/stream"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinTickMsg struct{}
type copyFadeMsg struct{}

var supportedModels = []string{"sonnet", "opus", "haiku"}

type inputMode int

const (
	modeNormal inputMode = iota
	modeGitRange
	modePrompt
	modeTilde
	modeLibrary
	modePersona
	modeConfirmLargeDiff
	modeHarness
)

type diffSource int

const (
	diffSourceNone diffSource = iota
	diffSourceRange
	diffSourceStaged
	diffSourceRoot
	diffSourceHEAD
)

const footerHintsBase = "[tab] switch pane | [j/k] scroll | %s | [:] git range | [S] staged | [D] uncommitted | [^] last commit | [~] HEAD~n | [/] prompt | [m] model | [H] harness | [c] copy | [L] library | [P] persona | [r] refresh | [q] quit"

// Message types with generation tracking for stream messages
type singleCommitRepoMsg struct{}

type streamChunkMsg struct {
	content string
	gen     int
}
type streamDoneMsg struct{ gen int }
type streamErrMsg struct {
	err error
	gen int
}

type diffFetchedMsg struct {
	diffText string
	label    string
	gen      int
}
type diffErrMsg struct {
	err error
	gen int
}

type streamStartedMsg struct {
	ch     <-chan stream.Event
	cancel context.CancelFunc
}
type streamStartErrMsg struct{ err error }

type Model struct {
	width, height  int
	gitRange       string // actual git range for diffSourceRange; empty otherwise
	diffLabel      string // display label for header (e.g. "staged", "initial commit", or the git range)
	prompt         string
	diffContent    string
	reviewContent  *strings.Builder
	diffViewport   viewport.Model
	reviewViewport viewport.Model
	streaming      bool
	done           bool
	err            error
	focusedPane    int // 0=diff, 1=review
	ch             <-chan stream.Event
	autoScroll     bool

	// New fields
	mode               inputMode
	gitRangeInput      textinput.Model
	promptInput        textinput.Model
	tildeInput         textinput.Model
	modelName          string
	activeHarness      harness.Harness
	harnessIndex       int
	availableHarnesses []harness.Harness
	diffSrc            diffSource
	cancelStream       context.CancelFunc
	cancelDiffFetch    context.CancelFunc
	diffFetchGen       int
	streamGen          int
	spinnerIndex       int
	copied             bool
	glamourRenderer    *glamour.TermRenderer
	libraryIndex       int
	personaIndex       int
	persona            *Persona // nil = no persona
	promptNoPersona    bool     // true when current library entry disables persona
	statusMsg          string
	pendingDiff        *diffFetchedMsg // held while awaiting large-diff confirmation
	zoomed             bool
}

func New(diffContent string, gitRange, prompt string, ch <-chan stream.Event, modelName string, cancel context.CancelFunc, persona *Persona, activeHarness harness.Harness, availableHarnesses []harness.Harness) Model {
	gi := textinput.New()
	gi.Placeholder = "e.g. HEAD~3..HEAD"
	gi.CharLimit = 256

	pi := textinput.New()
	pi.Placeholder = "e.g. Focus on security issues"
	pi.CharLimit = 512

	ti := textinput.New()
	ti.Placeholder = "e.g. 3"
	ti.CharLimit = 10

	return Model{
		diffContent:        diffContent,
		gitRange:           gitRange,
		diffLabel:          gitRange,
		prompt:             prompt,
		ch:                 ch,
		streaming:          true,
		autoScroll:         true,
		focusedPane:        1,
		modelName:          modelName,
		activeHarness:      activeHarness,
		availableHarnesses: availableHarnesses,
		cancelStream:       cancel,
		streamGen:          0,
		reviewContent:      &strings.Builder{},
		gitRangeInput:      gi,
		promptInput:        pi,
		tildeInput:         ti,
		persona:            persona,
	}
}

func NewEmpty(modelName string, persona *Persona, activeHarness harness.Harness, availableHarnesses []harness.Harness) Model {
	gi := textinput.New()
	gi.Placeholder = "e.g. HEAD~3..HEAD"
	gi.CharLimit = 256

	pi := textinput.New()
	pi.Placeholder = "e.g. Focus on security issues"
	pi.CharLimit = 512

	ti := textinput.New()
	ti.Placeholder = "e.g. 3"
	ti.CharLimit = 10

	return Model{
		prompt:             claude.DefaultUserPrompt,
		modelName:          modelName,
		activeHarness:      activeHarness,
		availableHarnesses: availableHarnesses,
		autoScroll:         true,
		focusedPane:        0,
		reviewContent:      &strings.Builder{},
		gitRangeInput:      gi,
		promptInput:        pi,
		tildeInput:         ti,
		persona:            persona,
	}
}

func initCheckCmd() tea.Cmd {
	return func() tea.Msg {
		count, err := git.CommitCount(context.Background())
		if err != nil || count != 1 {
			// bubbletea silently ignores nil messages — this is intentional
			// to skip auto-review when the check fails or repo has multiple commits
			return nil
		}
		return singleCommitRepoMsg{}
	}
}

func (m Model) Init() tea.Cmd {
	if m.ch == nil {
		return initCheckCmd()
	}
	return tea.Batch(
		waitForStreamGen(m.ch, m.streamGen),
		tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinTickMsg{} }),
	)
}

func waitForStreamGen(ch <-chan stream.Event, gen int) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return streamDoneMsg{gen: gen}
		}
		if event.Err != nil {
			return streamErrMsg{err: event.Err, gen: gen}
		}
		if event.Done {
			if event.Content != "" {
				return streamChunkMsg{content: event.Content, gen: gen}
			}
			return streamDoneMsg{gen: gen}
		}
		return streamChunkMsg{content: event.Content, gen: gen}
	}
}

func fetchDiffCmd(ctx context.Context, gitRange string, gen int) tea.Cmd {
	return func() tea.Msg {
		diffText, err := git.Diff(ctx, gitRange)
		if err != nil {
			return diffErrMsg{err: err, gen: gen}
		}
		return diffFetchedMsg{diffText: diffText, label: gitRange, gen: gen}
	}
}

func fetchDiffStagedCmd(ctx context.Context, gen int) tea.Cmd {
	return func() tea.Msg {
		diffText, err := git.DiffStaged(ctx)
		if err != nil {
			return diffErrMsg{err: err, gen: gen}
		}
		return diffFetchedMsg{diffText: diffText, label: "staged", gen: gen}
	}
}

func fetchDiffHEADCmd(ctx context.Context, gen int) tea.Cmd {
	return func() tea.Msg {
		diffText, err := git.DiffHEAD(ctx)
		if err != nil {
			return diffErrMsg{err: err, gen: gen}
		}
		return diffFetchedMsg{diffText: diffText, label: "uncommitted", gen: gen}
	}
}

func fetchDiffRootCmd(ctx context.Context, gen int) tea.Cmd {
	return func() tea.Msg {
		diffText, err := git.DiffRoot(ctx)
		if err != nil {
			return diffErrMsg{err: err, gen: gen}
		}
		return diffFetchedMsg{diffText: diffText, label: "initial commit", gen: gen}
	}
}

func (m Model) buildFullPrompt() string {
	sys := claude.DefaultSystemPrompt
	if m.persona != nil && !m.promptNoPersona {
		sys += fmt.Sprintf("\nAdopt the voice, opinions, and reviewing style of %s. %s. Review as they would — with their known priorities, pet peeves, and communication style.", m.persona.Name, m.persona.Description)
	}
	return claude.BuildPrompt(sys, m.prompt)
}

func startStreamCmd(h harness.Harness, prompt, diffText string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := h.RunStreaming(ctx, prompt, diffText)
		if err != nil {
			cancel()
			return streamStartErrMsg{err: err}
		}
		return streamStartedMsg{ch: ch, cancel: cancel}
	}
}

func saveProfileCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		prof := config.Load()
		prof.PersonaName = personaName(m.persona)
		prof.ModelName = m.modelName
		prof.HarnessName = m.activeHarness.Name()
		config.Save(prof)
		return nil
	}
}

func personaName(p *Persona) string {
	if p == nil {
		return ""
	}
	return p.Name
}

func humanizeInt(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// resetForNewReview cancels any in-flight stream and resets review state,
// preparing the model to start a new LLM review on an already-loaded diff.
func resetForNewReview(m Model) Model {
	if m.cancelStream != nil {
		m.cancelStream()
		m.cancelStream = nil
	}
	m.reviewContent.Reset()
	m.reviewViewport.SetContent("")
	m.err = nil
	m.done = false
	m.streaming = false
	m.autoScroll = true
	m.streamGen++
	return m
}

// resetForNewStream cancels both the stream and diff fetch, then sets up a
// new diff-fetch context. Used when loading a new diff (range, staged, root).
func resetForNewStream(m Model) (Model, context.Context) {
	m = resetForNewReview(m)
	if m.cancelDiffFetch != nil {
		m.cancelDiffFetch()
		m.cancelDiffFetch = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelDiffFetch = cancel
	m.diffFetchGen++
	return m, ctx
}

func (m *Model) renderMarkdown(raw string) string {
	if m.glamourRenderer == nil || raw == "" {
		return raw
	}
	rendered, err := m.glamourRenderer.Render(raw)
	if err != nil {
		return raw
	}
	return strings.TrimRight(rendered, "\n")
}

func (m Model) footerContent() string {
	switch m.mode {
	case modeGitRange:
		return "git range: " + m.gitRangeInput.View()
	case modePrompt:
		return "prompt: " + m.promptInput.View()
	case modeTilde:
		return "HEAD~n..HEAD, n = " + m.tildeInput.View()
	case modeLibrary, modePersona, modeHarness:
		return "[j/k] navigate | [enter] select | [esc] cancel"
	case modeConfirmLargeDiff:
		return "[enter] continue review | [esc] cancel"
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
		zoomHint := "[z] zoom"
		if m.zoomed {
			zoomHint = zoomHintStyle.Render("[z] zoom out")
		}
		hints := fmt.Sprintf(footerHintsBase, zoomHint)
		if status != "" {
			return hints + " | " + status
		}
		return hints
	}
}

func (m *Model) footerLines() int {
	if m.width <= 0 {
		return 1
	}
	rendered := footerStyle.Width(m.width).Render(m.footerContent())
	return lipgloss.Height(rendered)
}

// resizeViewports calculates and sets the dimensions for the diff and review
// viewports based on the current terminal size. It accounts for the header,
// footer, and border heights/widths to determine the available pane space.
// The glamour renderer is also reinitialized with the new pane width for
// proper markdown word wrapping. If auto-scroll is enabled, the review
// viewport scrolls to the bottom after content is set.
func (m *Model) resizeViewports() {
	headerHeight := 1
	footerHeight := m.footerLines()
	borderHeight := 2 // top + bottom border
	paneHeight := m.height - headerHeight - footerHeight - borderHeight
	if paneHeight < 1 {
		paneHeight = 1
	}

	var paneWidth int
	if m.zoomed {
		paneWidth = m.width
	} else {
		borderWidth := 2 // left + right border
		paneWidth = (m.width / 2) - borderWidth
	}
	if paneWidth < 1 {
		paneWidth = 1
	}

	m.diffViewport.Width = paneWidth
	m.diffViewport.Height = paneHeight
	m.diffViewport.SetContent(colorizeDiff(m.diffContent))

	m.glamourRenderer, _ = glamour.NewTermRenderer(
		glamour.WithStyles(styles.DarkStyleConfig),
		glamour.WithWordWrap(paneWidth),
	)

	m.reviewViewport.Width = paneWidth
	m.reviewViewport.Height = paneHeight
	m.reviewViewport.SetContent(m.renderMarkdown(m.reviewContent.String()))
	if m.autoScroll {
		m.reviewViewport.GotoBottom()
	}
}
