// Package metrics derives a trial's design.md §6 record from a harness's normalized usage, prices it
// from pricing.json, and aggregates a case's trials for the §7 pass rule.
package metrics

import (
	json "encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tweety53/flow-eval/internal/harness"
)

// Rates is a USD price per million tokens of each class.
type Rates struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	// CacheWrite1h prices 1-hour cache writes; 0 for a model whose harness never writes them.
	CacheWrite1h float64 `json:"cache_write_1h,omitzero"`
}

// cost is c's tokens priced at r, in USD.
func (r Rates) cost(c harness.Call) float64 {
	return (float64(c.Input)*r.Input + float64(c.Output)*r.Output + float64(c.CacheRead)*r.CacheRead +
		float64(c.CacheWrite)*r.CacheWrite + float64(c.CacheWrite1h)*r.CacheWrite1h) / 1e6
}

// Price is one model's prices.
type Price struct {
	Rates `json:",inline"`
	// Long, when set, prices a call whose prompt is over its threshold; nil for a flat-priced model.
	Long   *LongPrice `json:"long,omitzero"`
	Source string     `json:"source"` // the page the prices were read from
}

// LongPrice is a long-prompt tier: every token of a call whose prompt (input, cache read and both
// cache writes) is over OverPromptTokens is priced at its Rates.
type LongPrice struct {
	OverPromptTokens int64 `json:"over_prompt_tokens"`
	Rates            `json:",inline"`
}

// Pricing maps a model id to its price.
type Pricing map[string]Price

// LoadPricing reads pricing.json. Every entry needs all four prices and a source, and a long tier
// all five prices and its threshold: a missing price would silently cost 0.
func LoadPricing(path string) (Pricing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load pricing: %w", err)
	}
	var p Pricing
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("load pricing %s: %w", path, err)
	}
	if len(p) == 0 {
		return nil, fmt.Errorf("load pricing %s: no models", path)
	}
	for model, pr := range p {
		if pr.Input <= 0 || pr.Output <= 0 || pr.CacheRead <= 0 || pr.CacheWrite <= 0 || pr.Source == "" {
			return nil, fmt.Errorf("load pricing %s: %s needs input, output, cache_read, cache_write > 0 and a source", path, model)
		}
		if l := pr.Long; l != nil && (l.OverPromptTokens <= 0 || l.Input <= 0 || l.Output <= 0 ||
			l.CacheRead <= 0 || l.CacheWrite <= 0 || l.CacheWrite1h <= 0) {
			return nil, fmt.Errorf("load pricing %s: %s long tier needs over_prompt_tokens, input, output, cache_read, cache_write, cache_write_1h > 0", path, model)
		}
	}
	return p, nil
}

// Trial is one trial's record: one JSONL line of runs/<run-id>.jsonl (design.md §6, §8). The
// orchestrator sets TimedOut, EgressDenied and InfraError; the grader sets Score, Passed and the
// role block.
type Trial struct {
	WallMS     int64 `json:"wall_ms"`
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
	// CacheWrite1h is the input written to a 1-hour cache, priced apart from CacheWrite.
	CacheWrite1h int64   `json:"cache_write_1h"`
	Reasoning    int64   `json:"reasoning"`
	CostUSD      float64 `json:"cost_usd"`
	// ReportedCostUSD is the harness's own cost figure, kept beside CostUSD when it reports one.
	ReportedCostUSD *float64       `json:"reported_cost_usd,omitzero"`
	ToolCalls       map[string]int `json:"tool_calls"`
	ToolCallsTotal  int            `json:"tool_calls_total"`

	ServedModel string `json:"served_model"` // comma-separated when several answered
	// ServedMatchesRequested is whether a served model is the requested one; null when the harness
	// names no served model (codex exec), which is unknown rather than a mismatch.
	ServedMatchesRequested *bool `json:"served_matches_requested"`
	APIErrors              int   `json:"api_errors"`
	TimedOut               bool  `json:"timed_out"`
	EgressDenied           int   `json:"egress_denied"`
	InfraError             bool  `json:"infra_error"`

	Turns int `json:"turns"`
	// FirstEditToolCall is the 1-based position of the first file-editing tool call, 0 for none.
	// It stands in for time_to_first_edit_ms: not every harness timestamps its transcript events.
	FirstEditToolCall int     `json:"first_edit_tool_call"`
	PeakContextTokens int64   `json:"peak_context_tokens"`
	Compactions       int     `json:"compactions"`
	ToolErrors        int     `json:"tool_errors"`
	PermissionDenials int     `json:"permission_denials"`
	CacheHitRatio     float64 `json:"cache_hit_ratio"`

	Score  float64 `json:"score"` // 0–100 (design.md §7)
	Passed bool    `json:"passed"`

	Work   *Work   `json:"work,omitzero"`
	Review *Review `json:"review,omitzero"`
	Visual *Visual `json:"visual,omitzero"`
}

