package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/benstroud/lazyreview/internal/claude"
	"github.com/benstroud/lazyreview/internal/cli"
	"github.com/benstroud/lazyreview/internal/config"
	"github.com/benstroud/lazyreview/internal/git"
	"github.com/benstroud/lazyreview/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	cliMode bool
	model   string
)

var rootCmd = &cobra.Command{
	Use:   "lazyreview [git-range] [prompt]",
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
	if err := claude.CheckBinary(); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if cliMode {
		if gitRange == "" {
			return fmt.Errorf("git-range is required in CLI mode")
		}
		fullPrompt := claude.BuildPrompt(claude.DefaultSystemPrompt, prompt)
		return cli.Run(ctx, gitRange, fullPrompt, model)
	}

	prof := config.Load()
	persona := tui.ResolveByName(prof.PersonaName)
	if !cmd.Flags().Changed("model") && prof.ModelName != "" {
		model = prof.ModelName
	}

	// TUI mode with no args: launch empty
	if gitRange == "" {
		m := tui.NewEmpty(model, persona)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	}

	// TUI mode with args
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
		sys += fmt.Sprintf("\nAdopt the voice, opinions, and reviewing style of %s. %s. Review as they would — with their known priorities, pet peeves, and communication style.", persona.Name, persona.Description)
	}
	fullPrompt := claude.BuildPrompt(sys, prompt)
	streamCtx, streamCancel := context.WithCancel(ctx)
	ch, err := claude.RunStreaming(streamCtx, fullPrompt, diffText, model)
	if err != nil {
		streamCancel()
		return fmt.Errorf("starting claude: %w", err)
	}

	m := tui.New(diffText, gitRange, prompt, ch, model, streamCancel, persona)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
