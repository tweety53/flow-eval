// Package harness drives a coding-agent CLI headless inside a trial container and normalizes its
// JSONL transcript into one Usage, the same way for every harness (design.md "Harness adapters").
package harness

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"path"
)

// Harness is one headless agent CLI.
type Harness interface {
	// Name is the harness id the run command takes: claude-code, codex or muse-code.
	Name() string
	// Command returns the argv that runs the prompt in the container file promptFile at model and
	// effort in workdir, and the environment entries (KEY=value) it needs on top of the trial's
	// credentials. The prompt reaches the CLI on stdin or by its path, never as argument content:
	// Linux caps one argument at 128 KiB and a review prompt carries a whole diff. The argv creates
	// the CLI's config home in a fresh container before starting the CLI, holding nothing but what
	// the harness's login needs, so no user configuration reaches the agent.
	Command(promptFile, model, effort, workdir string) (argv, env []string)
	// Parse reads the CLI's stdout transcript and returns its normalized usage.
	Parse(r io.Reader) (Usage, error)
}

// Usage is a transcript normalized across harnesses. Token counts follow the Anthropic split: Input
// is uncached input, CacheRead the cached input read, CacheWrite the input written to a 5-minute
// (or the harness's only) cache and CacheWrite1h to a 1-hour cache, priced higher; Reasoning is
// already counted inside Output.
type Usage struct {
	Input, Output, CacheRead, CacheWrite, CacheWrite1h, Reasoning int64

	// Turns is the harness's own turn count: model round trips for claude-code, while codex exec
	// reports a whole run as one turn.
	Turns             int
	ToolCalls         map[string]int // tool name → calls
	ToolErrors        int            // tool calls that failed
	PermissionDenials int
	APIErrors         int // API errors and retries the CLI reported
	Compactions       int
	PeakContext       int64 // the largest input context of any one model call; 0 when the CLI does not say
	// FirstEditAt is the 1-based position, among all tool calls, of the first one that edits a file;
	// 0 when none does. Not every CLI timestamps its events, so a wall-clock offset cannot be derived
	// from the transcript.
	FirstEditAt     int
	ServedModels    []string // model ids that answered, sorted; empty when the CLI does not say
	ReportedCostUSD *float64 // the CLI's own cost figure, nil when it reports none
	FinalText       string   // the agent's final message
	// Outage is the API error that ended the run before its final message — a provider outage or an
	// exhausted quota, not the agent's doing; "" when the run ended on its own.
	Outage string
	// Calls is each model call's tokens, one per API message, in order; nil when the CLI does not
	// report them per call (codex). A long-prompt price tier is applied per call.
	Calls []Call
}

// Call is one model call's tokens, split as in Usage: final when the transcript carries the call's
// message_delta (claude-code with --include-partial-messages); otherwise its output is the count at
// its stream's start, short of the final one.
type Call struct {
	Input, Output, CacheRead, CacheWrite, CacheWrite1h int64
}

// Prompt is c's prompt size: every input token, cache reads and both cache writes included.
func (c Call) Prompt() int64 { return c.Input + c.CacheRead + c.CacheWrite + c.CacheWrite1h }

// Tokens is u's tokens of every class; Reasoning is already inside Output.
func (u Usage) Tokens() int64 {
	return u.Input + u.Output + u.CacheRead + u.CacheWrite + u.CacheWrite1h
}

// _homeRoot holds each harness's config home. It is outside /work, so it never shows in the
// agent's diff, and it is empty because every trial runs in a fresh container.
const _homeRoot = "/tmp/flow-eval-home"

// withHome wraps cli so that it runs in workdir with homeVar set to a freshly created config home
// and promptFile on its stdin. Positional arguments carry workdir, promptFile and cli through sh
// unquoted, so no argument is re-parsed.
func withHome(name, homeVar, workdir, promptFile string, cli ...string) (argv, env []string) {
	script := fmt.Sprintf(`mkdir -p "$%s" && cd "$1" && p="$2" && shift 2 && exec "$@" < "$p"`, homeVar)
	argv = append([]string{"sh", "-c", script, "sh", workdir, promptFile}, cli...)
	return argv, []string{homeVar + "=" + path.Join(_homeRoot, name)}
}

// decodeEach decodes every JSON value in r into a fresh T and passes it to fn, in order. It accepts
// invalid UTF-8 and duplicate names: a CLI that cuts a tool output mid-character emits a lone
// surrogate escape, and one such line must not lose the whole trial.
func decodeEach[T any](r io.Reader, fn func(T)) error {
	dec := jsontext.NewDecoder(r, jsontext.AllowInvalidUTF8(true), jsontext.AllowDuplicateNames(true))
	for n := 1; ; n++ {
		var ev T
		err := json.UnmarshalDecode(dec, &ev)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("transcript event %d: %w", n, err)
		}
		fn(ev)
	}
}

// countTool records one tool call and, for the first file edit, its position.
func (u *Usage) countTool(name string, edits bool) {
	if u.ToolCalls == nil {
		u.ToolCalls = map[string]int{}
	}
	u.ToolCalls[name]++
	if edits && u.FirstEditAt == 0 {
		total := 0
		for _, n := range u.ToolCalls {
			total += n
		}
		u.FirstEditAt = total
	}
}
