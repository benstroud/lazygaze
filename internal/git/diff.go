package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

var ErrNoDiffOutput = errors.New("no diff output")

// MaxDiffLines is the line count threshold above which users are warned before
// sending a diff to the LLM.
const MaxDiffLines = 5000

// LineCount returns the number of lines in s.
func LineCount(s string) int {
	return strings.Count(s, "\n")
}

// IsLargeDiff reports whether diff exceeds MaxDiffLines.
func IsLargeDiff(diff string) bool {
	return LineCount(diff) > MaxDiffLines
}

// CheckBinary verifies that git is installed and available in PATH.
func CheckBinary() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git not found in PATH. Install it from https://git-scm.com")
	}
	return nil
}

// runGitCmd executes a git command with the given arguments and returns its output.
// It runs the command within the provided context, allowing for cancellation or timeout.
// If the command fails, it returns an error that includes the git command that was run
// and, for exit errors, includes the stderr output for better debugging.
func runGitCmd(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		label := "git " + strings.Join(args, " ")
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s failed: %s", label, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("%s failed: %w", label, err)
	}
	return string(out), nil
}

// Diff returns the diff output for a given git range.
// The gitRange parameter should be a valid git range (e.g., "HEAD~1..HEAD").
// Returns an error if the range starts with "-" (to avoid flag confusion),
// if the git command fails, or if there is no diff output for the given range.
func Diff(ctx context.Context, gitRange string) (string, error) {
	if strings.HasPrefix(gitRange, "-") {
		return "", fmt.Errorf("invalid git range %q", gitRange)
	}
	result, err := runGitCmd(ctx, "diff", gitRange)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("%w for range %q", ErrNoDiffOutput, gitRange)
	}
	return result, nil
}

// DiffStaged returns the diff output for staged changes in the repository.
// It runs "git diff --cached" to retrieve changes that have been staged
// but not yet committed. Returns an error if there are no staged changes
// or if the git command fails.
func DiffStaged(ctx context.Context) (string, error) {
	result, err := runGitCmd(ctx, "diff", "--cached")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("%w for staged changes", ErrNoDiffOutput)
	}
	return result, nil
}

// CommitCount returns the total number of commits in the current Git repository.
// It executes "git rev-list --count HEAD" to count all commits reachable from HEAD.
// Returns the commit count as an integer, or an error if the git command fails.
func CommitCount(ctx context.Context) (int, error) {
	result, err := runGitCmd(ctx, "rev-list", "--count", "HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(result))
}

// DiffHEAD returns the diff of all uncommitted changes (staged + unstaged) vs HEAD.
func DiffHEAD(ctx context.Context) (string, error) {
	result, err := runGitCmd(ctx, "diff", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("%w for HEAD. No uncommitted changes.", ErrNoDiffOutput)
	}
	return result, nil
}

// DiffRoot returns the diff output for the initial/root commit of the repo.
// It finds the root commit (earliest ancestor of HEAD) and runs "git show" on it.
func DiffRoot(ctx context.Context) (string, error) {
	hash, err := runGitCmd(ctx, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", err
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return "", fmt.Errorf("%w for initial commit", ErrNoDiffOutput)
	}
	// If multiple roots exist, use the first one.
	if i := strings.IndexByte(hash, '\n'); i >= 0 {
		hash = hash[:i]
	}
	result, err := runGitCmd(ctx, "show", hash)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("%w for initial commit", ErrNoDiffOutput)
	}
	return result, nil
}

// GetUpstreamBranch returns the name of the upstream branch configured for the current HEAD.
// It uses git's @{upstream} shorthand to resolve the configured upstream branch.
// Returns an error if no upstream is configured or if the git command fails.
func GetUpstreamBranch(ctx context.Context) (string, error) {
	result, err := runGitCmd(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		// Only translate the specific "no upstream" error to a helpful message
		if strings.Contains(err.Error(), "fatal: no upstream configured") ||
			strings.Contains(err.Error(), "does not have any upstream") {
			return "", fmt.Errorf("no upstream branch configured — set with `git branch --set-upstream-to <remote>/<branch>`")
		}
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// DiffUpstream returns the diff between the current branch and its configured upstream branch.
// It returns the diff text, the upstream branch name, and any error encountered.
// Returns an error if no upstream is configured or if the git command fails.
// Returns an empty diff (but no error) when the branches are in sync.
func DiffUpstream(ctx context.Context) (string, string, error) {
	upstream, err := GetUpstreamBranch(ctx)
	if err != nil {
		return "", "", err
	}
	rangeSpec := upstream + "..HEAD"
	result, err := runGitCmd(ctx, "diff", rangeSpec)
	if err != nil {
		return "", "", err
	}
	return result, upstream, nil
}
