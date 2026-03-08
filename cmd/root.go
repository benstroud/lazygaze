package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/benstroud/lazygaze/internal/claude"
	"github.com/benstroud/lazygaze/internal/cli"
	"github.com/benstroud/lazygaze/internal/config"
	"github.com/benstroud/lazygaze/internal/copilot"
	"github.com/benstroud/lazygaze/internal/git"
	"github.com/benstroud/lazygaze/internal/harness"
	"github.com/benstroud/lazygaze/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	cliMode bool
	model   string
)

var rootCmd = &cobra.Command{
	Use:   "lazygaze [git-range] [prompt]",
	Short: "Review git diffs with Claude",
	Args:  cobra.RangeArgs(0, 2),
	RunE:  run,
}

func init() {
	rootCmd.Flags().BoolVar(&cliMode, "cli", false, "simple CLI mode (no TUI)")
	rootCmd.Flags().StringVar(&model, "model", "sonnet", "Claude model to use")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	gitRange := ""
	prompt := claude.DefaultUserPrompt
	if len(args) >= 1 {
		gitRange = args[0]
	}
	if len(args) > 1 {
		prompt = args[1]
	}

	if err := git.CheckBinary(); err != nil {
		return err
	}

	// Detect available harnesses.
	claudeAvail := claude.CheckBinary() == nil
	copilotAvail := copilot.CheckBinary() == nil
	if !claudeAvail && !copilotAvail {
		return fmt.Errorf("no harness found: install claude or copilot CLI")
	}

	prof := config.Load()
	if !cmd.Flags().Changed("model") && prof.ModelName != "" {
		model = prof.ModelName
	}

	var availableHarnesses []harness.Harness
	if claudeAvail {
		availableHarnesses = append(availableHarnesses, claude.New(model))
	}
	if copilotAvail {
		availableHarnesses = append(availableHarnesses, copilot.New())
	}

	// Select active harness from profile, falling back to first available.
	var activeHarness harness.Harness
	for _, h := range availableHarnesses {
		if h.Name() == prof.HarnessName {
			activeHarness = h
			break
		}
	}
	if activeHarness == nil {
		activeHarness = availableHarnesses[0]
	}

	if claudeAvail && copilotAvail && prof.HarnessName == "" {
		fmt.Fprintln(os.Stderr, "Info: both claude and copilot detected. Defaulting to claude. Press 'H' in TUI to switch harness.")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if cliMode {
		if gitRange == "" {
			return fmt.Errorf("git-range is required in CLI mode")
		}
		fullPrompt := claude.BuildPrompt(claude.DefaultSystemPrompt, prompt)
		return cli.Run(ctx, gitRange, fullPrompt, activeHarness)
	}

	persona := tui.ResolveByName(prof.PersonaName)

	// TUI mode with no args: launch empty.
	if gitRange == "" {
		m := tui.NewEmpty(model, persona, activeHarness, availableHarnesses)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	}

	// TUI mode with args.
	diffText, err := git.Diff(ctx, gitRange)
	if err != nil {
		return err
	}

	if git.IsLargeDiff(diffText) {
		lines := git.LineCount(diffText)
		fmt.Fprintf(os.Stderr, "Large Diff Warning: diff is %d lines (threshold: %d).\n", lines, git.MaxDiffLines)
		fmt.Fprint(os.Stderr, "Continue? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() || strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			return fmt.Errorf("aborted: diff too large")
		}
	}

	sys := claude.DefaultSystemPrompt
	if persona != nil {
		if persona.Anonymous {
			sys += fmt.Sprintf("\n%s", persona.Description)
		} else {
			sys += fmt.Sprintf("\nAdopt the voice, opinions, and reviewing style of %s. %s. Review as they would — with their known priorities, pet peeves, and communication style.", persona.Name, persona.Description)
		}
	}
	fullPrompt := claude.BuildPrompt(sys, prompt)
	streamCtx, streamCancel := context.WithCancel(ctx)
	ch, err := activeHarness.RunStreaming(streamCtx, fullPrompt, diffText)
	if err != nil {
		streamCancel()
		return fmt.Errorf("starting %s: %w", activeHarness.Name(), err)
	}

	m := tui.New(diffText, gitRange, prompt, ch, model, streamCancel, persona, activeHarness, availableHarnesses)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
