package harness

import (
	"cmp"
	"errors"
	"io"
	"maps"
	"slices"
)

// Claude drives Claude Code: claude -p with stream-json output.
type Claude struct{}

var _ Harness = Claude{}

// Name implements Harness.
func (Claude) Name() string { return "claude-code" }

// Command implements Harness. --bare is not used: it refuses the subscription
// CLAUDE_CODE_OAUTH_TOKEN; the empty CLAUDE_CONFIG_DIR gives the same clean slate (design.md §2).
// The agent runs as root in the runner container, where Claude Code refuses
// --dangerously-skip-permissions unless IS_SANDBOX=1 says the container is the sandbox.
// --include-partial-messages adds each call's message_delta, the one event carrying its final usage.
func (Claude) Command(promptFile, model, effort, workdir string) (argv, env []string) {
	argv, env = withHome("claude-code", "CLAUDE_CONFIG_DIR", workdir, promptFile,
		"claude", "-p", "--model", model, "--effort", effort,
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--dangerously-skip-permissions")
	return argv, append(env, "IS_SANDBOX=1")
}

// claudeTokens is the Anthropic API usage block.
type claudeTokens struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheRead  int64 `json:"cache_read_input_tokens"`
	CacheWrite int64 `json:"cache_creation_input_tokens"` // both TTLs
	// CacheCreation splits CacheWrite by TTL; subscription runs write 1-hour entries.
	CacheCreation struct {
		OneHour int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

func (t claudeTokens) call() Call {
	return Call{Input: t.Input, Output: t.Output, CacheRead: t.CacheRead,
		CacheWrite: t.CacheWrite - t.CacheCreation.OneHour, CacheWrite1h: t.CacheCreation.OneHour}
}

// claudeEvent is the subset of a stream-json event Parse reads.
type claudeEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		ID      string `json:"id"`
		Content []struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			IsError bool   `json:"is_error"`
		} `json:"content"`
		Usage claudeTokens `json:"usage"`
	} `json:"message"`

	// stream_event, written with --include-partial-messages. Only the host's newer CLI names the
	// message on every stream event (api_message_id); the runner's 2.1.286 names it on message_start alone.
	APIMessageID string `json:"api_message_id"`
	Event        struct {
		Type    string       `json:"type"`
		Usage   claudeTokens `json:"usage"` // message_delta
		Message struct {
			ID    string       `json:"id"`
			Usage claudeTokens `json:"usage"`
		} `json:"message"` // message_start
	} `json:"event"`

	// result event
	NumTurns     int          `json:"num_turns"`
	Result       string       `json:"result"`
	TotalCostUSD float64      `json:"total_cost_usd"`
	Usage        claudeTokens `json:"usage"`
	ModelUsage   map[string]struct {
		Input          int64 `json:"inputTokens"`
		Output         int64 `json:"outputTokens"`
		CacheRead      int64 `json:"cacheReadInputTokens"`
		CacheWrite     int64 `json:"cacheCreationInputTokens"`
		ThinkingTokens int64 `json:"thinkingTokens"`
	} `json:"modelUsage"`
	PermissionDenials []struct{} `json:"permission_denials"`
}

// _claudeEditTools are the tools that write a file.
var _claudeEditTools = []string{"Edit", "MultiEdit", "Write", "NotebookEdit"}

// Parse implements Harness. Totals come from the result events: a session that runs several turns
// writes one per turn, each with that turn's usage, turn count and permission denials, and each with
// the session's cumulative modelUsage and cost. A transcript without one (the CLI died mid-run) is
// an error. A session whose every result carries no token — a slash command whose skill ran in a
// forked agent the transcript skips, such as /code-review — takes its tokens from the last result's
// modelUsage instead. A call's final output is its message_delta's, attributed to the message the delta names
// or else to the one streaming; a transcript written without partial messages has only its assistant
// events, whose output is the count at its stream's start.
func (Claude) Parse(r io.Reader) (Usage, error) {
	var u Usage
	var sawResult bool
	var session Usage          // the last result's modelUsage tokens
	callAt := map[string]int{} // message id → its index in u.Calls
	// addCall records a call's usage the first time its id is seen: its earlier events split the cache
	// write by TTL, its message_delta does not, so the delta later sets only the final output.
	addCall := func(id string, c Call) {
		if _, ok := callAt[id]; !ok {
			callAt[id] = len(u.Calls)
			u.Calls = append(u.Calls, c)
		}
	}
	var streaming string // the message whose stream the last message_start opened
	err := decodeEach(r, func(ev claudeEvent) {
		switch ev.Type {
		case "system":
			switch ev.Subtype {
			case "api_retry":
				u.APIErrors++
			case "compact_boundary":
				u.Compactions++
			}
		case "assistant":
			c := ev.Message.Usage.call()
			u.PeakContext = max(u.PeakContext, c.Prompt())
			// A message streamed as several events repeats its id and the usage of its stream's start.
			addCall(ev.Message.ID, c)
			for _, c := range ev.Message.Content {
				if c.Type == "tool_use" {
					u.countTool(c.Name, slices.Contains(_claudeEditTools, c.Name))
				}
			}
		case "stream_event":
			switch ev.Event.Type {
			case "message_start":
				streaming = ev.Event.Message.ID
				if streaming != "" {
					addCall(streaming, ev.Event.Message.Usage.call())
				}
			case "message_delta":
				// ponytail: one open stream at a time, as every runner transcript shows; key it by
				// parent_tool_use_id if subagent streams ever interleave.
				if id := cmp.Or(ev.APIMessageID, streaming); id != "" {
					if i, ok := callAt[id]; ok {
						u.Calls[i].Output = ev.Event.Usage.Output
					}
				}
			}
		case "user":
			for _, c := range ev.Message.Content {
				if c.Type == "tool_result" && c.IsError {
					u.ToolErrors++
				}
			}
		case "result":
			sawResult = true
			c := ev.Usage.call()
			u.Input += c.Input
			u.Output += c.Output
			u.CacheRead += c.CacheRead
			u.CacheWrite += c.CacheWrite
			u.CacheWrite1h += c.CacheWrite1h
			u.Turns += ev.NumTurns
			u.PermissionDenials += len(ev.PermissionDenials)
			// Cumulative: the last result's figures are the session's.
			u.FinalText = ev.Result
			u.ReportedCostUSD = new(ev.TotalCostUSD)
			u.ServedModels = slices.Sorted(maps.Keys(ev.ModelUsage))
			u.Reasoning, session = 0, Usage{}
			for _, m := range ev.ModelUsage {
				u.Reasoning += m.ThinkingTokens
				session.Input += m.Input
				session.Output += m.Output
				session.CacheRead += m.CacheRead
				// modelUsage does not split cache writes by TTL; the CLI prices a forked command's
				// writes as 5-minute ones, its reported cost matching that pricing exactly.
				session.CacheWrite += m.CacheWrite
			}
		}
	})
	if err != nil {
		return Usage{}, err
	}
	if !sawResult {
		return Usage{}, errors.New("claude transcript has no result event")
	}
	if u.Tokens() == 0 {
		u.Input, u.Output, u.CacheRead, u.CacheWrite = session.Input, session.Output, session.CacheRead, session.CacheWrite
	}
	return u, nil
}
