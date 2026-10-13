package harness

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The transcripts in testdata are built from the event schemas compiled into claude 2.1.295 and
// codex-cli 0.145.0 (the result, api_retry and compact_boundary schemas; the token-usage field
// names), not captured from a live run: the capture was refused in the implementing session.
// Replace them with a real capture from the runner image when one is taken. claude.jsonl carries a
// tool output cut mid-emoji (a lone surrogate escape), as Node emits it.

func parseFile(t *testing.T, h Harness, name string) Usage {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	u, err := h.Parse(f)
	if err != nil {
		t.Fatalf("%s Parse(%s): %v", h.Name(), name, err)
	}
	return u
}

func TestClaudeCommandArgs(t *testing.T) {
	argv, env := Claude{}.Command("/tmp/prompt.md", "sonnet", "low", "/work")
	want := []string{
		"sh", "-c", `mkdir -p "$CLAUDE_CONFIG_DIR" && cd "$1" && p="$2" && shift 2 && exec "$@" < "$p"`,
		"sh", "/work", "/tmp/prompt.md",
		"claude", "-p", "--model", "sonnet", "--effort", "low",
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--dangerously-skip-permissions",
	}
	if !slices.Equal(argv, want) {
		t.Errorf("argv =\n%q\nwant\n%q", argv, want)
	}
	if wantEnv := []string{"CLAUDE_CONFIG_DIR=/tmp/flow-eval-home/claude-code", "IS_SANDBOX=1"}; !slices.Equal(env, wantEnv) {
		t.Errorf("env = %q, want %q", env, wantEnv)
	}
}

