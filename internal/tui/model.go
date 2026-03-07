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

	"github.com/atotto/clipboard"
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
)

type diffSource int

const (
	diffSourceNone diffSource = iota
	diffSourceRange
	diffSourceStaged
	diffSourceRoot
)

const footerHints = "[tab] switch pane | [j/k] scroll | [:] git range | [0] initial commit | [^] last commit | [~] HEAD~n | [s] staged | [/] prompt | [m] model | [c] copy | [L] library | [P] persona | [r] refresh | [q] quit"

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
	ch     <-chan claude.StreamEvent
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
	ch             <-chan claude.StreamEvent
	autoScroll     bool

	// New fields
	mode          inputMode
	gitRangeInput textinput.Model
	promptInput   textinput.Model
	tildeInput    textinput.Model
	modelName     string
	diffSrc         diffSource
	cancelStream     context.CancelFunc
	cancelDiffFetch  context.CancelFunc
	diffFetchGen     int
	streamGen        int
	spinnerIndex     int
	copied           bool
	glamourRenderer  *glamour.TermRenderer
	libraryIndex     int
	personaIndex     int
	persona          *Persona // nil = no persona
	promptNoPersona  bool     // true when current library entry disables persona
	statusMsg        string
	pendingDiff      *diffFetchedMsg // held while awaiting large-diff confirmation
}

func New(diffContent string, gitRange, prompt string, ch <-chan claude.StreamEvent, modelName string, cancel context.CancelFunc, persona *Persona) Model {
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
		diffContent:   diffContent,
		gitRange:      gitRange,
		diffLabel:     gitRange,
		prompt:        prompt,
		ch:            ch,
		streaming:     true,
		autoScroll:    true,
		focusedPane:   1,
		modelName:     modelName,
		cancelStream:  cancel,
		streamGen:     0,
		reviewContent: &strings.Builder{},
		gitRangeInput: gi,
		promptInput:   pi,
		tildeInput:    ti,
		persona:       persona,
	}
}

