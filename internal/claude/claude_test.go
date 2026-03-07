package claude

import (
	"os"
	"strings"
	"testing"
)

func TestParseStreamLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantNil     bool
		wantContent string
		wantDone    bool
		wantErr     bool
	}{
		{
			name:        "content_block_delta with text",
			line:        `{"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}`,
			wantContent: "hello",
		},
		{
			name:    "content_block_delta empty text",
			line:    `{"type":"content_block_delta","delta":{"type":"text_delta","text":""}}`,
			wantNil: true,
		},
		{
			name:        "assistant message with text content",
			line:        `{"type":"assistant","content":[{"type":"text","text":"review"}]}`,
			wantContent: "review",
		},
		{
			name:        "assistant message concatenates multiple text blocks",
			line:        `{"type":"assistant","content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]}`,
			wantContent: "hello world",
		},
		{
			name:    "assistant message with non-text content ignored",
			line:    `{"type":"assistant","content":[{"type":"tool_use","text":""}]}`,
			wantNil: true,
		},
		{
			name:    "assistant message with empty content array",
			line:    `{"type":"assistant","content":[]}`,
			wantNil: true,
		},
		{
			name:        "result with content",
			line:        `{"type":"result","result":"final answer"}`,
			wantContent: "final answer",
			wantDone:    true,
		},
		{
			name:     "result empty",
			line:     `{"type":"result","result":""}`,
			wantDone: true,
		},
		{
			name:    "error type",
			line:    `{"type":"error","message":"something went wrong"}`,
			wantErr: true,
		},
		{
			name:    "unknown type ignored",
			line:    `{"type":"ping"}`,
			wantNil: true,
		},
		{
			name:    "invalid json",
			line:    `not json`,
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStreamLine(tt.line)

			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil event")
			}

			if tt.wantErr {
				if got.Err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if got.Content != tt.wantContent {
				t.Errorf("content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.Done != tt.wantDone {
				t.Errorf("done = %v, want %v", got.Done, tt.wantDone)
			}
		})
	}
}

func TestBuildEnv_ExcludesCLAUDECODE(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("LAZYREVIEW_TEST_MARKER", "present")

	env := buildEnv()

	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Fatal("CLAUDECODE should be excluded from env")
		}
	}

	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "LAZYREVIEW_TEST_MARKER=") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected LAZYREVIEW_TEST_MARKER to be present")
	}
}

func TestBuildEnv_PreservesOtherVars(t *testing.T) {
	// Ensure HOME (or some standard var) passes through
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	env := buildEnv()
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("HOME should be present in env")
	}
}
