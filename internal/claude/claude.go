package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const DefaultSystemPrompt = "You are reviewing a git diff. The diff is provided via stdin."

// CheckBinary verifies that the claude CLI is installed and available in PATH.
func CheckBinary() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH. Install it from https://docs.anthropic.com/en/docs/claude-code")
	}
	return nil
}

const DefaultUserPrompt = "Give a thorough code review with actionable feedback"

type StreamEvent struct {
	Content string
	Done    bool
	Err     error
}

// BuildPrompt combines a system prompt and user prompt into a single prompt string.
// If systemPrompt is empty, only the user prompt is returned.
func BuildPrompt(systemPrompt, userPrompt string) string {
	if systemPrompt == "" {
		return userPrompt
	}
	return systemPrompt + "\n\n" + userPrompt
}

// buildEnv returns a copy of the current environment variables
// with all CLAUDECODE_* variables removed. This is used to
// spawn child processes without propagating the Claude Code
// session variables.
func buildEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	return env
}

// baseArgs returns a slice of command-line arguments for invoking the Claude
// CLI. It configures the session with the provided prompt, model, and disables
// browser and slash command features. Tools are disabled; consider enabling
// read-only tools (Glob, Grep, Read) if LSP code intelligence requires them.
func baseArgs(prompt, model string) []string {
	return []string{
		"-p", prompt,
		"--tools", "",
		"--no-chrome", // Disable browser features
		"--disable-slash-commands",
		"--model", model,
	}
}

// RunSimple executes the Claude CLI with a given prompt and diff text,
// writing the output to the provided writer. It uses the specified model
// and runs synchronously, blocking until the command completes.
func RunSimple(ctx context.Context, prompt, diffText, model string, out io.Writer) error {
	args := baseArgs(prompt, model)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Env = buildEnv()
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(diffText)
	return cmd.Run()
}

// RunStreaming executes the Claude CLI in streaming mode to process a diff.
// It runs the claude command with the provided prompt, diff text, and model,
// returning a channel that streams StreamEvent messages as they are received.
// The caller must read from the returned channel until it is closed to ensure
// the process is properly waited on. The context can be used to cancel the
// operation. Returns an error if the command cannot be started or stdout pipe
// cannot be created.
func RunStreaming(ctx context.Context, prompt, diffText, model string) (<-chan StreamEvent, error) {
	args := append(baseArgs(prompt, model), "--output-format", "stream-json", "--verbose")
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Env = buildEnv()
	cmd.Stdin = strings.NewReader(diffText)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			event := parseStreamLine(line)
			if event != nil {
				ch <- *event
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Err: fmt.Errorf("reading claude output: %w", err)}
		}
	}()

	return ch, nil
}

// streamMessage represents a parsed streaming message from the Claude API.
// It handles multiple message formats:
//   - Standard messages with Type and ContentRaw
//   - Delta messages (streaming text) with Delta.Text
//   - Result messages with Result field
type streamMessage struct {
	Type       string          `json:"type"`
	SubType    string          `json:"subtype"`
	ContentRaw json.RawMessage `json:"content"`
	// For content_block_delta
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	// For result messages
	Result string `json:"result"`
}

// parseStreamLine parses a single line from a stream response and converts it
// into a StreamEvent. It handles different message types from the Claude stream:
//
//   - "assistant": Parses full assistant messages containing a content array
//     and extracts text content from text-type blocks.
//   - "content_block_delta": Extracts incremental text deltas from streaming
//     responses.
//   - "result": Handles final result messages, marking the stream as complete.
//   - "error": Wraps stream errors into a StreamEvent with an error.
//
// Returns nil if the line cannot be parsed or does not represent a meaningful
// event.
func parseStreamLine(line string) *StreamEvent {
	var msg streamMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "assistant":
		// Full assistant message with content array
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
				return &StreamEvent{Content: text.String()}
			}
		}
	case "content_block_delta":
		if msg.Delta.Text != "" {
			return &StreamEvent{Content: msg.Delta.Text}
		}
	case "result":
		if msg.Result != "" {
			return &StreamEvent{Content: msg.Result, Done: true}
		}
		return &StreamEvent{Done: true}
	case "error":
		return &StreamEvent{Err: fmt.Errorf("claude error: %s", line)}
	}

	return nil
}
