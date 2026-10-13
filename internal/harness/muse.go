package harness

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

// MuseVolume is the Docker volume the operator's one-time Muse Code setup fills (docs/muse-code.md):
// the launcher and the binary it downloaded under bin/, the account login under config/.
const MuseVolume = "flow-eval-muse"

// _museBinDir and _museLoginDir are where a muse-code trial mounts the volume's bin/ and config/;
// _museEnv names them to the wrapper's shell, in the trial and in MuseReady alike.
const (
	_museBinDir   = "/muse/bin"
	_museLoginDir = "/muse/login"
	_museEnv      = "MUSE_INSTALL_DIR=" + _museBinDir + " FLOW_EVAL_MUSE_LOGIN=" + _museLoginDir
)

// MuseMounts returns the --mount values a muse-code trial gets, both read-only: the binary, so no
// trial changes what the next one runs, and the login, so nothing a trial writes reaches another
// trial. Each call returns a fresh slice.
func MuseMounts() []string {
	return []string{
		"type=volume,src=" + MuseVolume + ",dst=" + _museBinDir + ",volume-subpath=bin,readonly",
		"type=volume,src=" + MuseVolume + ",dst=" + _museLoginDir + ",volume-subpath=config,readonly",
	}
}

// _museCheck sets b to the binary the launcher's .muse-version names and l to the login, and exits 2
// naming the setup unless both are there.
const _museCheck = `b="$MUSE_INSTALL_DIR/muse-bin-$(cat "$MUSE_INSTALL_DIR/.muse-version" 2>/dev/null)"
l="$FLOW_EVAL_MUSE_LOGIN/muse/auth.json"
if [ ! -x "$b" ] || [ ! -s "$l" ]; then
	echo "muse-code: no Muse Code binary or login in the ` + MuseVolume + ` volume: run the Muse Code setup in docs/muse-code.md" >&2
	exit 2
fi`

// MuseReady is the shell script, run in the runner image with MuseMounts, that passes once the
// setup filled the volume: the trial wrapper's own check.
const MuseReady = _museEnv + "\n" + _museCheck

// Muse drives Meta's Muse Code CLI: muse exec with MSP JSONL records.
type Muse struct{}

var _ Harness = Muse{}

// Name implements Harness.
func (Muse) Name() string { return "muse-code" }

// _museScript runs the binary the setup installed — read from the launcher's .muse-version, never
// the launcher itself, so no trial self-updates or downloads — in workdir ($1), then appends the
// session logs of session id $2, main then subagents, to stdout and exits with muse's status. muse
// exec --json writes no token usage; only its session logs carry it. A volume without the binary or
// the login fails before muse runs, with no stdout, so the trial is an infra error naming the setup.
// The CLI's config home is the trial's own, holding only a copy of the read-only login: a token the
// CLI refreshes during the trial is not saved back.
const _museScript = _museCheck + `
mkdir -p "$XDG_CONFIG_HOME/muse" "$XDG_DATA_HOME" && cp "$l" "$XDG_CONFIG_HOME/muse/" && cd "$1" || exit 2
s="$2"
shift 2
"$b" "$@"
rc=$?
for f in "$XDG_DATA_HOME"/muse/sessions/*/*/*/"$s"/session.jsonl "$XDG_DATA_HOME"/muse/sessions/*/*/*/"$s"/subagent/*/session.jsonl; do
	if [ -f "$f" ]; then cat "$f"; fi
done
exit $rc`

// Command implements Harness. The prompt reaches muse by --prompt-file, a path, never as content.
// The container is the sandbox, so muse's own shell sandbox is off (--disable-sandbox): it needs a
// user namespace, which a trial container cannot create, so with it on every shell call fails.
// Approval prompts are off as well; reminders and web tools stay as the CLI ships them.
// XDG_CONFIG_HOME and XDG_DATA_HOME are the trial's own, created empty; the login is copied in from
// FLOW_EVAL_MUSE_LOGIN, the volume's read-only mount. Each call gets a fresh session id, so the logs
// Command collects are this trial's alone.
func (Muse) Command(promptFile, model, effort, workdir string) (argv, env []string) {
	sid := newUUID()
	argv = []string{"sh", "-c", _museScript, "sh", workdir, sid,
		"exec", "--json", "--model", model, "--reasoning-effort", effort,
		"--approval-mode", "never", "--approval-judge", "off", "--disable-sandbox", "--session-id", sid,
		"--workspace", workdir, "--prompt-file", promptFile}
	home := path.Join(_homeRoot, "muse-code")
	return argv, append(strings.Fields(_museEnv),
		"XDG_CONFIG_HOME="+path.Join(home, "config"), "XDG_DATA_HOME="+home)
}