func NewEmpty(modelName string, persona *Persona) Model {
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
		prompt:        claude.DefaultUserPrompt,
		modelName:     modelName,
		autoScroll:    true,
		focusedPane:   0,
		reviewContent: &strings.Builder{},
		gitRangeInput: gi,
		promptInput:   pi,
		tildeInput:    ti,
		persona:       persona,
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

func waitForStreamGen(ch <-chan claude.StreamEvent, gen int) tea.Cmd {
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

func startStreamCmd(prompt, diffText, modelName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		ch, err := claude.RunStreaming(ctx, prompt, diffText, modelName)
		if err != nil {
			cancel()
			return streamStartErrMsg{err: err}
		}
		return streamStartedMsg{ch: ch, cancel: cancel}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewports()
		return m, nil

	case tea.KeyMsg:
		// Modal input handling
		if m.mode == modeGitRange {
			return m.handleGitRangeInput(msg)
		}
		if m.mode == modePrompt {
			return m.handlePromptInput(msg)
		}
		if m.mode == modeTilde {
			return m.handleTildeInput(msg)
		}
		if m.mode == modeLibrary {
			return m.handleLibraryInput(msg)
		}
		if m.mode == modePersona {
			return m.handlePersonaInput(msg)
		}
		if m.mode == modeConfirmLargeDiff {
			return m.handleConfirmLargeDiff(msg)
		}

		// Normal mode keys
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancelStream != nil {
				m.cancelStream()
			}
			if m.cancelDiffFetch != nil {
				m.cancelDiffFetch()
			}
			return m, tea.Quit
		case "tab":
			m.focusedPane = (m.focusedPane + 1) % 2
			return m, nil
		case ":":
			m.mode = modeGitRange
			m.gitRangeInput.SetValue("")
			m.gitRangeInput.Focus()
			return m, textinput.Blink
		case "/":
			m.mode = modePrompt
			m.promptInput.SetValue("")
			m.promptInput.Focus()
			return m, textinput.Blink
		case "m":
			// Cycle through supported models
			for i, name := range supportedModels {
				if name == m.modelName {
					m.modelName = supportedModels[(i+1)%len(supportedModels)]
					return m, saveProfileCmd(m)
				}
			}
			m.modelName = supportedModels[0]
			return m, saveProfileCmd(m)
		case "c":
			var content string
			if m.focusedPane == 0 {
				content = m.diffContent
			} else {
				content = m.reviewContent.String()
			}
			if content == "" {
				return m, nil
			}
			if err := clipboard.WriteAll(content); err != nil {
				m.err = fmt.Errorf("clipboard: %w", err)
				return m, nil
			}
			m.copied = true
			return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return copyFadeMsg{} })
		case "~":
			m.mode = modeTilde
			m.tildeInput.SetValue("")
			m.tildeInput.Focus()
			return m, textinput.Blink
		case "s":
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceStaged
			return m, fetchDiffStagedCmd(ctx, m.diffFetchGen)
		case "0":
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceRoot
			return m, fetchDiffRootCmd(ctx, m.diffFetchGen)
		case "^":
			m.gitRange = "HEAD^..HEAD"
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceRange
			return m, fetchDiffCmd(ctx, m.gitRange, m.diffFetchGen)
		case "r":
			if m.diffSrc == diffSourceNone {
				m.err = fmt.Errorf("nothing to refresh — press : to set a git range first")
				return m, nil
			}
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			switch m.diffSrc {
			case diffSourceStaged:
				return m, fetchDiffStagedCmd(ctx, m.diffFetchGen)
			case diffSourceRoot:
				return m, fetchDiffRootCmd(ctx, m.diffFetchGen)
			case diffSourceRange:
				return m, fetchDiffCmd(ctx, m.gitRange, m.diffFetchGen)
			}
			return m, nil
		case "L":
			if m.diffContent == "" {
				m.err = fmt.Errorf("no diff loaded — press : to set a git range first")
				return m, nil
			}
			m.mode = modeLibrary
			m.libraryIndex = 0
			m.reviewViewport.SetContent(m.renderLibraryList())
			m.reviewViewport.GotoTop()
			return m, nil
		case "P":
			m.mode = modePersona
			m.personaIndex = 0
			m.reviewViewport.SetContent(m.renderPersonaList())
			m.reviewViewport.GotoTop()
			return m, nil
		}

		// Forward key to focused viewport
		var cmd tea.Cmd
		if m.focusedPane == 0 {
			m.diffViewport, cmd = m.diffViewport.Update(msg)
		} else {
			wasAtBottom := m.reviewViewport.AtBottom()
			m.reviewViewport, cmd = m.reviewViewport.Update(msg)
			if wasAtBottom && !m.reviewViewport.AtBottom() {
				m.autoScroll = false
			}
			if m.reviewViewport.AtBottom() {
				m.autoScroll = true
			}
		}
		return m, cmd

	case singleCommitRepoMsg:
		m.statusMsg = "single commit detected — loading initial commit diff"
		var ctx context.Context
		m, ctx = resetForNewStream(m)
		m.diffSrc = diffSourceRoot
		return m, fetchDiffRootCmd(ctx, m.diffFetchGen)

	case diffFetchedMsg:
		if msg.gen != m.diffFetchGen {
			return m, nil
		}
		// Release the diff-fetch context now that the fetch completed.
		if m.cancelDiffFetch != nil {
			m.cancelDiffFetch()
			m.cancelDiffFetch = nil
		}
		// Check if the diff is too large — ask for confirmation before sending to LLM.
		if git.IsLargeDiff(msg.diffText) {
			m.pendingDiff = &msg
			m.mode = modeConfirmLargeDiff
			m.diffContent = msg.diffText
			m.diffLabel = msg.label
			if m.diffSrc == diffSourceRange {
				m.gitRange = msg.label
			}
			m.diffViewport.SetContent(colorizeDiff(m.diffContent))
			m.diffViewport.GotoTop()
			lines := git.LineCount(msg.diffText)
			warning := fmt.Sprintf("Diff is %s lines — this may be expensive to review.\n\nPress Enter to continue or Esc to cancel.", humanizeInt(lines))
			m.reviewContent.Reset()
			m.reviewViewport.SetContent(warningStyle.Render(warning))
			m.reviewViewport.GotoTop()
			m.statusMsg = ""
			return m, nil
		}
		m.diffContent = msg.diffText
		m.diffLabel = msg.label
		if m.diffSrc == diffSourceRange {
			m.gitRange = msg.label
		}
		m.diffViewport.SetContent(colorizeDiff(m.diffContent))
		m.diffViewport.GotoTop()
		// Reset review and start streaming
		m.reviewContent.Reset()
		m.reviewViewport.SetContent("")
		m.err = nil
		m.done = false
		m.autoScroll = true
		return m, startStreamCmd(m.buildFullPrompt(), m.diffContent, m.modelName)

	case diffErrMsg:
		if msg.gen != m.diffFetchGen {
			return m, nil
		}
		m.err = msg.err
		m.streaming = false
		m.statusMsg = ""
		return m, nil

	case spinTickMsg:
		if !m.streaming {
			return m, nil
		}
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinTickMsg{} })

	case streamStartedMsg:
		m.ch = msg.ch
		m.cancelStream = msg.cancel
		m.streaming = true
		m.statusMsg = ""
		m.spinnerIndex = 0
		m.streamGen++
		m.focusedPane = 1
		return m, tea.Batch(
			waitForStreamGen(m.ch, m.streamGen),
			tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinTickMsg{} }),
		)

	case streamStartErrMsg:
		m.err = msg.err
		m.streaming = false
		m.statusMsg = ""
		return m, nil

	case streamChunkMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.reviewContent.WriteString(msg.content)
		m.reviewViewport.SetContent(m.reviewContent.String())
		if m.autoScroll {
			m.reviewViewport.GotoBottom()
		}
		return m, waitForStreamGen(m.ch, m.streamGen)

	case streamDoneMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.streaming = false
		m.done = true
		m.statusMsg = ""
		m.reviewViewport.SetContent(m.renderMarkdown(m.reviewContent.String()))
		m.reviewViewport.GotoTop()
		if m.cancelStream != nil {
			m.cancelStream()
			m.cancelStream = nil
		}
		return m, nil

	case copyFadeMsg:
		m.copied = false
		return m, nil

	case streamErrMsg:
		if msg.gen != m.streamGen {
			return m, nil
		}
		m.streaming = false
		m.err = msg.err
		m.statusMsg = ""
		if m.cancelStream != nil {
			m.cancelStream()
			m.cancelStream = nil
		}
		return m, nil
	}

	return m, nil
}

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
		// Cancel any in-flight stream
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

