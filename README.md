# lazygaze

**AI-powered git diff review in your terminal.**

The `lazygaze` TUI pipes git diffs to Claude CLI or Github Copilot CLI with
streaming output, prompt library, and persona system. Diff on the left. Analysis
on the right. No browser, reduced context switching. Fast workflow.

[![Go](https://github.com/benstroud/lazygaze/actions/workflows/go.yml/badge.svg)](https://github.com/benstroud/lazygaze/actions/workflows/go.yml) [![Built with bubbletea](https://img.shields.io/badge/built%20with-bubbletea-ff69b4)](https://github.com/charmbracelet/bubbletea) [![Powered by Claude](https://img.shields.io/badge/powered%20by-Claude-orange)](https://www.anthropic.com) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

This software is licensed under the MIT [LICENSE](./LICENSE)

```diff
+ IMPORTANT
- User is responsible for all Claude or Copilot costs incurred and meeting their terms.
- Lazygaze simply delegates to these CLI tools which you configure.
```

---

![split panes](readme_assets/screenshot_output.png)
![prompt library](readme_assets/screenshot_prompt_library.png)
![personas](readme_assets/screenshot_personas.png)

---

## Features

| Feature | Description |
|---|---|
| **Split-pane TUI** | Syntax-highlighted diff left, streaming markdown review right |
| **Prompt library** | 26 curated prompts across architecture, security, performance, testing, and workflow |
| **Personas** | 53 reviewer archetypes across 6 categories; living people represented by descriptive titles |
| **Model cycling** | Switch between Sonnet, Opus, and Haiku on the fly |
| **Multiple diff sources** | Git ranges, staged changes, initial commit |
| **Clipboard** | Copy diff or review to clipboard with a single key |
| **CLI mode** | `--cli` for scripting and CI pipelines |
| **Persistent config** | Remembers your preferred model and persona |

---

## Requirements

### To build

- Go 1.26+

### To run

- `lazygaze` delegates to [Git](https://git-scm.com/) and [Claude CLI](https://github.com/anthropics/claude-code). It expects to find `git` and `claude` shell commands in your `PATH`. Ensure Claude is installed properly, authenticated (`claude --version`), and has credits/subscription.

---

## Installation

### Homebrew

```bash
brew install benstroud/tap/lazygaze
```

### From source

```bash
git clone https://github.com/benstroud/lazygaze
cd lazygaze
make build
sudo mv lazygaze /usr/local/bin/   # or add to your PATH
```

---

## Usage

```bash
# Launch TUI — enter a git range interactively
lazygaze

# Diff expression set at launch instead of interactively (Review the last 3 commits)
lazygaze HEAD~3..HEAD

# Prompt set a launch instead of interactively
lazygaze HEAD~1..HEAD "Focus on security vulnerabilities"

# Headless non-interactive output (pipes to stdout)
lazygaze --cli HEAD~1..HEAD

# Specify a model at launch instead of interactively
lazygaze --model opus HEAD~5..HEAD
```

---

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `tab` | Switch focus between diff and review panes |
| `?` | Open full keybinding help screen |
| `j` / `k` | Scroll focused pane |
| `q` / `ctrl+c` | Quit |

### Diff Source

| Key | Action |
|-----|--------|
| `S` | Review staged changes |
| `D` | Review dirty changes (uncommitted) |
| `^` | Review last commit (`HEAD^..HEAD`) |
| `U` | Review upstream changes (diff against upstream branch) |
| `~` | Shorthand: enter `n` to review `HEAD~n..HEAD` |
| `:` | Enter a git range (e.g. `HEAD~3..HEAD`, `main..feature`) |
| `r` | Refresh / re-run current diff |

### Review

| Key | Action |
|-----|--------|
| `/` | Set a custom prompt |
| `L` | Open prompt library |
| `P` | Select a reviewer persona |
| `m` | Cycle model: sonnet → opus → haiku |
| `c` | Copy focused pane to clipboard |
| `z` | Zoom in/out of the active pane. Useful before selecting text with mouse |

---

## Prompt Library

Press `L` to browse 26 built-in prompts organized by category:

- **Architecture** — layering violations, structural improvements
- **Bug Detection** — nil dereferences, race conditions, off-by-ones
- **Code Review** — thorough review with actionable feedback
- **Code Quality** — DRY, naming, error handling
- **Documentation** — README generation, missing comments
- **Performance** — allocations, N+1 queries, hot paths
- **Security** — injection, hardcoded secrets, auth logic
- **Testing** — edge cases, unit test suggestions, Gherkin AC
- **Workflow** — commit messages, PR descriptions, Jira tickets

---

## Personas

Press `P` to browse 53 reviewer archetypes organized into 6 categories:

- **CS Foundations** — structured programming, algorithmic rigor, formal correctness
- **Legendary Creators** — language designers, OS authors, paradigm inventors
- **Clean Code & Design** — readability, SOLID principles, refactoring
- **Systems & Performance** — low-level optimization, real-time constraints
- **Educators & Evangelists** — pedagogy, documentation, knowledge transfer
- **Influencers** — modern open-source voices and engineering culture

Special modes: **(Critical Only)** suppresses style feedback and reports only bugs, security issues, and data loss risks. **(Terse)** returns bullet points only.

---

## Development

```bash
make build    # compile
make test     # run tests
make clean    # remove binary
```

---

## Links

Also see my Zsh LLM code review helper snippets that were a precursor to this
project. Example:

```zsh
# Review the last commit of your feature branch
claudiff "HEAD^..HEAD" "code review"
```

[diffreview - Github](https://github.com/benstroud/diffreview)

## License

MIT