func TestClaudeParseUsage(t *testing.T) {
	got := parseFile(t, Claude{}, "testdata/claude.jsonl")
	want := Usage{
		Input: 12, Output: 120, CacheRead: 18000, CacheWrite: 300, CacheWrite1h: 5000, Reasoning: 30,
		Turns:             4,
		ToolCalls:         map[string]int{"Bash": 1, "Read": 1, "Write": 1},
		ToolErrors:        1,
		PermissionDenials: 1,
		APIErrors:         1,
		Compactions:       1,
		PeakContext:       7902,
		FirstEditAt:       3,
		ServedModels:      []string{"claude-sonnet-5-5"},
		ReportedCostUSD:   new(0.0321),
		FinalText:         "done",
		Calls: []Call{
			{Input: 3, Output: 40, CacheWrite: 5000},
			{Input: 4, Output: 30, CacheRead: 5000, CacheWrite: 200},
			{Input: 3, Output: 45, CacheRead: 5200},
			{Input: 2, Output: 5, CacheRead: 7800, CacheWrite: 100},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
	}
}

func TestClaudeParseCalls(t *testing.T) {
	tests := []struct {
		name       string
		transcript string
		want       []Call
	}{
		{
			// The runner's CLI (2.1.286) with --include-partial-messages, as a real trial wrote it
			// (review kan-961, trimmed): a message's id is on its message_start and assistant events
			// only, its message_delta names no message, and the delta's output is the final one.
			name: "runner CLI names the message on message_start only",
			transcript: `{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_start","message":{"id":"msg_011CftBJRuQrteYWW88ywgmu","usage":{"input_tokens":2,"cache_creation_input_tokens":30360,"cache_read_input_tokens":18687,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":30360},"output_tokens":4}}}}
{"type":"assistant","message":{"id":"msg_011CftBJRuQrteYWW88ywgmu","content":[{"type":"thinking"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":30360,"cache_read_input_tokens":18687,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":30360},"output_tokens":4}}}
{"type":"assistant","message":{"id":"msg_011CftBJRuQrteYWW88ywgmu","content":[{"type":"tool_use","name":"Bash"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":30360,"cache_read_input_tokens":18687,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":30360},"output_tokens":4}}}
{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_delta","usage":{"input_tokens":2,"cache_creation_input_tokens":30360,"cache_read_input_tokens":18687,"output_tokens":5218}}}
{"type":"user","message":{"content":[{"type":"tool_result"}]}}
{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_start","message":{"id":"msg_011CftBLBQqLzuy5ZLLuQ4dx","usage":{"input_tokens":2,"cache_creation_input_tokens":5714,"cache_read_input_tokens":49047,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":5714},"output_tokens":21}}}}
{"type":"assistant","message":{"id":"msg_011CftBLBQqLzuy5ZLLuQ4dx","content":[{"type":"tool_use","name":"Bash"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":5714,"cache_read_input_tokens":49047,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":5714},"output_tokens":21}}}
{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_delta","usage":{"input_tokens":2,"cache_creation_input_tokens":5714,"cache_read_input_tokens":49047,"output_tokens":175}}}
{"type":"user","message":{"content":[{"type":"tool_result"}]}}
{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_start","message":{"id":"msg_011CftBLGUS7NDpXyMNf8D1b","usage":{"input_tokens":2,"cache_creation_input_tokens":819,"cache_read_input_tokens":54761,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":819},"output_tokens":6}}}}
{"type":"assistant","message":{"id":"msg_011CftBLGUS7NDpXyMNf8D1b","content":[{"type":"text"}],"usage":{"input_tokens":2,"cache_creation_input_tokens":819,"cache_read_input_tokens":54761,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":819},"output_tokens":6}}}
{"type":"stream_event","parent_tool_use_id":null,"event":{"type":"message_delta","usage":{"input_tokens":2,"cache_creation_input_tokens":819,"cache_read_input_tokens":54761,"output_tokens":4222}}}
{"type":"result","subtype":"success","num_turns":3,"result":"done","usage":{"input_tokens":6,"cache_creation_input_tokens":36893,"cache_read_input_tokens":122495,"cache_creation":{"ephemeral_1h_input_tokens":36893,"ephemeral_5m_input_tokens":0},"output_tokens":9615}}
`,
			want: []Call{
				{Input: 2, Output: 5218, CacheRead: 18687, CacheWrite1h: 30360},
				{Input: 2, Output: 175, CacheRead: 49047, CacheWrite1h: 5714},
				{Input: 2, Output: 4222, CacheRead: 54761, CacheWrite1h: 819},
			},
		},
		{
			// The host's newer CLI names the message on every stream event (api_message_id). msg_a's
			// prompt is over 100k tokens and writes both cache TTLs; a message_start naming no message
			// is no call, and its delta is not msg_a's; msg_b, from a transcript without partial
			// messages, has only its assistant event.
			name: "host CLI names the message on every stream event",
			transcript: `{"type":"stream_event","api_message_id":"msg_a","event":{"type":"message_start","message":{"id":"msg_a","usage":{"input_tokens":7,"cache_read_input_tokens":110000,"cache_creation_input_tokens":900,"cache_creation":{"ephemeral_1h_input_tokens":600},"output_tokens":3}}}}
{"type":"assistant","message":{"id":"msg_a","content":[{"type":"tool_use","name":"Read"}],"usage":{"input_tokens":7,"cache_read_input_tokens":110000,"cache_creation_input_tokens":900,"cache_creation":{"ephemeral_1h_input_tokens":600},"output_tokens":3}}}
{"type":"stream_event","api_message_id":"msg_a","event":{"type":"message_delta","usage":{"input_tokens":7,"cache_read_input_tokens":110000,"cache_creation_input_tokens":900,"output_tokens":70}}}
{"type":"stream_event","event":{"type":"message_start","message":{"usage":{"input_tokens":99}}}}
{"type":"stream_event","event":{"type":"message_delta","usage":{"input_tokens":99,"output_tokens":500}}}
{"type":"assistant","message":{"id":"msg_b","content":[{"type":"text"}],"usage":{"input_tokens":1,"cache_read_input_tokens":500,"output_tokens":2}}}
{"type":"result","subtype":"success","num_turns":2,"result":"done","usage":{"input_tokens":8,"cache_read_input_tokens":110500,"cache_creation_input_tokens":900,"cache_creation":{"ephemeral_1h_input_tokens":600},"output_tokens":72}}
`,
			want: []Call{
				{Input: 7, Output: 70, CacheRead: 110000, CacheWrite: 300, CacheWrite1h: 600},
				{Input: 1, Output: 2, CacheRead: 500},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Claude{}.Parse(strings.NewReader(tt.transcript))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got.Calls, tt.want) {
				t.Errorf("Calls =\n%+v\nwant\n%+v", got.Calls, tt.want)
			}
			var out int64
			for _, c := range got.Calls {
				out += c.Output
			}
			if out != got.Output {
				t.Errorf("calls' output sums to %d, the result's is %d", out, got.Output)
			}
		})
	}
}