// Work is an impl trial's work quality. It has no lint count: a case's verify commands do not mark
// which one is lint, and a 0 that was never measured would read as a measurement.
type Work struct {
	DiffLines         int      `json:"diff_lines"`
	OverbuildRatio    float64  `json:"overbuild_ratio"`
	OutOfScopeFiles   []string `json:"out_of_scope_files"`
	TestsAdded        int      `json:"tests_added"`
	PreexistingBroken int      `json:"preexisting_broken"`
}

// Review is a review trial's scoring against the case labels.
type Review struct {
	Recall            float64 `json:"recall"`
	Precision         float64 `json:"precision"`
	F1                float64 `json:"f1"`
	SeverityAgreement float64 `json:"severity_agreement"`
	Duplicates        int     `json:"duplicates"`
	KnownFalseHits    int     `json:"known_false_hits"`
}

// Visual is a visual trial's verdict scoring.
type Visual struct {
	// Label is the case's labelled verdict, match or mismatch: the class RoleScore balances over.
	Label        string  `json:"label"`
	Correct      bool    `json:"correct"`
	FalsePass    bool    `json:"false_pass"`
	Localization float64 `json:"localization"`
}

// FromUsage derives a trial from u. The cost is priced at the served model matching requested (see
// matches), else the first served model, else, when the harness names none, the one pricing.json id
// matching requested. A model missing from p, or 1-hour cache writes on a model
// with no 1-hour price, is an error, never a cost of 0. A model with a long-prompt tier is priced
// per call, and is an error when the harness reports no calls for its tokens.
// ponytail: all tokens are priced at one model; a run whose CLI also called a side model is
// overcharged at the main model's rates. Price per served model when the harness reports the split.
func FromUsage(u harness.Usage, requested string, p Pricing, wall time.Duration) (Trial, error) {
	model := pricedModel(requested, u.ServedModels, p)
	price, ok := p[model]
	if !ok {
		price, ok = p[undated(model)]
	}
	if !ok {
		return Trial{}, fmt.Errorf("price model %s (requested %s): not in pricing.json", model, requested)
	}
	if u.CacheWrite1h > 0 && price.CacheWrite1h <= 0 {
		return Trial{}, fmt.Errorf("price model %s: %d 1-hour cache-write tokens but no cache_write_1h price", model, u.CacheWrite1h)
	}
	if price.Long != nil && len(u.Calls) == 0 && u.Tokens() > 0 {
		return Trial{}, fmt.Errorf("price model %s: its long-prompt tier needs per-call usage, which the harness did not report", model)
	}
	t := Trial{
		WallMS:            wall.Milliseconds(),
		Input:             u.Input,
		Output:            u.Output,
		CacheRead:         u.CacheRead,
		CacheWrite:        u.CacheWrite,
		CacheWrite1h:      u.CacheWrite1h,
		Reasoning:         u.Reasoning,
		ReportedCostUSD:   u.ReportedCostUSD,
		ToolCalls:         u.ToolCalls,
		ServedModel:       strings.Join(u.ServedModels, ","),
		APIErrors:         u.APIErrors,
		Turns:             u.Turns,
		FirstEditToolCall: u.FirstEditAt,
		PeakContextTokens: u.PeakContext,
		Compactions:       u.Compactions,
		ToolErrors:        u.ToolErrors,
		PermissionDenials: u.PermissionDenials,
	}
	// The totals come from the harness's own summary; a long call adds its difference over base.
	t.CostUSD = price.cost(harness.Call{Input: u.Input, Output: u.Output, CacheRead: u.CacheRead,
		CacheWrite: u.CacheWrite, CacheWrite1h: u.CacheWrite1h})
	if l := price.Long; l != nil {
		for _, c := range u.Calls {
			if c.Prompt() > l.OverPromptTokens {
				t.CostUSD += l.cost(c) - price.cost(c)
			}
		}
	}
	for _, n := range u.ToolCalls {
		t.ToolCallsTotal += n
	}
	if len(u.ServedModels) > 0 {
		// A side model the CLI calls on its own (titles, summaries) is not a substitution.
		t.ServedMatchesRequested = new(slices.ContainsFunc(u.ServedModels, func(m string) bool {
			return matches(requested, m)
		}))
	}
	if in := u.Input + u.CacheRead; in > 0 {
		t.CacheHitRatio = float64(u.CacheRead) / float64(in)
	}
	return t, nil
}

