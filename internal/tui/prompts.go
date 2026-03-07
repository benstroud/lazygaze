package tui

// PromptEntry represents a single prompt in the library.
// NoPersona disables persona injection for entries where a character voice
// would hurt output quality (e.g. generating commit messages or Jira tickets).
type PromptEntry struct {
	Category  string
	Prompt    string
	NoPersona bool
}

// PromptLibrary is the predefined set of review prompts, sorted by category.
var PromptLibrary = []PromptEntry{
	{Category: "Architecture", Prompt: "Evaluate architectural decisions and suggest structural improvements"},
	{Category: "Architecture", Prompt: "Identify layering violations or misplaced responsibilities"},
	{Category: "Bug Detection", Prompt: "Find potential nil pointer dereferences and unhandled errors"},
	{Category: "Bug Detection", Prompt: "Look for race conditions and concurrency issues"},
	{Category: "Bug Detection", Prompt: "Spot off-by-one errors and boundary condition bugs"},
	{Category: "Code Quality", Prompt: "Check for code duplication and suggest DRY improvements"},
	{Category: "Code Quality", Prompt: "Evaluate naming conventions and code readability"},
	{Category: "Code Quality", Prompt: "Review error handling patterns for consistency"},
	{Category: "Code Review", Prompt: "Give a thorough code review with actionable feedback"},
	{Category: "Documentation", Prompt: "Identify missing or outdated comments and documentation"},
	{Category: "Documentation", Prompt: "Generate README.md - highly professional", NoPersona: true},
	{Category: "Documentation", Prompt: "Generate README.md - bring the hype cycle!", NoPersona: true},
	{Category: "Documentation", Prompt: "Generate README.md - explained over coffee", NoPersona: true},
	{Category: "Performance", Prompt: "Find unnecessary allocations and memory inefficiencies"},
	{Category: "Performance", Prompt: "Identify performance bottlenecks and hot paths"},
	{Category: "Performance", Prompt: "Spot N+1 queries and database performance issues"},
	{Category: "Security", Prompt: "Check for command injection and input validation issues"},
	{Category: "Security", Prompt: "Look for hardcoded secrets and credential exposure"},
	{Category: "Security", Prompt: "Review authentication and authorization logic"},
	{Category: "Testing", Prompt: "Generate Gherkin acceptance criteria", NoPersona: true},
	{Category: "Testing", Prompt: "Identify untested edge cases and missing test coverage"},
	{Category: "Testing", Prompt: "Suggest unit tests for the changed code"},
	{Category: "Workflow", Prompt: "Generate Git commit message", NoPersona: true},
	{Category: "Workflow", Prompt: "Generate Git pull request message", NoPersona: true},
	{Category: "Workflow", Prompt: "Generate Jira ticket description and AC", NoPersona: true},
	{Category: "Workflow", Prompt: "Suggest how these commits should be squashed and organized"},
}