func TestClaudeParseSumsEveryResult(t *testing.T) {
	// A session that ran two turns writes two result events: usage, num_turns and
	// permission_denials per turn; modelUsage and total_cost_usd for the whole session so far.
	const transcript = `{"type":"result","num_turns":3,"result":"first","total_cost_usd":0.5,"usage":{"input_tokens":10,"cache_read_input_tokens":1000,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_1h_input_tokens":40},"output_tokens":50},"modelUsage":{"m":{"thinkingTokens":5}},"permission_denials":[{}]}
{"type":"result","num_turns":2,"result":"second","total_cost_usd":0.8,"usage":{"input_tokens":3,"cache_read_input_tokens":200,"cache_creation_input_tokens":20,"cache_creation":{"ephemeral_1h_input_tokens":20},"output_tokens":7},"modelUsage":{"m":{"thinkingTokens":9},"side":{"thinkingTokens":1}},"permission_denials":[{}]}
`
	got, err := Claude{}.Parse(strings.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	want := Usage{
		Input: 13, Output: 57, CacheRead: 1200, CacheWrite: 60, CacheWrite1h: 60, Reasoning: 10,
		Turns: 5, PermissionDenials: 2, ServedModels: []string{"m", "side"},
		ReportedCostUSD: new(0.8), FinalText: "second",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Usage =\n%+v\nwant\n%+v", got, want)
	}
}

func TestClaudeParseRejectsTranscriptWithoutResult(t *testing.T) {
	data, err := os.ReadFile("testdata/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	initOnly := data[:bytes.IndexByte(data, '\n')+1]
	if _, err := (Claude{}).Parse(bytes.NewReader(initOnly)); err == nil {
		t.Fatal("Parse of a transcript with no result event succeeded, want an error")
	}
}

func TestCodexCommandArgs(t *testing.T) {
	argv, env := Codex{}.Command("/tmp/prompt.md", "gpt-5.6-terra", "high", "/work")
	want := []string{
		"sh", "-c", `mkdir -p "$CODEX_HOME" && cd "$1" && p="$2" && shift 2 && exec "$@" < "$p"`,
		"sh", "/work", "/tmp/prompt.md",
		"codex", "exec", "--json", "--skip-git-repo-check", "-m", "gpt-5.6-terra",
		"-c", "model_reasoning_effort=high", "--dangerously-bypass-approvals-and-sandbox", "-",
	}
	if !slices.Equal(argv, want) {
		t.Errorf("argv =\n%q\nwant\n%q", argv, want)
	}
	if wantEnv := []string{"CODEX_HOME=/tmp/flow-eval-home/codex"}; !slices.Equal(env, wantEnv) {
		t.Errorf("env = %q, want %q", env, wantEnv)
	}
}

func TestCodexParseUsage(t *testing.T) {
	got := parseFile(t, Codex{}, "testdata/codex.jsonl")
	want := Usage{
		// Codex's input_tokens include the cached and cache-written tokens; Input is the rest.
		Input: 5500, Output: 350, CacheRead: 19000, CacheWrite: 500, Reasoning: 130,
		Turns:       2,
		ToolCalls:   map[string]int{"command_execution": 2, "file_change": 1},
		ToolErrors:  1,
		APIErrors:   1,
		FirstEditAt: 3,
		FinalText:   "really done",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
	}
}

// _fakeMuse stands in for the muse binary: it records its working directory and argv, writes into
// its config home as an agent or the CLI may, writes one stdout record, and writes the session logs
// muse writes under XDG_DATA_HOME — its own session's, one subagent's, and another session's that
// must not reach stdout — then exits 3.
const _fakeMuse = `#!/bin/sh
printf '%s\n' "$PWD" "$@" > "$ARGS_OUT"
echo planted > "$XDG_CONFIG_HOME/muse/planted"
while [ "$#" -gt 0 ]; do
	if [ "$1" = --session-id ]; then sid=$2; fi
	shift
done
d="$XDG_DATA_HOME/muse/sessions/2026/10/10"
mkdir -p "$d/$sid/subagent/s1" "$d/other"
echo '{"from":"stdout"}'
echo '{"from":"main"}' > "$d/$sid/session.jsonl"
echo '{"from":"subagent"}' > "$d/$sid/subagent/s1/session.jsonl"
echo '{"from":"other"}' > "$d/other/session.jsonl"
exit 3
`

func TestMuseCommandArgs(t *testing.T) {
	argv, env := Muse{}.Command("/tmp/prompt.md", "muse-spark-1.3", "low", "/work")
	if len(argv) < 7 || argv[0] != "sh" || argv[1] != "-c" || argv[4] != "/work" {
		t.Fatalf("argv = %q, want sh -c <script> sh /work <session id> <cli args>", argv)
	}
	sid := argv[5]
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(sid) {
		t.Errorf("session id %q is not a version 4 UUID", sid)
	}
	wantCLI := []string{"exec", "--json", "--model", "muse-spark-1.3", "--reasoning-effort", "low",
		"--approval-mode", "never", "--approval-judge", "off", "--disable-sandbox", "--session-id", sid,
		"--workspace", "/work", "--prompt-file", "/tmp/prompt.md"}
	if !slices.Equal(argv[6:], wantCLI) {
		t.Errorf("cli args =\n%q\nwant\n%q", argv[6:], wantCLI)
	}
	wantEnv := []string{"MUSE_INSTALL_DIR=/muse/bin", "FLOW_EVAL_MUSE_LOGIN=/muse/login",
		"XDG_CONFIG_HOME=/tmp/flow-eval-home/muse-code/config", "XDG_DATA_HOME=/tmp/flow-eval-home/muse-code"}
	if !slices.Equal(env, wantEnv) {
		t.Errorf("env = %q, want %q", env, wantEnv)
	}
	if again, _ := (Muse{}).Command("/tmp/prompt.md", "muse-spark-1.3", "low", "/work"); again[5] == sid {
		t.Errorf("two trials share session id %s", sid)
	}

	// The real wrapper, run with sh against the fake binary in a volume laid out as the setup leaves it.
	for _, tc := range []struct {
		name               string
		noBinary, noLogin  bool
		wantExit           int
		wantStdout, stderr string
	}{
		{name: "set up", wantExit: 3,
			wantStdout: "{\"from\":\"stdout\"}\n{\"from\":\"main\"}\n{\"from\":\"subagent\"}\n"},
		{name: "no binary", noBinary: true, wantExit: 2, stderr: "Muse Code setup in docs/muse-code.md"},
		{name: "no login", noLogin: true, wantExit: 2, stderr: "Muse Code setup in docs/muse-code.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin, conf, work := filepath.Join(dir, "bin"), filepath.Join(dir, "config"), filepath.Join(dir, "work")
			for _, d := range []string{bin, filepath.Join(conf, "muse"), work} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			files := map[string]string{filepath.Join(bin, ".muse-version"): "1.4.4-R5419.1\n"}
			if !tc.noBinary {
				files[filepath.Join(bin, "muse-bin-1.4.4-R5419.1")] = _fakeMuse
			}
			if !tc.noLogin {
				files[filepath.Join(conf, "muse", "auth.json")] = `{"providers":{}}`
			}
			for name, body := range files {
				if err := os.WriteFile(name, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			argv, _ := Muse{}.Command("/tmp/prompt.md", "muse-spark-1.3", "low", work)
			argsOut := filepath.Join(dir, "args")
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			home := filepath.Join(dir, "home")
			cmd.Env = append(os.Environ(), "MUSE_INSTALL_DIR="+bin, "FLOW_EVAL_MUSE_LOGIN="+conf,
				"XDG_CONFIG_HOME="+home, "XDG_DATA_HOME="+filepath.Join(dir, "data"), "ARGS_OUT="+argsOut)
			err := cmd.Run()
			if ee, ok := errors.AsType[*exec.ExitError](err); !ok || ee.ExitCode() != tc.wantExit {
				t.Fatalf("exit: %v, want %d; stderr %q", err, tc.wantExit, stderr.String())
			}
			if stdout.String() != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantStdout)
			}
			if !strings.Contains(stderr.String(), tc.stderr) {
				t.Errorf("stderr = %q, want it to name %q", stderr.String(), tc.stderr)
			}
			if tc.wantExit != 3 {
				return
			}
			got, err := os.ReadFile(argsOut)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
			wantDir, _ := filepath.EvalSymlinks(work)
			if gotDir, _ := filepath.EvalSymlinks(lines[0]); gotDir != wantDir {
				t.Errorf("muse ran in %s, want %s", lines[0], work)
			}
			if !slices.Equal(lines[1:], argv[6:]) {
				t.Errorf("muse got args %q, want %q", lines[1:], argv[6:])
			}
			// muse ran on its own config home holding a copy of the login; what it wrote there stays out
			// of the shared login, so it cannot reach the next trial.
			if got, err := os.ReadFile(filepath.Join(home, "muse", "auth.json")); err != nil || string(got) != `{"providers":{}}` {
				t.Errorf("config home login = %q, %v; want a copy of the volume's", got, err)
			}
			if _, err := os.Stat(filepath.Join(conf, "muse", "planted")); err == nil {
				t.Error("a file muse wrote to its config home reached the shared login")
			}
		})
	}
}

// muse-exec.jsonl is a real `muse exec --json` stdout followed by its session logs, main then two
// subagents, as Command writes them (muse 1.4.4-R5419.1, muse-spark-1.3, effort low, 2026-10-10),
// trimmed to the records Parse reads plus a few it skips, tool outputs cut short.
func TestMuseParse(t *testing.T) {
	got := parseFile(t, Muse{}, "testdata/muse-exec.jsonl")
	calls := []Call{
		{Input: 33226, Output: 205},
		{Input: 33510 - 32625, Output: 118, CacheRead: 32625},
		{Input: 33827 - 33393, Output: 163, CacheRead: 33393},
		{Input: 34060 - 33777, Output: 251, CacheRead: 33777},
		{Input: 34411 - 34033, Output: 88, CacheRead: 34033},
		{Input: 34569 - 34289, Output: 48, CacheRead: 34289},
		{Input: 4452, Output: 1212}, // the subagents
		{Input: 4656, Output: 392},
	}
	want := Usage{
		Input: 44594, Output: 2477, CacheRead: 168117, Reasoning: 1733,
		Turns:        6,
		ToolCalls:    map[string]int{"read_file": 3, "edit_file": 1, "bash": 1},
		ToolErrors:   1,
		PeakContext:  34569,
		FirstEditAt:  3,
		ServedModels: []string{"muse-spark-1.3"},
		FinalText:    "I changed `add` to return `a + b` instead of `a - b`.\n",
		Calls:        calls,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
	}
	if _, err := (Muse{}).Parse(strings.NewReader(`{"payload_type":"tool.result"}`)); err == nil {
		t.Error("Parse of a transcript with no run.terminal record succeeded, want an error")
	}
}

// A run muse ends on an API error — the 429 of an exhausted subscription quota — is an outage; one
// that fails for any other reason is not.
func TestMuseParseReportsAnAPIOutage(t *testing.T) {
	const quota = "API error 429 [request_id=r1]: Subscription quota exhausted. (rate_limit_error)"
	tests := []struct {
		name, payloadType, reason, wantOutage string
		wantAPIErrors                         int
	}{
		{"quota exhausted", "run.terminal.failed", quota, quota, 1},
		{"failed for another reason", "run.terminal.failed", "tool budget exceeded", "", 0},
		{"completed", "run.terminal.completed", "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := fmt.Sprintf(`{"payload_type":%q,"payload":{"text":"","reason":%q}}`, tt.payloadType, tt.reason)
			got, err := (Muse{}).Parse(strings.NewReader(rec))
			if err != nil {
				t.Fatal(err)
			}
			if got.Outage != tt.wantOutage || got.APIErrors != tt.wantAPIErrors {
				t.Errorf("Outage %q, APIErrors %d; want %q, %d", got.Outage, got.APIErrors, tt.wantOutage, tt.wantAPIErrors)
			}
		})
	}
}

