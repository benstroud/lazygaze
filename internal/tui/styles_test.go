package tui

import (
	"strings"
	"testing"
)

func TestColorizeDiff_PreservesLineCount(t *testing.T) {
	input := "+added\n-removed\n context\n@@ -1 +1 @@\ndiff --git a/f b/f"
	result := colorizeDiff(input)
	inputLines := strings.Split(input, "\n")
	resultLines := strings.Split(result, "\n")
	if len(resultLines) != len(inputLines) {
		t.Errorf("line count changed: input %d, output %d", len(inputLines), len(resultLines))
	}
}

func TestColorizeDiff_ContextLinesUnchanged(t *testing.T) {
	input := " context line"
	result := colorizeDiff(input)
	// Context lines (space prefix) should not be modified
	if result != input {
		t.Errorf("context line should be unchanged, got %q", result)
	}
}

func TestColorizeDiff_ContainsOriginalText(t *testing.T) {
	input := "+added\n-removed\n@@ -1,3 +1,3 @@"
	result := colorizeDiff(input)
	for _, text := range []string{"added", "removed", "@@ -1,3 +1,3 @@"} {
		if !strings.Contains(result, text) {
			t.Errorf("result should contain %q", text)
		}
	}
}

func TestColorizeDiff_Empty(t *testing.T) {
	if colorizeDiff("") != "" {
		t.Error("empty input should return empty")
	}
}
