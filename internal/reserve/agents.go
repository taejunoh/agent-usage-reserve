package reserve

import "fmt"

// agent describes where one coding agent keeps its global instructions and
// which policy text belongs in its managed block.
type agent struct {
	name, label string   // marker and flag name, display name
	homeFlag    string   // CLI flag selecting the home directory
	homeEnv     string   // environment variable read when the flag is absent
	defaultDir  string   // home directory relative to the user's home
	targets     []string // candidate files; a later entry wins when nonempty
	legacy      bool     // also parse codex-usage-reserve markers
	note        string   // caveat printed on every output line for this agent
	policy      func(threshold int) string
}

var codexAgent = agent{
	name:       "codex",
	label:      "Codex",
	homeFlag:   "codex-home",
	homeEnv:    "CODEX_HOME",
	defaultDir: ".codex",
	targets:    []string{"AGENTS.md", "AGENTS.override.md"},
	legacy:     true,
	policy:     codexPolicy,
}

var claudeAgent = agent{
	name:       "claude",
	label:      "Claude",
	homeFlag:   "claude-home",
	homeEnv:    "CLAUDE_CONFIG_DIR",
	defaultDir: ".claude",
	targets:    []string{"CLAUDE.md"},
	note:       "works in the Claude desktop app's Code tab",
	policy:     claudePolicy,
}

// agents is the fixed order for processing and printing.
var agents = []agent{claudeAgent, codexAgent}

const policyHeading = "## Account usage reserve\n\n"

// Unchanged from codex-usage-reserve 0.1.0.
const codexUsageCheck = `Before ordinary work and between major stages, check available Codex account usage limits with get_usage_limits or its namespaced equivalent. Prefer rateLimitsByLimitId over legacy rateLimits. Check the actual rate-limit windows applicable to the current task and model; do not stop because of unrelated special-model buckets. For each available window, remaining percent is 100 minus usedPercent, clamped to 0–100. Null or missing values/windows are unavailable, not zero usage.

`

const claudeUsageCheck = "Before ordinary work and between major stages, check Claude plan usage with the `get_usage` tool (it may be namespaced, e.g. `mcp__ccd_session_mgmt__get_usage`). Use `plan.windows`; for each window, remaining percent is 100 minus `percentUsed`, clamped to 0–100. Applicable windows are the 5-hour limit, the all-models weekly limit, and a per-model weekly limit only when it matches the model doing the current work; ignore other model buckets and extra usage. If `plan.status` is not `ok` or a value is missing, usage is unavailable, not zero.\n" +
	"\n" +
	"If you are a subagent, skip this check; the main session handles it.\n" +
	"\n"

// stopRules is shared by every agent; the Codex wording is unchanged from 0.1.0.
func stopRules(threshold int) string {
	return fmt.Sprintf(`If any applicable window has %d%% or less remaining, checkpoint progress safely and stop ordinary work. Tell the user the measured window and remaining percentage, that work stopped to reserve usage for scheduled tasks, the current progress, and how to resume.

Actual scheduled task runs are exempt from this stopping threshold; unrelated manual work is not. If the user explicitly asks to continue, resume only the specified task (or the current task if unspecified). Follow a user-specified lower temporary threshold; otherwise that task may finish. This permission does not change the default threshold for other tasks.

If the usage tool or data is unavailable, disclose that fact without guessing remaining usage, claiming the threshold was reached, or inventing another blocking rule. This is a behavioral operating policy, not a hard account cap; it cannot guarantee an exact reserve or scheduled-task success.
`, threshold)
}

func codexPolicy(threshold int) string {
	return policyHeading + codexUsageCheck + stopRules(threshold)
}

func claudePolicy(threshold int) string {
	return policyHeading + claudeUsageCheck + stopRules(threshold)
}
