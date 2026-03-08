package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/benstroud/lazyreview/internal/git"
	"github.com/benstroud/lazyreview/internal/harness"
)

// Run executes the lazyreview CLI workflow. It retrieves the git diff for the
// given range, checks if the diff exceeds the size threshold, and if so,
// prompts the user for confirmation before proceeding. Finally, it runs
// the harness with the provided prompt and diff text.
func Run(ctx context.Context, gitRange, prompt string, h harness.Harness) error {
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
	return h.RunSimple(ctx, prompt, diffText, os.Stdout)
}
