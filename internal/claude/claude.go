package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/benstroud/lazyreview/internal/harness"
	"github.com/benstroud/lazyreview/internal/stream"
)

const DefaultSystemPrompt = "You are reviewing a git diff. The diff is provided via stdin."
const DefaultUserPrompt = "Give a thorough code review with actionable feedback"

// CheckBinary verifies that the claude CLI is installed and available in PATH.
func CheckBinary() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH. Install it from https://docs.anthropic.com/en/docs/claude-code")
	}
	return nil
}

// BuildPrompt combines a system prompt and user prompt into a single prompt string.
// If systemPrompt is empty, only the user prompt is returned.
func BuildPrompt(systemPrompt, userPrompt string) string {
	if systemPrompt == "" {
		return userPrompt
	}
	return systemPrompt + "\n\n" + userPrompt
}

// buildEnv is a package-local alias so existing tests continue to call it directly.
func buildEnv() []string { return harness.BuildEnv() }

// Claude implements harness.Harness using the claude CLI.
type Claude struct{ model string }

// New returns a Claude harness configured with the given model name.
func New(model string) harness.Harness { return Claude{model: model} }

func (c Claude) Name() string { return "claude" }

func (c Claude) WithModel(model string) harness.Harness { return Claude{model: model} }

func (c Claude) baseArgs(prompt string) []string {
	return []string{
		"-p", prompt,
		"--tools", "",
		"--no-chrome",
		"--disable-slash-commands",
		"--model", c.model,
	}
}

func (c Claude) RunSimple(ctx context.Context, prompt, diffText string, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "claude", c.baseArgs(prompt)...)
	cmd.Env = harness.BuildEnv()
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(diffText)
	return cmd.Run()
}

func (c Claude) RunStreaming(ctx context.Context, prompt, diffText string) (<-chan stream.Event, error) {
	args := append(c.baseArgs(prompt), "--output-format", "stream-json", "--verbose")
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Env = harness.BuildEnv()
	cmd.Stdin = strings.NewReader(diffText)
	// doneOnEOF=false: the JSON "result" message is the protocol terminator.
	return harness.StreamLines(cmd, parseStreamLine, false)
}

// streamMessage represents a parsed streaming message from the Claude API.
type streamMessage struct {
	Type       string          `json:"type"`
	SubType    string          `json:"subtype"`
	ContentRaw json.RawMessage `json:"content"`
	Delta      struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Result string `json:"result"`
}

// parseStreamLine parses a single JSON line from the Claude stream into a
// stream.Event. Returns nil for empty lines, unknown types, or parse errors.
func parseStreamLine(line string) *stream.Event {
	if line == "" {
		return nil
	}
	var msg streamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "assistant":
		var contents []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(msg.ContentRaw, &contents); err == nil {
			var text strings.Builder
			for _, c := range contents {
				if c.Type == "text" {
					text.WriteString(c.Text)
				}
			}
			if text.Len() > 0 {
				return &stream.Event{Content: text.String()}
			}
		}
	case "content_block_delta":
		if msg.Delta.Text != "" {
			return &stream.Event{Content: msg.Delta.Text}
		}
	case "result":
		if msg.Result != "" {
			return &stream.Event{Content: msg.Result, Done: true}
		}
		return &stream.Event{Done: true}
	case "error":
		return &stream.Event{Err: fmt.Errorf("claude error: %s", line)}
	}

	return nil
}