// muse-denied.jsonl is a real muse-code trial's stdout tool and terminal records followed by the
// main session log's assistant_tool_calls_committed records for those calls (muse 1.4.4-R5419.1,
// muse-spark-1.3, effort low, 2026-10-10). Its second call is a bash call muse denied: the
// tool.result carries no correlation_facts, only "tool denied: ...", so its name comes from the
// committed record with the same call_id.
func TestMuseParseCountsADeniedCallAsADenialUnderItsName(t *testing.T) {
	got := parseFile(t, Muse{}, "testdata/muse-denied.jsonl")
	want := Usage{
		ToolCalls:         map[string]int{"bash": 2, "read_file": 2},
		ToolErrors:        1, // the first bash call, whose shell failed
		PermissionDenials: 1,
		FinalText:         got.FinalText,
	}
	if !strings.HasSuffix(got.FinalText, "verdict: mismatch") {
		t.Errorf("FinalText = %q, want the terminal record's text", got.FinalText)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
	}
}

// A file edit's position counts a denied call before it, although its name resolves only once the
// session log is read.
func TestMuseFirstEditAtCountsADeniedCallBeforeIt(t *testing.T) {
	const stream = `{"stream":{"id":"s"},"payload_type":"tool.result","payload":{"call_id":"c1","text":"tool denied: x"}}
{"stream":{"id":"s"},"payload_type":"tool.result","payload":{"call_id":"c2","correlation_facts":{"tool_name":"edit_file","outcome":"success"},"edit_facts":{}}}
{"stream":{"id":"s"},"payload_type":"run.terminal.completed","payload":{"text":"done"}}
{"stream":{"id":"s"},"payload_type":"runtime.session","payload":{"event":{"kind":"assistant_tool_calls_committed","tool_calls":[{"call_id":"c1","name":"bash"}]}}}
`
	u, err := Muse{}.Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if u.FirstEditAt != 2 || !maps.Equal(u.ToolCalls, map[string]int{"bash": 1, "edit_file": 1}) {
		t.Errorf("FirstEditAt %d, ToolCalls %v; want 2 and bash 1, edit_file 1", u.FirstEditAt, u.ToolCalls)
	}
}

