package copilot

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/benstroud/lazyreview/internal/harness"
	"github.com/benstroud/lazyreview/internal/stream"
)

// CheckBinary verifies that the copilot CLI is installed and available in PATH.
func CheckBinary() error {
	_, err := exec.LookPath("copilot")
	if err != nil {
		return fmt.Errorf("copilot CLI not found in PATH")
	}
	return nil
}

// Copilot implements harness.Harness using the copilot CLI.
type Copilot struct{}

// New returns a Copilot harness.
func New() harness.Harness { return Copilot{} }

func (c Copilot) Name() string { return "copilot" }

// WithModel is a no-op: copilot uses model selection implicitly.
func (c Copilot) WithModel(_ string) harness.Harness { return c }

func (c Copilot) baseArgs(prompt string) []string {
	return []string{
		"--disable-builtin-mcps",
		"--deny-tool", "shell",
		"--deny-tool", "url",
		"--deny-tool", "write",
		"--deny-tool", "memory",
		"--silent",
		"--prompt", prompt,
	}
}

func buildPromptWithDiff(prompt, diffText string) (string, error) {
	f, err := os.CreateTemp("", "lazyreview-diff-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(diffText); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	content, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}
	return prompt + "\n\n" + string(content), nil
}

func (c Copilot) RunSimple(ctx context.Context, prompt, diffText string, w io.Writer) error {
	combined, err := buildPromptWithDiff(prompt, diffText)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "copilot", c.baseArgs(combined)...)
	cmd.Env = harness.BuildEnv()
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (c Copilot) RunStreaming(ctx context.Context, prompt, diffText string) (<-chan stream.Event, error) {
	combined, err := buildPromptWithDiff(prompt, diffText)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "copilot", c.baseArgs(combined)...)
	cmd.Env = harness.BuildEnv()
	// doneOnEOF=true: copilot emits plain text; EOF is the protocol terminator.
	return harness.StreamLines(cmd, func(line string) *stream.Event {
		return &stream.Event{Content: line + "\n"}
	}, true)
}