// pricedModel picks the model a trial is priced at. With no served model (a CLI that names none,
// or a trial that timed out before any output) requested resolves through the one pricing.json id
// matching it; with none or several, requested itself, which pricing then refuses.
func pricedModel(requested string, served []string, p Pricing) string {
	for _, m := range served {
		if matches(requested, m) {
			return m
		}
	}
	if len(served) > 0 {
		return served[0]
	}
	var ids []string
	for id := range p {
		if matches(requested, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return requested
}

// matches reports whether served is the requested model: the same id, the same id with a date
// snapshot suffix (claude-sonnet-5-5-20260301), or, for a Claude CLI alias such as sonnet, an id of
// the form claude-<alias>-<version>. Any other request must name its id exactly: gpt-5 is not
// gpt-5.6-terra, and gpt-5 is not gpt-5-mini.
func matches(requested, served string) bool {
	return requested != "" && (served == requested || undated(served) == requested ||
		strings.HasPrefix(served, "claude-"+requested+"-"))
}

// undated strips a date snapshot suffix: claude-sonnet-5-5-20260301 is claude-sonnet-5-5.
func undated(id string) string { return _dateSuffix.ReplaceAllString(id, "") }

var _dateSuffix = regexp.MustCompile(`-\d{8}$`)

// Aggregate is one case's trials summarized for the pass rule (design.md §7). Infra-error trials are
// excluded from every figure.
type Aggregate struct {
	Valid     int     `json:"valid"` // trials that are not infra errors
	Passes    int     `json:"passes"`
	PassK     bool    `json:"pass_k"` // every valid trial passed
	MeanScore float64 `json:"mean_score"`
	StdDev    float64 `json:"stddev"` // population standard deviation of the scores
	// CostPerPass is the valid trials' total cost over their passes; nil when none passed.
	CostPerPass *float64 `json:"cost_per_pass,omitzero"`
	Cost        float64  `json:"cost"`    // the valid trials' total cost
	WallMS      int64    `json:"wall_ms"` // the valid trials' total wall time
	Tokens      int64    `json:"tokens"`  // the valid trials' tokens of every class
}

// CaseAggregate summarizes one case's trials.
func CaseAggregate(trials []Trial) Aggregate {
	var a Aggregate
	var sum float64
	var scores []float64
	for _, t := range trials {
		if t.InfraError {
			continue
		}
		scores = append(scores, t.Score)
		sum += t.Score
		a.Cost += t.CostUSD
		a.WallMS += t.WallMS
		a.Tokens += t.Input + t.Output + t.CacheRead + t.CacheWrite + t.CacheWrite1h
		if t.Passed {
			a.Passes++
		}
	}
	a.Valid = len(scores)
	if a.Valid == 0 {
		return a
	}
	a.PassK = a.Passes == a.Valid
	a.MeanScore = sum / float64(a.Valid)
	var sq float64
	for _, s := range scores {
		sq += (s - a.MeanScore) * (s - a.MeanScore)
	}
	a.StdDev = math.Sqrt(sq / float64(a.Valid))
	if a.Passes > 0 {
		a.CostPerPass = new(a.Cost / float64(a.Passes))
	}
	return a
}

// RoleScore is a run's role score over its cases' trials (design.md §7): the mean of the case mean
// scores, a case with no valid trial left out. Visual cases are first grouped by their labelled
// verdict, and the score is the mean of each present verdict's mean — balanced accuracy. A verifier
// that always answers one verdict scores 50 when both verdicts are present, whatever the mix, and
// 100 on a set holding only that verdict. ok is false when no case is scored.
func RoleScore(cases [][]Trial) (score float64, ok bool) {
	sums, counts := map[string]float64{}, map[string]int{}
	for _, trials := range cases {
		a := CaseAggregate(trials)
		if a.Valid == 0 {
			continue
		}
		class := "" // every non-visual case, and a visual record from before labels were kept
		if v := trials[slices.IndexFunc(trials, func(t Trial) bool { return !t.InfraError })].Visual; v != nil {
			class = v.Label
		}
		sums[class] += a.MeanScore
		counts[class]++
	}
	if len(counts) == 0 {
		return 0, false
	}
	for _, c := range slices.Sorted(maps.Keys(counts)) { // a fixed order keeps the float sum stable
		score += sums[c] / float64(counts[c])
	}
	return score / float64(len(counts)), true
}