func (m Model) handleLibraryInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		entry := PromptLibrary[m.libraryIndex]
		m.prompt = entry.Prompt
		m.promptNoPersona = entry.NoPersona
		m.mode = modeNormal
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

func saveProfileCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		prof := config.Load()
		prof.PersonaName = personaName(m.persona)
		prof.ModelName = m.modelName
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

func (m Model) handleConfirmLargeDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		pending := m.pendingDiff
		m.pendingDiff = nil
		m.mode = modeNormal
		// Proceed with the review as normal.
		m.reviewContent.Reset()
		m.reviewViewport.SetContent("")
		m.err = nil
		m.done = false
		m.autoScroll = true
		_ = pending // diffContent/diffLabel already set when we entered confirmation mode
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

func resetForNewStream(m Model) (Model, context.Context) {
	if m.cancelStream != nil {
		m.cancelStream()
		m.cancelStream = nil
	}
	if m.cancelDiffFetch != nil {
		m.cancelDiffFetch()
		m.cancelDiffFetch = nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelDiffFetch = cancel
	m.diffFetchGen++
	m.streaming = false
	m.reviewContent.Reset()
	m.reviewViewport.SetContent("")
	m.err = nil
	m.done = false
	m.autoScroll = true
	m.streamGen++
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

func (m *Model) footerLines() int {
	if m.width <= 0 {
		return 1
	}
	rendered := footerStyle.Width(m.width).Render(footerHints)
	return lipgloss.Height(rendered)
}

func (m *Model) resizeViewports() {
	headerHeight := 1
	footerHeight := m.footerLines()
	borderHeight := 2 // top + bottom border
	paneHeight := m.height - headerHeight - footerHeight - borderHeight
	if paneHeight < 1 {
		paneHeight = 1
	}

	borderWidth := 2 // left + right border
	paneWidth := (m.width / 2) - borderWidth
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
