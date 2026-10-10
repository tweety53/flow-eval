package metrics

import (
	json "encoding/json/v2"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tweety53/flow-eval/internal/harness"
)

var _testPricing = Pricing{
	"claude-sonnet-5-5": {Input: 2, Output: 10, CacheRead: 0.1, CacheWrite: 2.5, CacheWrite1h: 4, Source: "https://example.test/a"},
	"gpt-5.6-terra":     {Input: 2, Output: 12, CacheRead: 0.2, CacheWrite: 2.5, Source: "https://example.test/o"},
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCostFromPricing(t *testing.T) {
	u := harness.Usage{Input: 1_000_000, Output: 100_000, CacheRead: 2_000_000, CacheWrite: 400_000}
	t.Run("alias resolves through the served model", func(t *testing.T) {
		u := u
		u.ServedModels = []string{"claude-sonnet-5-5"}
		got, err := FromUsage(u, "sonnet", _testPricing, 1500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		// 1M×2 + 0.1M×10 + 2M×0.1 + 0.4M×2.5 = 2 + 1 + 0.2 + 1 = 4.2
		if !approx(got.CostUSD, 4.2) || got.WallMS != 1500 {
			t.Errorf("cost=%v wall=%v, want 4.2 and 1500", got.CostUSD, got.WallMS)
		}
	})
	t.Run("1-hour cache writes take their own price", func(t *testing.T) {
		u := u
		u.CacheWrite, u.CacheWrite1h = 100_000, 300_000
		u.ServedModels = []string{"claude-sonnet-5-5"}
		got, err := FromUsage(u, "sonnet", _testPricing, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		// 2 + 1 + 0.2 + 0.1M×2.5 + 0.3M×4 = 3.2 + 0.25 + 1.2 = 4.65
		if !approx(got.CostUSD, 4.65) || got.CacheWrite1h != 300_000 {
			t.Errorf("cost=%v cache_write_1h=%d, want 4.65 and 300000", got.CostUSD, got.CacheWrite1h)
		}
	})
	t.Run("1-hour cache writes on a model with no 1-hour price are an error", func(t *testing.T) {
		u := u
		u.CacheWrite1h = 1
		if _, err := FromUsage(u, "gpt-5.6-terra", _testPricing, time.Second); err == nil {
			t.Error("FromUsage priced 1-hour writes at 0, want an error")
		}
	})
	t.Run("an alias with no served model resolves through pricing.json", func(t *testing.T) {
		got, err := FromUsage(harness.Usage{}, "sonnet", _testPricing, time.Minute)
		if err != nil || got.CostUSD != 0 {
			t.Errorf("FromUsage of an empty timed-out trial = cost %v, err %v; want 0, nil", got.CostUSD, err)
		}
	})
	t.Run("no served model prices the requested id", func(t *testing.T) {
		got, err := FromUsage(u, "gpt-5.6-terra", _testPricing, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		// 2 + 1.2 + 0.4 + 1 = 4.6
		if !approx(got.CostUSD, 4.6) {
			t.Errorf("cost=%v, want 4.6", got.CostUSD)
		}
	})
	t.Run("pricing.json prices the operator's models", func(t *testing.T) {
		p, err := LoadPricing(filepath.Join("..", "..", "pricing.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5", "gpt-5.6-terra", "muse-spark-1.3"} {
			if _, ok := p[m]; !ok {
				t.Errorf("pricing.json has no %s", m)
			}
		}
		if p["claude-haiku-5-5"].Long == nil {
			t.Error("pricing.json prices claude-haiku-5-5 without its long-prompt tier")
		}
	})
}

func TestUnknownModelPricingErrors(t *testing.T) {
	u := harness.Usage{Input: 10, ServedModels: []string{"claude-haiku-9"}}
	if got, err := FromUsage(u, "haiku", _testPricing, time.Second); err == nil {
		t.Fatalf("FromUsage of an unpriced model = cost %v, want an error", got.CostUSD)
	}
}

func TestUnpricedModelIsNotPricedAsAnIdContainingIt(t *testing.T) {
	// codex exec names no served model, so only the requested id is known.
	if got, err := FromUsage(harness.Usage{Input: 1_000_000}, "gpt-5", _testPricing, time.Second); err == nil {
		t.Fatalf("FromUsage of unpriced gpt-5 = cost %v (gpt-5.6-terra's rate), want an error", got.CostUSD)
	}
}

func TestLoadPricingRejectsIncompleteEntry(t *testing.T) {
	const base = `{"m":{"input":1,"output":2,"cache_read":0.1,"cache_write":1,"source":"https://x",`
	tests := []struct {
		name string
		json string
	}{
		{name: "missing cache_write", json: `{"m":{"input":1,"output":2,"cache_read":0.1,"source":"https://x"}}`},
		{name: "missing source", json: `{"m":{"input":1,"output":2,"cache_read":0.1,"cache_write":1}}`},
		{name: "empty file", json: `{}`},
		{name: "long tier missing threshold", json: base + `"long":{"input":2,"output":4,"cache_read":0.2,"cache_write":2,"cache_write_1h":3}}}`},
		{name: "long tier missing input", json: base + `"long":{"over_prompt_tokens":100,"output":4,"cache_read":0.2,"cache_write":2,"cache_write_1h":3}}}`},
		{name: "long tier missing output", json: base + `"long":{"over_prompt_tokens":100,"input":2,"cache_read":0.2,"cache_write":2,"cache_write_1h":3}}}`},
		{name: "long tier missing cache_read", json: base + `"long":{"over_prompt_tokens":100,"input":2,"output":4,"cache_write":2,"cache_write_1h":3}}}`},
		{name: "long tier missing cache_write", json: base + `"long":{"over_prompt_tokens":100,"input":2,"output":4,"cache_read":0.2,"cache_write_1h":3}}}`},
		{name: "long tier missing cache_write_1h", json: base + `"long":{"over_prompt_tokens":100,"input":2,"output":4,"cache_read":0.2,"cache_write":2}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pricing.json")
			if err := os.WriteFile(path, []byte(tt.json), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPricing(path); err == nil {
				t.Error("LoadPricing succeeded, want an error")
			}
		})
	}
}

func TestLongPromptCallsPricedAtTheLongRate(t *testing.T) {
	// Haiku 5.5's prices: base for a prompt up to 100,000 tokens, long over it.
	p := Pricing{"claude-haiku-5-5": {
		Rates: Rates{Input: 0.1, Output: 0.5, CacheRead: 0.01, CacheWrite: 0.125, CacheWrite1h: 0.2},
		Long: &LongPrice{OverPromptTokens: 100_000,
			Rates: Rates{Input: 0.5, Output: 2.5, CacheRead: 0.05, CacheWrite: 0.625, CacheWrite1h: 1}},
		Source: "https://example.test/a",
	}}
	// Each prompt class is needed to cross the threshold: 1 + 99,997 + 1 + 2 = 100,001.
	over := harness.Call{Input: 1, CacheRead: 99_997, CacheWrite: 1, CacheWrite1h: 2, Output: 1000}
	at := harness.Call{Input: 1, CacheRead: 99_997, CacheWrite: 1, CacheWrite1h: 1, Output: 1000}
	under := harness.Call{Input: 10, CacheRead: 50_000, Output: 500}
	usage := func(calls ...harness.Call) harness.Usage {
		u := harness.Usage{Calls: calls, ServedModels: []string{"claude-haiku-5-5"}}
		for _, c := range calls {
			u.Input += c.Input
			u.Output += c.Output
			u.CacheRead += c.CacheRead
			u.CacheWrite += c.CacheWrite
			u.CacheWrite1h += c.CacheWrite1h
		}
		return u
	}
	tests := []struct {
		name    string
		usage   harness.Usage
		want    float64
		wantErr bool
	}{
		// long(over) = 0.5 + 4999.85 + 0.625 + 2 + 2500 = 7502.975; base(under) = 1 + 500 + 250 = 751
		{name: "a call over the threshold at the long rate", usage: usage(over, under), want: 8253.975e-6},
		// base(at) = 0.1 + 999.97 + 0.125 + 0.2 + 500 = 1500.395
		{name: "a call at the threshold at the base rate", usage: usage(at, under), want: 2251.395e-6},
		{name: "a tiered model without per-call usage", usage: harness.Usage{Input: 1, ServedModels: []string{"claude-haiku-5-5"}}, wantErr: true},
		{name: "an empty tiered trial costs nothing", usage: harness.Usage{}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromUsage(tt.usage, "claude-haiku-5-5", p, time.Second)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FromUsage err = %v, wantErr %v", err, tt.wantErr)
			}
			if !approx(got.CostUSD, tt.want) {
				t.Errorf("cost = %v, want %v", got.CostUSD, tt.want)
			}
		})
	}
}

func TestCacheHitRatio(t *testing.T) {
	tests := []struct {
		name             string
		input, cacheRead int64
		want             float64
	}{
		{name: "three quarters cached", input: 25, cacheRead: 75, want: 0.75},
		{name: "nothing cached", input: 40, want: 0},
		{name: "no input at all", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := harness.Usage{Input: tt.input, CacheRead: tt.cacheRead}
			got, err := FromUsage(u, "gpt-5.6-terra", _testPricing, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if !approx(got.CacheHitRatio, tt.want) {
				t.Errorf("CacheHitRatio = %v, want %v", got.CacheHitRatio, tt.want)
			}
		})
	}
}

func TestServedModelMismatch(t *testing.T) {
	p := Pricing{
		"claude-sonnet-5-5": _testPricing["claude-sonnet-5-5"],
		"claude-opus-5-5":   _testPricing["claude-sonnet-5-5"],
		"opus":              _testPricing["claude-sonnet-5-5"],
		"gpt-5.6-terra":     _testPricing["gpt-5.6-terra"],
		"gpt-5-mini":        _testPricing["gpt-5.6-terra"],
	}
	tests := []struct {
		name      string
		requested string
		served    []string
		want      *bool
	}{
		{name: "alias matches its served id", requested: "sonnet", served: []string{"claude-sonnet-5-5"}, want: new(true)},
		{name: "another model served", requested: "sonnet", served: []string{"claude-opus-5-5"}, want: new(false)},
		{
			name: "a side model beside the requested one", requested: "sonnet",
			served: []string{"claude-haiku-5-5", "claude-sonnet-5-5"}, want: new(true),
		},
		{name: "served model unknown", requested: "opus", want: nil},
		{name: "an id containing the requested one", requested: "gpt-5", served: []string{"gpt-5.6-terra"}, want: new(false)},
		{
			name: "a dated snapshot of the requested id", requested: "claude-sonnet-5-5",
			served: []string{"claude-haiku-5-5", "claude-sonnet-5-5-20260301"}, want: new(true),
		},
		{name: "a longer id sharing the requested prefix", requested: "gpt-5", served: []string{"gpt-5-mini"}, want: new(false)},
		{name: "a version suffix is not a date", requested: "claude-opus-5", served: []string{"claude-opus-5-5"}, want: new(false)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := harness.Usage{ServedModels: tt.served}
			got, err := FromUsage(u, tt.requested, p, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.ServedMatchesRequested, tt.want) {
				t.Errorf("ServedMatchesRequested = %v (served %q), want %v",
					fmtBool(got.ServedMatchesRequested), got.ServedModel, fmtBool(tt.want))
			}
		})
	}
}

func TestPassK(t *testing.T) {
	tr := func(passed bool, cost float64) Trial {
		score := 0.0
		if passed {
			score = 100
		}
		return Trial{Passed: passed, Score: score, CostUSD: cost}
	}
	t.Run("one failed trial breaks pass^k", func(t *testing.T) {
		got := CaseAggregate([]Trial{tr(true, 1), tr(true, 1), tr(false, 1)})
		if got.PassK || got.Passes != 2 || !approx(got.MeanScore, 200.0/3) {
			t.Errorf("aggregate = %+v, want PassK false, 2 passes, mean 66.7", got)
		}
		// population stddev of {100,100,0}: sqrt(((33.3)²×2 + (66.7)²)/3) = 47.14
		if !approx(math.Round(got.StdDev*100)/100, 47.14) || got.CostPerPass == nil || !approx(*got.CostPerPass, 1.5) {
			t.Errorf("stddev=%v costPerPass=%v, want 47.14 and 1.5", got.StdDev, got.CostPerPass)
		}
	})
	t.Run("infra-error trials are excluded", func(t *testing.T) {
		infra := Trial{InfraError: true}
		got := CaseAggregate([]Trial{tr(true, 2), infra, tr(true, 2)})
		if !got.PassK || got.Valid != 2 || got.MeanScore != 100 {
			t.Errorf("aggregate = %+v, want PassK true over 2 valid trials", got)
		}
	})
	t.Run("cost, wall time and every token class are summed over valid trials", func(t *testing.T) {
		valid := Trial{CostUSD: 1, WallMS: 1000, Input: 1, Output: 2, CacheRead: 4, CacheWrite: 8, CacheWrite1h: 16, Reasoning: 2}
		got := CaseAggregate([]Trial{valid, {InfraError: true, CostUSD: 5, WallMS: 5, Input: 100}, valid})
		if !approx(got.Cost, 2) || got.WallMS != 2000 || got.Tokens != 62 {
			t.Errorf("cost %v, wall %d ms, tokens %d; want 2, 2000, 62", got.Cost, got.WallMS, got.Tokens)
		}
	})
	t.Run("no pass has no cost per pass", func(t *testing.T) {
		got := CaseAggregate([]Trial{tr(false, 1)})
		if got.PassK || got.CostPerPass != nil {
			t.Errorf("aggregate = %+v, want PassK false and nil CostPerPass", got)
		}
	})
}

func fmtBool(b *bool) string {
	if b == nil {
		return "null"
	}
	return strconv.FormatBool(*b)
}

func TestRoleScoreBalancesVisualVerdicts(t *testing.T) {
	visual := func(label string, scores ...float64) []Trial {
		var ts []Trial
		for _, s := range scores {
			ts = append(ts, Trial{Score: s, Visual: &Visual{Label: label}})
		}
		return ts
	}
	alwaysMismatch := [][]Trial{visual("match", 0)}
	for range 12 {
		alwaysMismatch = append(alwaysMismatch, visual("mismatch", 100))
	}
	for _, tc := range []struct {
		name   string
		cases  [][]Trial
		want   float64
		wantOK bool
	}{
		{"non-visual cases: the mean of the case means", [][]Trial{{{Score: 100}, {Score: 0}}, {{Score: 100}}}, 75, true},
		{"an always-mismatch verifier scores 50 on 1 match and 12 mismatch cases", alwaysMismatch, 50, true},
		{"an always-mismatch verifier scores 100 on a set of mismatch cases only", alwaysMismatch[1:], 100, true},
		{"each verdict's cases are averaged before the verdicts", [][]Trial{visual("match", 100), visual("match", 0), visual("mismatch", 100)}, 75, true},
		{"an infra-only case is left out", [][]Trial{{{Score: 80}}, {{InfraError: true}}}, 80, true},
		{"no scored case", [][]Trial{{{InfraError: true}}}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := RoleScore(tc.cases)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("RoleScore = %v, %v; want %v, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestImplRecordCarriesNoUnmeasuredLintCount(t *testing.T) {
	b, err := json.Marshal(Trial{Work: &Work{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "lint") {
		t.Errorf("impl record %s carries a lint count nothing measures", b)
	}
}
