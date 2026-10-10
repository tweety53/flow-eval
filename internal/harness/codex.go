package harness

import (
	"io"
)

// Codex drives the Codex CLI: codex exec with JSONL events.
type Codex struct{}

var _ Harness = Codex{}

// Name implements Harness.
func (Codex) Name() string { return "codex" }

// Command implements Harness.
func (Codex) Command(promptFile, model, effort, workdir string) (argv, env []string) {
	return withHome("codex", "CODEX_HOME", workdir, promptFile,
		"codex", "exec", "--json", "--skip-git-repo-check", "-m", model,
		"-c", "model_reasoning_effort="+effort, "--dangerously-bypass-approvals-and-sandbox", "-")
}

// codexEvent is the subset of a codex exec --json event Parse reads.
type codexEvent struct {
	Type string `json:"type"`
	Item struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Text   string `json:"text"`
	} `json:"item"`
	Usage struct {
		Input      int64 `json:"input_tokens"`
		Cached     int64 `json:"cached_input_tokens"`
		CacheWrite int64 `json:"cache_write_input_tokens"`
		Output     int64 `json:"output_tokens"`
		Reasoning  int64 `json:"reasoning_output_tokens"`
	} `json:"usage"`
}

// _codexToolItems are the item types that are tool calls, mapped to whether they edit a file.
var _codexToolItems = map[string]bool{
	"command_execution": false,
	"file_change":       true,
	"mcp_tool_call":     false,
	"web_search":        false,
}

// Parse implements Harness. Usage is summed over turn.completed events. codex exec names no served
// model and reports no cost, so ServedModels and ReportedCostUSD stay empty; its turn usage sums
// every model request in the turn, so no single call's context is known and PeakContext stays 0.
func (Codex) Parse(r io.Reader) (Usage, error) {
	var u Usage
	err := decodeEach(r, func(ev codexEvent) {
		switch ev.Type {
		case "turn.completed":
			t := ev.Usage
			u.Turns++
			// input_tokens counts the whole prompt, cached and cache-written parts included.
			u.Input += t.Input - t.Cached - t.CacheWrite
			u.CacheRead += t.Cached
			u.CacheWrite += t.CacheWrite
			u.Output += t.Output
			u.Reasoning += t.Reasoning
		case "turn.failed", "error":
			u.APIErrors++
		case "item.completed":
			if ev.Item.Type == "agent_message" {
				u.FinalText = ev.Item.Text
			}
			edits, isTool := _codexToolItems[ev.Item.Type]
			if !isTool {
				return
			}
			u.countTool(ev.Item.Type, edits)
			if ev.Item.Status == "failed" {
				u.ToolErrors++
			}
		}
	})
	if err != nil {
		return Usage{}, err
	}
	return u, nil
}