// Each call returns its own mounts, so no caller's change reaches another trial.
func TestMuseMountsAreAFreshCopy(t *testing.T) {
	m := MuseMounts()
	m[0] = "type=bind,src=/,dst=/muse/bin"
	if MuseMounts()[0] == m[0] {
		t.Error("a change to one MuseMounts result reached the next")
	}
}

func TestParseRejectsMalformedLine(t *testing.T) {
	for _, h := range []Harness{Claude{}, Codex{}, Muse{}} {
		if _, err := h.Parse(bytes.NewReader([]byte("{\"type\":\"turn.started\"}\nnot json\n"))); err == nil {
			t.Errorf("%s Parse of a malformed line succeeded, want an error", h.Name())
		}
	}
}

// TestCommandFeedsPromptOnStdin runs the real wrapper with sh: the prompt file reaches the CLI on
// stdin in workdir, whatever its size, and never as an argument.
func TestCommandFeedsPromptOnStdin(t *testing.T) {
	dir := t.TempDir()
	prompt := filepath.Join(dir, "prompt.md")
	big := strings.Repeat("x", 200<<10) // over Linux's 128 KiB single-argument cap
	if err := os.WriteFile(prompt, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	argv, _ := withHome("probe", "PROBE_HOME", dir, prompt, "sh", "-c", `pwd; wc -c; echo "$#"`, "sh")
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PROBE_HOME="+filepath.Join(dir, "home"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(out))
	wantDir, _ := filepath.EvalSymlinks(dir)
	if len(fields) != 3 || fields[1] != strconv.Itoa(len(big)) || fields[2] != "0" {
		t.Errorf("wrapper output %q, want [<workdir> %d 0]", out, len(big))
	}
	if gotDir, _ := filepath.EvalSymlinks(fields[0]); gotDir != wantDir {
		t.Errorf("ran in %s, want %s", fields[0], dir)
	}
}

func TestUsageTokensCountsEveryClassOnce(t *testing.T) {
	u := Usage{Input: 1, Output: 2, CacheRead: 4, CacheWrite: 8, CacheWrite1h: 16, Reasoning: 2}
	if got := u.Tokens(); got != 31 {
		t.Errorf("Tokens() = %d, want 31 (Reasoning is inside Output)", got)
	}
}
