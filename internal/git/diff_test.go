package git

import (
	"context"
	"testing"
)

func TestDiff_RejectsDashPrefix(t *testing.T) {
	_, err := Diff(context.Background(), "--flag-injection")
	if err == nil {
		t.Fatal("expected error for range starting with -")
	}
}

func TestDiff_RejectsSingleDash(t *testing.T) {
	_, err := Diff(context.Background(), "-")
	if err == nil {
		t.Fatal("expected error for range starting with -")
	}
}
