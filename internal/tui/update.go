package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/benstroud/lazygaze/internal/git"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
		if m.mode == modeHarness {
			return m.handleHarnessInput(msg)
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
			m.zoomed = false
			m.focusedPane = (m.focusedPane + 1) % 2
			m.resizeViewports()
			return m, nil
		case "z":
			m.zoomed = !m.zoomed
			m.resizeViewports()
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
					m.activeHarness = m.activeHarness.WithModel(m.modelName)
					return m, saveProfileCmd(m)
				}
			}
			m.modelName = supportedModels[0]
			m.activeHarness = m.activeHarness.WithModel(m.modelName)
			return m, saveProfileCmd(m)
		case "H":
			if len(m.availableHarnesses) <= 1 {
				m.statusMsg = "Only one harness available"
				return m, nil
			}
			m.mode = modeHarness
			for i, h := range m.availableHarnesses {
				if h.Name() == m.activeHarness.Name() {
					m.harnessIndex = i
					break
				}
			}
			m.reviewViewport.SetContent(m.renderHarnessList())
			m.reviewViewport.GotoTop()
			return m, nil
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
		case "S":
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceStaged
			return m, fetchDiffStagedCmd(ctx, m.diffFetchGen)
		case "D":
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceHEAD
			return m, fetchDiffHEADCmd(ctx, m.diffFetchGen)
		case "^":
			m.gitRange = "HEAD^..HEAD"
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceRange
			return m, fetchDiffCmd(ctx, m.gitRange, m.diffFetchGen)
		case "U":
			var ctx context.Context
			m, ctx = resetForNewStream(m)
			m.diffSrc = diffSourceUpstream
			return m, fetchDiffUpstreamCmd(ctx, m.diffFetchGen)
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
			case diffSourceHEAD:
				return m, fetchDiffHEADCmd(ctx, m.diffFetchGen)
			case diffSourceRange:
				return m, fetchDiffCmd(ctx, m.gitRange, m.diffFetchGen)
			case diffSourceUpstream:
				return m, fetchDiffUpstreamCmd(ctx, m.diffFetchGen)
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
			content, _ := m.renderPersonaList()
			m.reviewViewport.SetContent(content)
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
		// Handle empty upstream diff specially - branches are in sync
		if m.diffSrc == diffSourceUpstream && strings.TrimSpace(msg.diffText) == "" {
			m.diffContent = msg.diffText
			m.diffLabel = msg.label
			m.diffViewport.SetContent(colorizeDiff(m.diffContent))
			m.diffViewport.GotoTop()
			infoMsg := fmt.Sprintf("Your branch is up to date with %s (no changes to review)", msg.label)
			m.reviewContent.Reset()
			m.reviewContent.WriteString(infoMsg)
			m.reviewViewport.SetContent(infoStyle.Render(infoMsg))
			m.reviewViewport.GotoTop()
			m.err = nil
			m.streaming = false
			m.done = true
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
		return m, startStreamCmd(m.activeHarness, m.buildFullPrompt(), m.diffContent)

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
		m.pulseIndex = (m.pulseIndex + 1) % pulseCycleTicks
		return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return spinTickMsg{} })

	case streamStartedMsg:
		m.ch = msg.ch
		m.cancelStream = msg.cancel
		m.streaming = true
		m.statusMsg = ""
		m.spinnerIndex = 0
		m.pulseIndex = 0
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