// newUUID returns a random version 4 UUID, the form muse's --session-id takes.
func newUUID() string {
	var b [16]byte
	rand.Read(b[:]) // crypto/rand.Read never returns an error
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// museRecord is the subset of an MSP record — from stdout or a session log — Parse reads.
type museRecord struct {
	Stream struct {
		ID string `json:"id"`
	} `json:"stream"`
	PayloadType string `json:"payload_type"`
	Payload     struct {
		Text   string `json:"text"`    // run.terminal.*: the final message
		Reason string `json:"reason"`  // run.terminal.failed: why the run failed
		CallID string `json:"call_id"` // tool.result
		// CorrelationFacts is on every tool.result but a denied call's.
		CorrelationFacts *struct {
			ToolName string `json:"tool_name"`
			Outcome  string `json:"outcome"`
		} `json:"correlation_facts"`
		EditFacts *struct{} `json:"edit_facts"` // present on a tool result that edited a file
		Event     struct {
			Kind      string `json:"kind"`
			Model     string `json:"model"`
			ToolCalls []struct {
				CallID string `json:"call_id"`
				Name   string `json:"name"`
			} `json:"tool_calls"` // assistant_tool_calls_committed
			Usage struct {
				Input     int64 `json:"input_tokens"`
				Output    int64 `json:"output_tokens"`
				Cached    int64 `json:"cached_tokens"`
				Reasoning int64 `json:"reasoning_tokens"`
			} `json:"usage"`
		} `json:"event"` // runtime.session
	} `json:"payload"`
}

// Parse implements Harness. Tool calls and the final text come from muse's stdout records; one Call
// per model_completed event, across the main session's log and every subagent's; Turns counts the
// main session's (the first record's stream). A transcript without a run.terminal record (muse died
// mid-run) is an error. muse reports no cost and no context compaction. A run.terminal.failed whose
// reason is an API error (a 429 once the subscription quota is exhausted) is an Outage.
//
// A call muse denies is a permission denial, not a tool error: its tool.result carries no
// correlation_facts, only "tool denied: …", so its tool name comes from the session log's
// assistant_tool_calls_committed record with the same call_id, which follows stdout. Tool calls are
// therefore counted once the whole transcript is read, in stdout's order; a denied call with no
// committed record is a denial left out of ToolCalls.
//
// Token split, measured on a real five-tool-call muse-spark-1.3 run (testdata/muse-exec.jsonl):
//   - cached_tokens is inside input_tokens. Successive main-session calls report input_tokens
//     33226, 33510, 33827, … — the prompt growing by one tool round each — with cached_tokens 0,
//     32625, 33393, …; were cached tokens extra, the second call's prompt would jump to 66135.
//     Meta's pricing page bills "those tokens" of the prompt that match a cached prefix at the
//     cached rate. So Input is input_tokens less cached_tokens, and CacheRead is cached_tokens.
//   - cache_read_tokens repeats cached_tokens in every call, so it is not read a second time.
//   - reasoning_tokens is inside output_tokens. The final call answered one ~18-token sentence with
//     output_tokens 48 and reasoning_tokens 19; were reasoning extra, that sentence alone would be 48.
//   - cache_write_tokens is 0 in every call and the pricing page names no cache-write price: a
//     written token is billed as input, and input_tokens already counts it.
func (Muse) Parse(r io.Reader) (Usage, error) {
	var u Usage
	var main string // the main session's id
	var sawTerminal bool
	type result struct {
		callID, name string // name is "" for a denied call until the committed record names it
		edits        bool
	}
	var results []result
	names := map[string]string{} // call_id → tool name, from assistant_tool_calls_committed
	err := decodeEach(r, func(rec museRecord) {
		if main == "" {
			main = rec.Stream.ID
		}
		p := rec.Payload
		switch {
		case strings.HasPrefix(rec.PayloadType, "run.terminal."):
			sawTerminal = true
			u.FinalText = p.Text
			if rec.PayloadType == "run.terminal.failed" && strings.HasPrefix(p.Reason, "API error") {
				u.APIErrors++
				u.Outage = p.Reason
			}
		case rec.PayloadType == "tool.result" && p.CorrelationFacts == nil:
			u.PermissionDenials++
			results = append(results, result{callID: p.CallID})
		case rec.PayloadType == "tool.result":
			results = append(results, result{
				callID: p.CallID, name: p.CorrelationFacts.ToolName, edits: p.EditFacts != nil,
			})
			if p.CorrelationFacts.Outcome != "success" {
				u.ToolErrors++
			}
		case rec.PayloadType == "runtime.session" && p.Event.Kind == "assistant_tool_calls_committed":
			for _, c := range p.Event.ToolCalls {
				names[c.CallID] = c.Name
			}
		case rec.PayloadType == "runtime.session" && p.Event.Kind == "model_completed":
			t := p.Event.Usage
			c := Call{Input: t.Input - t.Cached, Output: t.Output, CacheRead: t.Cached}
			u.Calls = append(u.Calls, c)
			u.Input += c.Input
			u.Output += c.Output
			u.CacheRead += c.CacheRead
			u.Reasoning += t.Reasoning
			u.PeakContext = max(u.PeakContext, c.Prompt())
			if rec.Stream.ID == main {
				u.Turns++
			}
			if p.Event.Model != "" && !slices.Contains(u.ServedModels, p.Event.Model) {
				u.ServedModels = append(u.ServedModels, p.Event.Model)
				slices.Sort(u.ServedModels)
			}
		}
	})
	if err != nil {
		return Usage{}, err
	}
	if !sawTerminal {
		return Usage{}, errors.New("muse transcript has no run.terminal record")
	}
	for _, r := range results {
		if r.name == "" {
			r.name = names[r.callID]
		}
		if r.name != "" {
			u.countTool(r.name, r.edits)
		}
	}
	return u, nil
}
