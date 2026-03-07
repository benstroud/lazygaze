package tui

import "testing"

func TestPromptLibrary_NoEmpty(t *testing.T) {
	for i, e := range PromptLibrary {
		if e.Category == "" {
			t.Errorf("entry %d has empty category", i)
		}
		if e.Prompt == "" {
			t.Errorf("entry %d has empty prompt", i)
		}
	}
}

func TestPromptLibrary_CategoriesContiguous(t *testing.T) {
	seen := map[string]bool{}
	prev := ""
	for _, e := range PromptLibrary {
		if e.Category != prev && seen[e.Category] {
			t.Errorf("category %q appears non-contiguously", e.Category)
		}
		seen[e.Category] = true
		prev = e.Category
	}
}
