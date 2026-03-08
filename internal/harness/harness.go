package harness

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/benstroud/lazyreview/internal/stream"
)

// Harness is a CLI-backed LLM runner.
type Harness interface {
	// Name returns a short identifier shown in the UI (e.g. "claude", "copilot").
	Name() string
	// RunStreaming starts a review and streams events until done or ctx cancelled.
	RunStreaming(ctx context.Context, prompt, diffText string) (<-chan stream.Event, error)
	// RunSimple runs a review synchronously, writing output to w.
	RunSimple(ctx context.Context, prompt, diffText string, w io.Writer) error
	// WithModel returns a copy of the harness configured with the given model name.
	// Harnesses that do not use a model concept return themselves unchanged.
	WithModel(model string) Harness
}

// BuildEnv returns os.Environ() with CLAUDECODE= entries removed so that
// child processes do not inherit Claude Code session variables.
func BuildEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	return env
}

// StreamLines starts cmd, scans its stdout line-by-line, and calls parseLine
// for each line. Non-nil results are forwarded to the returned channel.
// If doneOnEOF is true, a Done event is synthesised after the final line (use
// this for plain-text protocols where EOF signals completion). Set doneOnEOF
// to false when parseLine itself emits the Done event (e.g. via a JSON sentinel).
func StreamLines(cmd *exec.Cmd, parseLine func(string) *stream.Event, doneOnEOF bool) (<-chan stream.Event, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	ch := make(chan stream.Event)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			if ev := parseLine(scanner.Text()); ev != nil {
				ch <- *ev
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- stream.Event{Err: fmt.Errorf("scanner: %w", err)}
			return
		}
		if doneOnEOF {
			ch <- stream.Event{Done: true}
		}
	}()
	return ch, nil
}
